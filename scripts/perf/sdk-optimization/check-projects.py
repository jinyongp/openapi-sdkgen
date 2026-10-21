"""Fresh, bounded strict checks of every owned source and all public declarations.

Composite requires incremental metadata, but every invocation creates fresh
configs, declaration output and build-info files. No cached success is reused.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def dump(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def check(args):
    plan_path = args.plan.resolve()
    plan = json.loads(plan_path.read_text())
    ownership_path = plan_path.parent / 'ownership.jsonl'
    if digest(ownership_path) != plan['ownershipSha256']:
        raise ValueError('ownership manifest changed')
    ownership = [json.loads(line) for line in ownership_path.read_text().splitlines()]
    root = Path(plan['root'])
    owners = {}
    for group in plan['groups']:
        for file in group['files']:
            if file in owners or Path(file).is_absolute() or '..' in Path(file).parts:
                raise ValueError('duplicate/invalid source owner: ' + file)
            owners[file] = group['id']
    if len(owners) != plan['files'] or len(ownership) != plan['files']:
        raise ValueError('incomplete ownership')
    if len({entry['path'] for entry in ownership}) != plan['files']:
        raise ValueError('duplicate source in ownership manifest')
    for index, group in enumerate(plan['groups']):
        if group['id'] != index or any(dep >= index or dep < 0 for dep in group['dependencies']):
            raise ValueError('invalid project dependency order')

    def verify_sources():
        actual = {str(file.relative_to(root)) for file in root.rglob('*.ts') if file.is_file()}
        if actual != set(owners):
            raise ValueError('SDK sources added/removed after inventory')
        for entry in ownership:
            file = entry['path']
            if owners.get(file) != entry['owner'] or digest(root / file) != entry['sha256']:
                raise ValueError('changed or unowned source: ' + file)

    verify_sources()
    output = args.output.resolve()
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    declarations = output / 'declarations'
    configs = output / 'configs'
    configs.mkdir()
    declarations.mkdir()
    dump(declarations / 'package.json', {'type': 'module', 'private': True})
    strict = json.loads((ROOT / 'internal/tscheck/strict-options.json').read_text())['compilerOptions']
    options = dict(strict, target='ES2022', module='NodeNext', moduleResolution='NodeNext',
                   lib=['ES2022', 'DOM', 'DOM.Iterable'], types=[], noEmit=False,
                   composite=True, incremental=True, declaration=True, emitDeclarationOnly=True,
                   removeComments=args.remove_declaration_comments,
                   rootDir=str(root), outDir=str(declarations),
                   disableSourceOfProjectReferenceRedirect=True)
    for group in plan['groups']:
        config = {'compilerOptions': dict(options, tsBuildInfoFile=str(output / f"build-{group['id']}.tsbuildinfo")),
                  'files': [str(root / file) for file in group['files']],
                  'references': [{'path': str(configs / f'{dep}.json')} for dep in group['dependencies']]}
        dump(configs / f"{group['id']}.json", config)
    compiler = args.compiler.resolve()
    package = json.loads((compiler.parent.parent / 'package.json').read_text())
    native = package['name'] == '@typescript/native-preview' or package['version'].startswith('7.')
    command = [args.node, '--max-old-space-size=' + str(args.node_heap_mib), str(compiler)]
    if native:
        command.append('--singleThreaded')
    started = time.monotonic()
    results = []
    result = {'status': 'fail', 'planSha256': digest(plan_path), 'ownershipSha256': digest(ownership_path),
              'referenceSha256': digest(args.reference.resolve()),
              'compilerVersion': package['version'], 'files': plan['files'], 'projects': len(plan['groups']),
              'freshBuildInfo': True, 'strictOptions': strict, 'results': results}
    result['removeDeclarationComments'] = args.remove_declaration_comments
    result['usageConsumers'] = {str(file.resolve()): digest(file.resolve()) for file in args.usage_consumer}

    def trial(name, argv, owned_sources=None):
        remaining = args.timeout - (time.monotonic() - started)
        if remaining <= 0:
            raise ValueError('whole compiler invocation time budget exhausted')
        directory = output / name
        run = subprocess.run([sys.executable, str(HERE / 'record.py'), '--output', str(directory),
                              '--input', str(plan_path), '--timeout', str(remaining),
                              '--max-rss-mib', str(args.max_rss_mib), '--', *argv], cwd=ROOT, check=False)
        value = json.loads((directory / 'result.json').read_text())
        results.append({'name': name, 'status': value['status'], 'elapsedSeconds': value.get('elapsedSeconds'),
                        'peakSessionRSSBytes': value.get('peakSessionRSSBytes')})
        if run.returncode or value['status'] != 'pass':
            raise ValueError('strict compiler check failed: ' + name)
        if owned_sources is not None:
            observed = set()
            for line in (directory / 'stdout.log').read_text().splitlines():
                file = Path(line)
                if file.is_absolute() and file.is_relative_to(root) and line.endswith('.ts'):
                    observed.add(str(file.relative_to(root)))
            if observed != set(owned_sources):
                raise ValueError('compiler source ownership/redirect mismatch: ' + name)

    try:
        for group in plan['groups']:
            trial(f"project-{group['id']:04d}", [*command, '-p', str(configs / f"{group['id']}.json"), '--pretty', 'false', '--listFiles'], group['files'])
            for file in group['files']:
                declaration = declarations / (file[:-3] + '.d.ts')
                if not declaration.is_file():
                    raise ValueError('missing checked declaration: ' + file)
        verify_sources()
        declaration_files = list(declarations.rglob('*.ts'))
        if len(declaration_files) != plan['files']:
            raise ValueError('declaration ownership count mismatch')
        for file in declaration_files:
            if not str(file).endswith('.d.ts'):
                raise ValueError('source leaked into declaration consumer')
        subprocess.run([args.node, str(HERE / 'prepare-ts.mjs'), str(declarations)], check=True, cwd=ROOT)
        consumer = output / 'consumer'
        subprocess.run([args.node, str(HERE / 'public-consumer.mjs'), 'prepare', str(declarations),
                        str(args.reference.resolve()), str(consumer)], check=True, cwd=ROOT)
        for index, template in enumerate(args.usage_consumer):
            code = template.read_text()
            replaced = 0
            for marker, target in {'__SDK_ROOT__': 'index.js', '__SDK_WEBHOOKS__': 'server/webhooks.js', '__SDK_CALLBACKS__': 'server/callbacks.js'}.items():
                quoted = json.dumps(marker)
                if quoted not in code:
                    continue
                if code.count(quoted) != 1:
                    raise ValueError('each usage import marker must occur exactly once')
                relative = os.path.relpath(declarations / target, consumer).replace(os.sep, '/')
                code = code.replace(quoted, json.dumps(relative))
                replaced += 1
            if not replaced or '"__SDK_' in code:
                raise ValueError('unknown/missing usage import marker')
            usage = consumer / f'public-usage-{index}.mts'
            usage.write_text(code)
            config_path = consumer / 'tsconfig.json'
            config = json.loads(config_path.read_text())
            config['files'].append(str(usage))
            dump(config_path, config)
        trial('public-consumer', [*command, '-p', str(consumer / 'tsconfig.json'), '--pretty', 'false'])
        verify_sources()
        result['status'] = 'pass'
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        result['error'] = str(error)
    result['elapsedSeconds'] = time.monotonic() - started
    result['completedProjects'] = sum(row['status'] == 'pass' and row['name'].startswith('project-') for row in results)
    dump(output / 'result.json', result)
    print(json.dumps({key: result[key] for key in ('status', 'files', 'projects', 'completedProjects', 'elapsedSeconds')}))
    return 0 if result['status'] == 'pass' else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--plan', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--compiler', type=Path, required=True)
    parser.add_argument('--reference', type=Path, required=True)
    parser.add_argument('--node', default='node')
    parser.add_argument('--node-heap-mib', type=int, default=7168)
    parser.add_argument('--max-rss-mib', type=int, default=8192)
    parser.add_argument('--timeout', type=float, default=3600)
    parser.add_argument('--remove-declaration-comments', action='store_true',
                        help='omit comments only from scratch checking declarations; original SDK remains unchanged')
    parser.add_argument('--usage-consumer', type=Path, action='append', default=[], help='additional independently justified public usage fixture; repeatable')
    args = parser.parse_args()
    if args.node_heap_mib > args.max_rss_mib - 1024 or args.timeout <= 0:
        parser.error('heap must leave at least 1 GiB RSS headroom; timeout must be positive')
    return check(args)


if __name__ == '__main__':
    raise SystemExit(main())
