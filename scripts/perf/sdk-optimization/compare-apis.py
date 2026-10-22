"""Matched real-API generation and cold/warm build trials; local evidence only."""
import argparse
from contextlib import nullcontext
import hashlib
import io
import itertools
import json
import math
import os
from pathlib import Path
import resource
import statistics
import subprocess
import sys
import tarfile
import time

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from resources import POLICY_PATH, cancellation_signals, go_cache, owned_process
REVISIONS = {
    'baseline': '5ada624a69ebfd85103248379970f78a8f3d2899',
    'final': '52eab9882d4a4df33b0131b119f37d87c254344b',
}
PROVIDERS = {
    'tvmaze': ('.tmp/compatibility-holdout/APIs/tvmaze.com/1.0/openapi.yaml',
               'd74e37b951a03b17ccd271777c0083053992f91b9c58e8681a51ca88af4eab62'),
    'listennotes': ('.tmp/compatibility-holdout/APIs/listennotes.com/2.0/openapi.yaml',
                   '51b15eb0e703c26eece40c964245c0434cb0e8ff7a419be2cd6fd9dcebd93216'),
    'github': ('.tmp/compatibility-regression/github/api.github.com.2022-11-28.json',
               'd39842ee4d43d701e8a8c5483b4afa7c23218518f943dea2e42d36a3798cbdcc'),
}


def sha(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def dump(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def host_state():
    memory = dict(line.split(':', 1) for line in Path('/proc/meminfo').read_text().splitlines())
    counters = dict(line.split() for line in Path('/proc/vmstat').read_text().splitlines())
    return {'memoryKiB': {key: int(memory[key].split()[0]) for key in ['MemAvailable', 'SwapFree']},
            'swapPages': {key: int(counters[key]) for key in ['pswpin', 'pswpout']},
            'loadAverage': Path('/proc/loadavg').read_text().split()[:3]}


def measure_child(arguments):
    """Fresh worker: precise subprocess wall time and wait4 high-water RSS."""
    destination, *command = arguments
    started = time.perf_counter()
    result = subprocess.run(command)
    usage = resource.getrusage(resource.RUSAGE_CHILDREN)
    dump(Path(destination), {'seconds': time.perf_counter() - started,
                            'peakProcessRSSBytes': usage.ru_maxrss * 1024,
                            'userSeconds': usage.ru_utime, 'systemSeconds': usage.ru_stime,
                            'exitCode': result.returncode})
    return result.returncode


def summarize(rows):
    summaries = []
    groups = sorted({(row['provider'], row['target'], row['phase']) for row in rows})
    for provider, target, phase in groups:
        selected = [r for r in rows if (r['provider'], r['target'], r['phase']) == (provider, target, phase)]
        item = {'provider': provider, 'target': target, 'phase': phase, 'variants': {}}
        for variant in REVISIONS:
            successful = [r for r in selected if r['variant'] == variant and r['status'] == 'pass']
            item['variants'][variant] = {'completed': len(successful)}
            for metric in ['seconds', 'peakProcessRSSBytes', 'peakSessionRSSBytes']:
                values = [r[metric] for r in successful]
                item['variants'][variant][metric] = {
                    'median': statistics.median(values) if values else None,
                    'range': [min(values), max(values)] if values else None,
                }
        paired = {variant: {r['pair']: r for r in selected if r['variant'] == variant and r['status'] == 'pass'}
                  for variant in REVISIONS}
        item['pairedTimeRatios'] = [paired['final'][pair]['seconds'] / paired['baseline'][pair]['seconds']
                                    for pair in sorted(paired['baseline'].keys() & paired['final'].keys())]
        ratios = item['pairedTimeRatios']
        if ratios:
            item['pairedGeometricTimeRatio'] = statistics.geometric_mean(ratios)
        if 2 <= len(ratios) <= 5:
            samples = sorted(statistics.geometric_mean(sample)
                             for sample in itertools.product(ratios, repeat=len(ratios)))
            item['pairedBootstrap95'] = [samples[math.floor((len(samples) - 1) * 0.025)],
                                         samples[math.ceil((len(samples) - 1) * 0.975)]]
        summaries.append(item)
    return summaries


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--pairs', type=int, default=5)
    parser.add_argument('--provider', choices=list(PROVIDERS), action='append')
    args = parser.parse_args()
    if args.pairs < 1:
        parser.error('positive pair count required')
    output = args.output.resolve()
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    with cancellation_signals(), go_cache(output, existing=os.environ.get('GOCACHE')) as cache:
        return compare(args, output, cache)


def compare(args, output, cache):
    inputs = {}
    for provider in args.provider or PROVIDERS:
        relative, expected = PROVIDERS[provider]
        source = ROOT / relative
        if sha(source) != expected:
            raise ValueError('input pin mismatch: ' + provider)
        inputs[provider] = {'path': str(source), 'sha256': expected, 'bytes': source.stat().st_size}
    environment = dict(os.environ, GOTOOLCHAIN='local', GOWORK='off', CGO_ENABLED='0',
                       GOMAXPROCS='4', GOMEMLIMIT='4GiB', NODE_OPTIONS='--max-old-space-size=5120',
                       GOFLAGS='', GOEXPERIMENT='', GOGC='100', GOCACHE=cache)
    # No network API requests. Compiler modules/toolchains must already be installed.
    environment['GOPROXY'] = 'off'
    provenance = {
        'status': 'running', 'revisions': REVISIONS, 'pairs': args.pairs, 'inputs': inputs,
        'producerSha256': sha(__file__), 'recorderSha256': sha(HERE / 'record.py'),
        'resourcePolicySha256': sha(POLICY_PATH),
        'cachePolicy': {'run': 'caller-owned' if os.environ.get('GOCACHE') else 'temporary-owned',
                        'coldWarm': 'temporary-owned-per-pair'},
        'hostBefore': host_state(),
        'tools': {name: subprocess.check_output(command, text=True, env=environment).strip()
                  for name, command in {'go': ['go', 'version'], 'node': ['node', '--version']}.items()},
        'environment': {key: environment[key] for key in ['GOTOOLCHAIN', 'GOWORK', 'CGO_ENABLED',
            'GOMAXPROCS', 'GOMEMLIMIT', 'NODE_OPTIONS', 'GOFLAGS', 'GOEXPERIMENT', 'GOGC', 'GOPROXY', 'GOCACHE']},
        'scope': 'Full pinned real API documents, server enabled, no selection, serial alternating pairs. '
                 'Fresh output per generation. Go cold: empty dedicated GOCACHE including stdlib; '
                 'warm: identical no-change build in same cache. TS cold: no build-info; '
                 'warm: no-change incremental typecheck with same build-info, fresh process. '
                 'Filesystem/page cache uncontrolled, not OS-cold. Generator build/preparation excluded. '
                 'RSS high-water is largest individual process/descendant, not simultaneous total; '
                 'separate session RSS is sampled every 200 ms and can double-count shared pages.',
    }
    dump(output / 'provenance.json', provenance)
    rows, inventories = [], []

    def save():
        dump(output / 'trials.json', rows)
        dump(output / 'summary.json', {'status': provenance['status'], 'pairs': args.pairs,
             'failures': sum(row['status'] != 'pass' for row in rows),
             'summaries': summarize(rows), 'inventories': inventories})

    def trial(directory, command, cwd=ROOT, extra_env=None, timeout=600):
        before = host_state()
        metrics = directory.parent / (directory.name + '-metrics.json')
        invocation = [sys.executable, str(HERE / 'record.py'), '--output', str(directory),
                      '--cwd', str(cwd), '--timeout', str(timeout), '--max-rss-mib', '6144',
                      '--input', str(Path(__file__).resolve()), '--', sys.executable,
                      str(Path(__file__).resolve()), '_child', str(metrics), *map(str, command)]
        with owned_process(invocation, cwd=ROOT, env=environment | (extra_env or {})) as process:
            process.wait()
        result = json.loads((directory / 'result.json').read_text())
        measured = json.loads(metrics.read_text()) if metrics.exists() else {}
        return {'status': result['status'], 'recorder': str(directory / 'result.json'),
                'peakSessionRSSBytes': result.get('peakSessionRSSBytes'),
                'hostBefore': before, 'hostAfter': host_state(), **measured}

    binaries = {}
    for variant, revision in REVISIONS.items():
        source = output / (variant + '-source')
        source.mkdir()
        archive = subprocess.check_output(['git', 'archive', revision, 'cmd', 'internal', 'go.mod', 'go.sum'], cwd=ROOT)
        with tarfile.open(fileobj=io.BytesIO(archive)) as stream:
            stream.extractall(source, filter='data')
        binary = output / ('sdkgen-' + variant)
        build = trial(output / ('build-' + variant), ['go', 'build', '-p=1', '-trimpath', '-buildvcs=false',
                      '-o', binary, './cmd/openapi-sdkgen'], cwd=source)
        if build['status'] != 'pass':
            raise RuntimeError('generator build failed: ' + variant)
        binaries[variant] = binary
        provenance[variant] = {'binarySha256': sha(binary), 'archiveSha256': hashlib.sha256(archive).hexdigest(),
                               'build': build}
        dump(output / 'provenance.json', provenance)
    compiler = ROOT / 'test/typescript/node_modules/typescript-5-9/lib/tsc.js'
    provenance['compiler'] = {'version': json.loads((compiler.parent.parent / 'package.json').read_text())['version'],
                             'files': {p.name: sha(p) for p in sorted(compiler.parent.glob('*')) if p.is_file()}}
    dump(output / 'provenance.json', provenance)
    for provider, input_info in inputs.items():
        for target in ['typescript', 'golang']:
            reference = output / f'{provider}-public-contract.json'
            stop_case = False
            for pair in range(args.pairs):
                if stop_case:
                    break
                for variant in (['baseline', 'final'] if pair % 2 == 0 else ['final', 'baseline']):
                    case = output / f'{provider}-{target}-{pair}-{variant}'
                    case.mkdir()
                    sdk = case / 'api'
                    command = [binaries[variant], 'generate', '--target', target, '--with', 'server',
                               '--input', input_info['path'], '--output', sdk]
                    if target == 'golang':
                        (case / 'go.mod').write_text('module example.test/matched\n\ngo 1.27.0\n')
                        command += ['--go-import-path', 'example.test/matched/api']
                    else:
                        command += ['--typecheck']
                    result = trial(case / 'generation', command)
                    rows.append({'provider': provider, 'target': target, 'pair': pair,
                                 'variant': variant, 'phase': 'generation', **result})
                    save()
                    if result['status'] != 'pass':
                        stop_case = True
                        continue
                    manifest = sdk / '.openapi-sdkgen-manifest.json'
                    files = json.loads(manifest.read_text())['files']
                    if any(sha(sdk / name) != digest for name, digest in files.items()):
                        raise ValueError('managed output hash mismatch')
                    inventories.append({'provider': provider, 'target': target, 'pair': pair, 'variant': variant,
                        'files': len(files), 'bytes': sum((sdk / name).stat().st_size for name in files),
                        'manifestSha256': sha(manifest)})
                    if target == 'golang':
                        command = ['go', 'build', '-p=1', '-trimpath', '-buildvcs=false', './api/...']
                    else:
                        setup = [['node', HERE / 'prepare-ts.mjs', sdk]]
                        if variant == 'baseline' and pair == 0:
                            setup.append(['node', HERE / 'public-consumer.mjs', 'freeze', sdk, reference])
                        setup.append(['node', HERE / 'public-consumer.mjs', 'prepare', sdk, reference, case / 'consumer'])
                        prepared = True
                        for step, setup_command in enumerate(setup):
                            if trial(case / f'prepare-{step}', setup_command)['status'] != 'pass':
                                prepared = False
                                break
                        if not prepared:
                            rows.append({'provider': provider, 'target': target, 'pair': pair,
                                         'variant': variant, 'phase': 'prepare', 'status': 'fail'})
                            stop_case = True
                            save()
                            continue
                        command = ['node', compiler, '-p', case / 'consumer/tsconfig.json', '--pretty', 'false',
                                   '--extendedDiagnostics', '--incremental', '--tsBuildInfoFile', case / 'check.tsbuildinfo']
                    lifetime = go_cache(case, prefix='cache-') if target == 'golang' else nullcontext(None)
                    with lifetime as pair_cache:
                        extra = {'GOCACHE': pair_cache} if target == 'golang' else {}
                        for phase in ['cold', 'warm']:
                            result = trial(case / phase, command, cwd=case, extra_env=extra)
                            rows.append({'provider': provider, 'target': target, 'pair': pair,
                                         'variant': variant, 'phase': phase, **result})
                            save()
                            if result['status'] != 'pass':
                                stop_case = True
                                break
                    print(json.dumps({'provider': provider, 'target': target, 'pair': pair,
                                      'variant': variant, 'completed': not stop_case}), flush=True)
    provenance['status'] = 'fail' if any(row['status'] != 'pass' for row in rows) else 'pass'
    provenance['hostAfter'] = host_state()
    dump(output / 'provenance.json', provenance)
    save()
    return 0 if provenance['status'] == 'pass' else 1


if __name__ == '__main__':
    sys.exit(measure_child(sys.argv[2:]) if len(sys.argv) > 1 and sys.argv[1] == '_child' else main())
