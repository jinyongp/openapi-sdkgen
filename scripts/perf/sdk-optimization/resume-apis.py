"""Resume only missing complete Go cases, retaining the frozen timing worker."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(HERE.parent))
from resources import POLICY_PATH, cancellation_signals, go_cache, owned_process
WORKER = HERE / 'compare-apis.py'
spec = importlib.util.spec_from_file_location('compare_apis', WORKER)
study = importlib.util.module_from_spec(spec)
spec.loader.exec_module(study)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True, help='existing interrupted study')
    args = parser.parse_args()
    output = args.output.resolve()
    with cancellation_signals(), go_cache(output, existing=os.environ.get('GOCACHE')) as cache:
        return resume(output, cache)


def resume(output, cache):
    provenance = json.loads((output / 'provenance.json').read_text())
    rows = json.loads((output / 'trials.json').read_text())
    inventories = json.loads((output / 'summary.json').read_text())['inventories']
    assert provenance['producerSha256'] == study.sha(WORKER)
    assert provenance['recorderSha256'] == study.sha(HERE / 'record.py')
    assert provenance['resourcePolicySha256'] == study.sha(POLICY_PATH)
    assert provenance['revisions'] == study.REVISIONS and provenance['status'] == 'running'
    assert all(row['status'] == 'pass' for row in rows)
    completed = {(r['provider'], r['target'], r['pair'], r['variant'], r['phase']) for r in rows}
    assert len(completed) == len(rows)
    environment = {**os.environ, **provenance['environment'], 'GOCACHE': cache}
    assert subprocess.check_output(['go', 'version'], env=environment, text=True).strip() == provenance['tools']['go']
    for variant in study.REVISIONS:
        assert study.sha(output / ('sdkgen-' + variant)) == provenance[variant]['binarySha256']
    for value in provenance['inputs'].values():
        assert study.sha(value['path']) == value['sha256']
    recovery = output / 'recovery'
    recovery.mkdir(exist_ok=False)
    for name in ['provenance.json', 'summary.json', 'trials.json']:
        (recovery / ('before-' + name)).write_bytes((output / name).read_bytes())
    provenance['recovery'] = {'producerSha256': study.sha(__file__), 'completedCommandsBefore': len(rows),
        'reason': 'Original host execution ended with status 143 between cases; cause unknown. '
                  'No controllers remained. Completed samples retained without rerunning.',
        'worker': str(WORKER), 'workerSha256': study.sha(WORKER), 'hostBefore': study.host_state(),
        'runCache': cache, 'resourcePolicySha256': study.sha(POLICY_PATH)}

    def save():
        study.dump(output / 'provenance.json', provenance)
        study.dump(output / 'trials.json', rows)
        study.dump(output / 'summary.json', {'status': provenance['status'], 'pairs': provenance['pairs'],
            'failures': sum(row['status'] != 'pass' for row in rows),
            'summaries': study.summarize(rows), 'inventories': inventories})

    def trial(directory, command, cwd, extra=None):
        before = study.host_state()
        metrics = directory.parent / (directory.name + '-metrics.json')
        invocation = [sys.executable, str(HERE / 'record.py'), '--output', str(directory),
            '--cwd', str(cwd), '--timeout', '600', '--max-rss-mib', '6144',
            '--input', str(WORKER), '--input', str(Path(__file__).resolve()), '--', sys.executable,
            str(WORKER), '_child', str(metrics), *map(str, command)]
        with owned_process(invocation, cwd=ROOT, env=environment | (extra or {})) as process:
            process.wait()
        result = json.loads((directory / 'result.json').read_text())
        measured = json.loads(metrics.read_text()) if metrics.exists() else {}
        return {'status': result['status'], 'recorder': str(directory / 'result.json'),
            'peakSessionRSSBytes': result.get('peakSessionRSSBytes'),
            'hostBefore': before, 'hostAfter': study.host_state(), **measured}

    save()
    for provider, value in provenance['inputs'].items():
        for target in ['typescript', 'golang']:
            for pair in range(provenance['pairs']):
                for variant in (['baseline', 'final'] if pair % 2 == 0 else ['final', 'baseline']):
                    keys = {(provider, target, pair, variant, phase) for phase in ['generation', 'cold', 'warm']}
                    if keys <= completed:
                        continue
                    if keys & completed or target != 'golang':
                        raise ValueError('only between-case Go recovery is supported')
                    case = output / f'{provider}-{target}-{pair}-{variant}'
                    case.mkdir(exist_ok=False)
                    (case / 'go.mod').write_text('module example.test/matched\n\ngo 1.27.0\n')
                    sdk = case / 'api'
                    command = [output / ('sdkgen-' + variant), 'generate', '--target', target,
                        '--with', 'server', '--input', value['path'], '--output', sdk,
                        '--go-import-path', 'example.test/matched/api']
                    with go_cache(case, prefix='cache-') as pair_cache:
                        for phase in ['generation', 'cold', 'warm']:
                            if phase == 'generation':
                                result = trial(case / phase, command, ROOT)
                            else:
                                result = trial(case / phase, ['go', 'build', '-p=1', '-trimpath',
                                    '-buildvcs=false', './api/...'], case, {'GOCACHE': pair_cache})
                            rows.append({'provider': provider, 'target': target, 'pair': pair,
                                         'variant': variant, 'phase': phase, **result})
                            save()
                            if result['status'] != 'pass':
                                provenance['status'] = 'fail'
                                save()
                                return 1
                            if phase == 'generation':
                                manifest = sdk / '.openapi-sdkgen-manifest.json'
                                files = json.loads(manifest.read_text())['files']
                                assert all(study.sha(sdk / name) == digest for name, digest in files.items())
                                inventories.append({'provider': provider, 'target': target, 'pair': pair,
                                    'variant': variant, 'files': len(files),
                                    'bytes': sum((sdk / name).stat().st_size for name in files),
                                    'manifestSha256': study.sha(manifest)})
                    print(json.dumps({'provider': provider, 'pair': pair, 'variant': variant,
                                      'status': 'pass'}), flush=True)
    provenance['status'] = 'pass'
    provenance['hostAfter'] = study.host_state()
    save()
    return 0


if __name__ == '__main__':
    sys.exit(main())
