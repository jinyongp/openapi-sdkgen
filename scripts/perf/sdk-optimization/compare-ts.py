"""Optional matched generation/checker trials; keeps every failed trial."""
import argparse
import hashlib
import json
from pathlib import Path
import statistics
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--baseline', required=True, type=Path)
    parser.add_argument('--candidate', required=True, type=Path)
    parser.add_argument('--input', action='append', required=True, type=Path)
    parser.add_argument('--reference', action='append', type=Path, help='independently frozen contract for each input')
    parser.add_argument('--pairs', type=int, default=5)
    parser.add_argument('--compiler', default='typescript-5-9')
    args = parser.parse_args()
    if args.reference and len(args.reference) != len(args.input):
        parser.error('supply one reference per input')
    if args.pairs < 1 or args.compiler not in ['typescript-5-7', 'typescript-5-9', 'typescript-6', 'typescript']:
        parser.error('positive pairs and an installed compiler alias required')
    output = args.output.resolve()
    output.mkdir(mode=0o700)
    variants = {'baseline': args.baseline.resolve(), 'candidate': args.candidate.resolve()}
    rows = []
    failures = 0

    def save():
        (output / 'trials.json').write_text(json.dumps(rows, indent=2) + '\n')

    def trial(directory, input_path, command):
        nonlocal failures
        invocation = [sys.executable, str(HERE / 'record.py'), '--output', str(directory),
                      '--input', str(input_path), '--max-rss-mib', '8192', '--timeout', '900', '--', *map(str, command)]
        result = subprocess.run(invocation, cwd=ROOT, text=True, capture_output=True)
        report = directory / 'result.json'
        if not report.exists():
            raise RuntimeError('recorder did not produce a result: ' + result.stderr[-2000:])
        row = json.loads(report.read_text())
        if row['status'] != 'pass':
            failures += 1
        return row

    provenance = {'pairs': args.pairs, 'compiler': args.compiler,
                  'binaries': {key: {'path': str(path), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
                               for key, path in variants.items()},
                  'inputs': [{'path': str(path.resolve()), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
                             for path in args.input],
                  'references': [{'path': str(path.resolve()), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
                                 for path in (args.reference or [])],
                  'boundary': 'fresh generator/checker processes; filesystem cache warm/unspecified; no incremental; alternating order; raw recorder per command'}
    (output / 'provenance.json').write_text(json.dumps(provenance, indent=2) + '\n')
    for input_index, input_path in enumerate(args.input):
        input_path = input_path.resolve()
        reference = args.reference[input_index].resolve() if args.reference else output / f'contract-{input_index}.json'
        for pair in range(args.pairs):
            failures_before_pair = failures
            order = ['baseline', 'candidate'] if pair % 2 == 0 else ['candidate', 'baseline']
            for variant in order:
                directory = output / f'input-{input_index}-pair-{pair}-{variant}'
                directory.mkdir(mode=0o700)
                sdk = directory / 'sdk'
                generation = trial(directory / 'generation', input_path,
                                   [variants[variant], 'generate', '--target', 'typescript', '--typecheck',
                                    '--input', input_path, '--output', sdk])
                row = {'input': input_index, 'pair': pair, 'variant': variant, 'generation': generation}
                rows.append(row)
                save()
                if generation['status'] != 'pass':
                    continue
                sources = sorted(sdk.rglob('*.ts'))
                row['tsFiles'] = len(sources)
                row['tsBytes'] = sum(path.stat().st_size for path in sources)
                config = trial(directory / 'prepare', input_path, ['node', HERE / 'prepare-ts.mjs', sdk])
                if config['status'] != 'pass':
                    save()
                    continue
                if variant == 'baseline' and pair == 0 and not args.reference:
                    frozen = trial(directory / 'freeze', input_path,
                                   ['node', HERE / 'public-consumer.mjs', 'freeze', sdk, reference])
                    if frozen['status'] != 'pass':
                        save()
                        continue
                consumer = directory / 'consumer'
                prepared = trial(directory / 'consumer-prepare', input_path,
                                 ['node', HERE / 'public-consumer.mjs', 'prepare', sdk, reference, consumer])
                if prepared['status'] != 'pass':
                    save()
                    continue
                command = ['node', ROOT / 'test/typescript/node_modules' / args.compiler / 'lib/tsc.js',
                           '-p', consumer / 'tsconfig.json', '--extendedDiagnostics', '--pretty', 'false']
                if args.compiler == 'typescript':
                    command.append('--singleThreaded')
                row['checker'] = trial(directory / 'checker', input_path, command)
                save()
            if failures > failures_before_pair:
                # Preserve this pair and its requested denominator, rather than
                # repeatedly invoking a known broken input/configuration.
                break
    summaries = []
    for input_index in range(len(args.input)):
        for metric in ['generation', 'checker']:
            samples = {variant: [row[metric]['elapsedSeconds'] for row in rows
                                 if row['input'] == input_index and row['variant'] == variant
                                 and metric in row and row[metric]['status'] == 'pass'] for variant in variants}
            paired = {variant: {row['pair']: row[metric]['elapsedSeconds'] for row in rows
                                if row['input'] == input_index and row['variant'] == variant
                                and metric in row and row[metric]['status'] == 'pass'} for variant in variants}
            ratios = [paired['candidate'][pair] / paired['baseline'][pair]
                      for pair in sorted(paired['baseline'].keys() & paired['candidate'].keys())]
            summaries.append({'input': input_index, 'metric': metric,
                              'requestedPerVariant': args.pairs, 'pairedCandidateBaselineRatios': ratios,
                              'completed': {key: len(value) for key, value in samples.items()},
                              'medianSeconds': {key: statistics.median(value) if value else None for key, value in samples.items()},
                              'rangeSeconds': {key: [min(value), max(value)] if value else None for key, value in samples.items()}})
    (output / 'summary.json').write_text(json.dumps({'failures': failures, 'summaries': summaries}, indent=2) + '\n')
    print(json.dumps({'failures': failures, 'summaries': summaries}))
    return 1 if failures else 0


if __name__ == '__main__':
    sys.exit(main())
