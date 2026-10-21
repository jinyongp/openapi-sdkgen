"""Linux-only, bounded local trials. No product optimization or public benchmark."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
FLAGS = ('GOTOOLCHAIN', 'GOWORK', 'CGO_ENABLED', 'GOMAXPROCS', 'GOMEMLIMIT', 'GOCACHE', 'NODE_OPTIONS')


def sha(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def dump(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def session_usage(session):
    """Include native children and children spawned by non-main Go threads.

    RSS sums can double-count shared pages; this is not cgroup physical usage.
    Descendants must retain their session. Deliberately detached daemons are out
    of scope for these finite generator/compiler commands.
    """
    rss, binaries = 0, set()
    for path in Path('/proc').glob('[0-9]*/stat'):
        try:
            fields = path.read_text().rsplit(')', 1)[1].split()
            if int(fields[3]) != session:
                continue
            rss += int(fields[21]) * os.sysconf('SC_PAGE_SIZE')
            binaries.add(str((path.parent / 'exe').resolve(strict=True)))
        except (OSError, ValueError, IndexError):
            continue
    return rss, binaries


def stop(process):
    # A Go verifier may put its compiler in a separate process group while
    # retaining the session. Terminate those children too, including orphans.
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    for _ in range(2):
        for path in Path('/proc').glob('[0-9]*/stat'):
            try:
                fields = path.read_text().rsplit(')', 1)[1].split()
                if int(fields[3]) == process.pid:
                    os.kill(int(path.parent.name), signal.SIGKILL)
            except (OSError, ValueError, IndexError):
                continue
    process.wait()


def run(args):
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    os.chmod(output, 0o700)
    env = dict(os.environ, GOTOOLCHAIN='go1.27.2', GOWORK='off', CGO_ENABLED='0', GOMAXPROCS='4')
    argv = args.command
    if argv and argv[0] == '--':
        argv = argv[1:]
    result = {'status': 'blocked', 'argv': argv, 'cwd': str(args.cwd.resolve()),
              'timeoutSeconds': args.timeout, 'rssLimitMiB': args.max_rss_mib,
              'rssScope': 'sum of owned session RSS, sampled every 200ms; shared pages may count twice',
              'environment': {k: env[k] for k in FLAGS if k in env}}
    dump(output / 'started.json', result)
    try:
        if not Path('/proc/self/stat').exists():
            raise ValueError('Linux procfs is required for resource protection')
        result['inputs'] = {str(p.resolve()): sha(p) for p in args.input}
        # Resolve, check AND execute the same Go binary/environment.
        goroot = subprocess.check_output(['go', 'env', 'GOROOT'], env=env, text=True, timeout=30).strip()
        go = str(Path(goroot) / 'bin/go')
        env['GOTOOLCHAIN'] = 'local'
        env['PATH'] = str(Path(go).parent) + os.pathsep + env.get('PATH', '')
        go_version = subprocess.check_output([go, 'version'], env=env, text=True, timeout=15).strip()
        if not go_version.startswith('go version go1.27.2 '):
            raise ValueError('required Go 1.27.2: ' + go_version)
        if not argv:
            raise ValueError('missing command after --')
        if argv[0] == 'go':
            argv = [go, *argv[1:]]
        executable = shutil.which(argv[0], path=env.get('PATH'))
        if executable is None:
            raise ValueError('executable unavailable: ' + argv[0])
        argv = [str(Path(executable).resolve()), *argv[1:]]
        result.update(argv=argv, goVersion=go_version, goBinary=go, goSha256=sha(go))
        result['environment'] = {k: env[k] for k in FLAGS if k in env}
        names = subprocess.check_output(['git', 'ls-files', '-co', '--exclude-standard', '-z'], cwd=ROOT).decode().split('\0')
        source = {n: sha(ROOT / n) for n in sorted(set(names)) if n and (ROOT / n).is_file()}
        dump(output / 'source.json', source)
        result['sourceManifestSha256'] = sha(output / 'source.json')
        result['head'] = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
        result['gitStatus'] = subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True).splitlines()
        result['commandFiles'] = {str(Path(a).resolve()): sha(a) for a in argv if Path(a).is_file()}
        for argument in argv:
            script = Path(argument)
            if script.name == 'tsc.js' and (script.parent.parent / 'package.json').is_file():
                package = script.parent.parent / 'package.json'
                result['compiler'] = {'version': json.loads(package.read_text())['version'],
                                      'files': {str(p.resolve()): sha(p) for p in sorted(script.parent.glob('*')) if p.is_file()} | {str(package.resolve()): sha(package)}}
        binaries = {argv[0]}
        started, peak, reason = time.monotonic(), 0, None
        with (output / 'stdout.log').open('wb') as out, (output / 'stderr.log').open('wb') as err:
            # Reuse the project's stream redactor BEFORE logs are stored.
            filters = [subprocess.Popen([sys.executable, str(ROOT / 'scripts/lib/redact.py')],
                       stdin=subprocess.PIPE, stdout=target, env=env) for target in (out, err)]
            try:
                process = subprocess.Popen(argv, cwd=args.cwd, env=env, start_new_session=True,
                                           stdout=filters[0].stdin, stderr=filters[1].stdin)
                while True:
                    rss, observed = session_usage(process.pid)
                    peak = max(peak, rss)
                    binaries.update(observed)
                    if rss > args.max_rss_mib * 1024**2:
                        reason = 'rss_protection'
                    elif time.monotonic() - started > args.timeout:
                        reason = 'timeout'
                    if reason:
                        stop(process)
                        break
                    if process.poll() is not None:
                        break
                    time.sleep(0.2)
                # Remove surviving ordinary descendants even after a driver exit.
                stop(process)
            finally:
                for filt in filters:
                    filt.stdin.close()
                for filt in filters:
                    if filt.wait(timeout=10):
                        raise RuntimeError('diagnostic redactor failed')
        result.update(status=reason or ('pass' if process.returncode == 0 else 'signal' if process.returncode < 0 else 'fail'),
                      exitCode=process.returncode, elapsedSeconds=time.monotonic() - started,
                      peakSessionRSSBytes=peak, binaries={p: sha(p) for p in sorted(binaries) if Path(p).is_file()})
    except (OSError, ValueError, subprocess.SubprocessError, RuntimeError) as error:
        result.update(status='blocked', error=str(error))
    for name in ('stdout.log', 'stderr.log'):
        if (output / name).exists():
            result[name + 'Sha256'] = sha(output / name)
    dump(output / 'result.json', result)
    print(json.dumps({k: result[k] for k in ('status', 'elapsedSeconds', 'peakSessionRSSBytes', 'error') if k in result} | {'result': str(output / 'result.json')}))
    return 0 if result['status'] == 'pass' else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True, help='fresh trial directory, never reused')
    parser.add_argument('--cwd', type=Path, default=ROOT)
    parser.add_argument('--input', type=Path, action='append', default=[])
    parser.add_argument('--timeout', type=float, default=600)
    parser.add_argument('--max-rss-mib', type=int, default=8192)
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if args.timeout <= 0 or args.max_rss_mib <= 0:
        parser.error('timeout and RSS limit must be positive')
    return run(args)


if __name__ == '__main__':
    sys.exit(main())
