"""Optional local Go measurements. Raw results are not public benchmark claims."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def tree(path):
    files = {str(p.relative_to(path)): digest(p) for p in sorted(path.rglob('*')) if p.is_file()}
    return {'files': len(files), 'bytes': sum(p.stat().st_size for p in path.rglob('*') if p.is_file()), 'sha256': hashlib.sha256(json.dumps(files, sort_keys=True).encode()).hexdigest(), 'fileHashes': files}

class Measurement:
    def __init__(self, output):
        self.output = output
        self.results = []
        self.environment = {**os.environ, 'GOWORK': 'off', 'GOTOOLCHAIN': 'local', 'CGO_ENABLED': '0'}
        self.binary = output / 'sdkgen'
        names = subprocess.check_output(['git', 'ls-files', '-co', '--exclude-standard', '-z'], cwd=ROOT).decode().split('\0')
        source = {n: digest(ROOT / n) for n in sorted(names) if n and not n.startswith('plans/') and (ROOT / n).is_file()}
        self.basis = {'scope': 'Local diagnostics; no public numeric claim or fixed CI percentage gate.', 'source': source, 'go': subprocess.check_output(['go', 'version'], text=True).strip(), 'node': subprocess.check_output(['node', '--version'], text=True).strip(), 'flags': {k: self.environment[k] for k in ['GOWORK', 'GOTOOLCHAIN', 'CGO_ENABLED']}}
        # Do not persist the process environment: it can contain credentials.
        self.basis['flags'] = {k: self.environment[k] for k in ['GOWORK', 'GOTOOLCHAIN', 'CGO_ENABLED']}
        (output / 'basis.json').write_text(json.dumps(self.basis, indent=2) + '\n')
        self.run('build-generator', ['go', 'build', '-o', str(self.binary), './cmd/openapi-sdkgen'], ROOT)
        self.basis['generatorSha256'] = digest(self.binary)
        (output / 'basis.json').write_text(json.dumps(self.basis, indent=2) + '\n')

    def stop(self, process):
        os.killpg(process.pid, signal.SIGTERM)
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait()

    def process_rss(self, pid):
        # GNU time launches the measured process; include its owned descendants.
        total = 0
        try:
            status = Path(f'/proc/{pid}/status').read_text()
            total = next(int(line.split()[1]) for line in status.splitlines() if line.startswith('VmRSS:'))
            children = Path(f'/proc/{pid}/task/{pid}/children').read_text().split()
            for child in children:
                total += self.process_rss(int(child))
        except (FileNotFoundError, ProcessLookupError, StopIteration):
            pass
        return total

    def run(self, label, argv, cwd, env=None, timeout=900, required=True):
        index = len(self.results)
        prefix = self.output / f'{index:03}-{label}'
        raw_out, raw_err, metrics = [prefix.with_suffix(suffix) for suffix in ['.stdout', '.stderr', '.time.json']]
        started = time.monotonic()
        with raw_out.open('w') as out, raw_err.open('w') as err:
            command = ['/usr/bin/time', '-f', '{"elapsedSeconds":%e,"maximumRSSKiB":%M,"userSeconds":%U,"systemSeconds":%S}', '-o', str(metrics), *map(str, argv)]
            process = subprocess.Popen(command, cwd=cwd, env=env or self.environment, stdout=out, stderr=err, start_new_session=True)
            stopped = None
            observed_rss = 0
            if (env or self.environment).get('GOMEMLIMIT'):
                while process.poll() is None:
                    observed_rss = max(observed_rss, self.process_rss(process.pid))
                    if observed_rss > 7 * 1024 * 1024:
                        stopped = '7GiB RSS safety bound'
                    elif time.monotonic() - started > timeout:
                        stopped = 'measurement deadline'
                    if stopped:
                        self.stop(process)
                        break
                    time.sleep(0.5)
                status = 124 if stopped else process.returncode
            else:
                try:
                    status = process.wait(timeout=timeout)
                except subprocess.TimeoutExpired:
                    self.stop(process)
                    stopped = 'measurement deadline'
                    status = 124
            if stopped:
                err.write('\nStopped at ' + stopped + '; not a passing sample.\n')
        result = {'label': label, 'argv': list(map(str, argv)), 'cwd': str(cwd), 'environment': {k: (env or self.environment)[k] for k in ['CGO_ENABLED', 'GOTOOLCHAIN', 'GOWORK', 'GOCACHE', 'GOOS', 'GOARCH', 'GOMEMLIMIT', 'GOMAXPROCS'] if k in (env or self.environment)}, 'exitCode': status, 'wallSeconds': time.monotonic() - started, 'stdout': str(raw_out), 'stderr': str(raw_err), 'stdoutSha256': digest(raw_out), 'stderrSha256': digest(raw_err), 'time': str(metrics), 'stoppedReason': stopped, 'maximumObservedGroupRSSKiB': observed_rss}
        if metrics.exists() and metrics.read_text().splitlines():
            # GNU time prefixes nonzero exits with a status line.
            line = metrics.read_text().splitlines()[-1]
            if line.startswith('{'):
                result['metrics'] = json.loads(line)
        self.results.append(result)
        (self.output / 'results.json').write_text(json.dumps(self.results, indent=2) + '\n')
        print(json.dumps({'label': label, 'exitCode': status}), flush=True)
        if required and status:
            raise SystemExit(f'{label} failed; inspect {raw_err} and {raw_out}')
        return result

    def generate(self, label, input_file, module, extra=()):
        sdk = module / 'api'
        if sdk.exists():
            shutil.rmtree(sdk)
        args = [self.binary, 'generate', '--target', 'golang', '--input', input_file, '--output', sdk, '--go-import-path', 'example.test/goperf/api', *extra]
        result = self.run(label, args, ROOT)
        result['inputSha256'] = digest(input_file)
        result['output'] = tree(sdk)
        (self.output / 'results.json').write_text(json.dumps(self.results, indent=2) + '\n')
        return sdk

    def module(self, name):
        path = self.output / name
        path.mkdir()
        (path / 'go.mod').write_text((ROOT / 'examples/golang-todo-app/go.mod').read_text().replace('example.com/sdkgen-go-example', 'example.test/goperf'))
        return path

    def runtime(self):
        module = self.module('runtime')
        sdk = self.generate('runtime-generation', HERE / 'runtime-fixture.json', module)
        shutil.copyfile(HERE / 'runtime_bench.go.tmpl', sdk / 'runtime_bench_test.go')
        shutil.copyfile(HERE / 'regex_parallel_bench.go.tmpl', sdk / 'regex_parallel_bench_test.go')
        self.run('runtime-compile', ['go', 'test', '-mod=readonly', '-run', '^$', './api'], module)
        self.run('runtime-live-heap', ['go', 'test', '-mod=readonly', '-v', '-run', '^Test((Stream|ResponseBody)LiveHeap|LargeJSONLiveHeapAndGC)$', '-count=1', './api'], module)
        self.run('runtime-benchmark', ['go', 'test', '-mod=readonly', '-run', '^$', '-bench', '.', '-benchmem', '-benchtime=100ms', '-count=5', '-timeout=15m', './api'], module)
        profile_binary = module / 'runtime-profile.test'
        mutex_profile = self.output / 'runtime-mutex.pprof'
        block_profile = self.output / 'runtime-block.pprof'
        self.run('runtime-concurrent-contention', ['go', 'test', '-mod=readonly', '-run', '^$', '-bench', '^BenchmarkConcurrent$', '-benchtime=100ms', '-count=1', '-o', profile_binary, '-mutexprofile', mutex_profile, '-blockprofile', block_profile, './api'], module)
        for name, profile in [('mutex', mutex_profile), ('block', block_profile)]:
            self.run('runtime-' + name + '-profile', ['go', 'tool', 'pprof', '-top', '-nodecount=15', profile_binary, profile], module)
        race_env = {**self.environment, 'CGO_ENABLED': '1'}
        self.run('runtime-concurrent-race', ['go', 'test', '-mod=readonly', '-race', '-run', '^$', '-bench', '^BenchmarkConcurrent$', '-benchtime=2x', '-count=1', './api'], module, env=race_env)

    def platform(self):
        module = self.module('platform')
        sdk = self.generate('platform-generation', HERE / 'fixture.json', module, ['--with', 'server', '--with', 'metadata'])
        imports = 'api "example.test/goperf/api";_ "example.test/goperf/api/server/runtime"'
        body = 'c,err:=api.NewClient(api.ClientOptions{BaseURL:"https://example.test",Transport:memoryTransport{}});if err!=nil{panic(err)};if _,err:=c.Operations.Todo().Do(ctx);err!=nil{panic(err)}'
        (module / 'main.go').write_text((HERE / 'consumer.go.tmpl').read_text().replace('ENTRY_IMPORTS', imports).replace('ENTRY_BODY', body))
        for target_os in ['linux', 'darwin', 'windows']:
            for arch in ['amd64', 'arm64']:
                env = {**self.environment, 'GOOS': target_os, 'GOARCH': arch}
                binary = module / (target_os + '-' + arch + ('.exe' if target_os == 'windows' else ''))
                result = self.run('platform-' + target_os + '-' + arch, ['go', 'build', '-mod=readonly', '-buildvcs=false', '-o', binary, '.'], module, env=env)
                result['binaryBytes'] = binary.stat().st_size
                result['binarySha256'] = digest(binary)
                result['executed'] = target_os == 'linux' and arch == 'amd64'
                if result['executed']:
                    self.run('platform-native-execution', [binary], module)
        wasm = module / 'playground.wasm'
        self.run('platform-playground-wasm', ['go', 'build', '-buildvcs=false', '-o', wasm, './cmd/playground-wasm'], ROOT, env={**self.environment, 'GOOS': 'js', 'GOARCH': 'wasm'})
        (self.output / 'results.json').write_text(json.dumps(self.results, indent=2) + '\n')

    def generation(self):
        for operations in [1000, 10000]:
            module = self.module(f'generation-{operations}')
            document = json.loads((HERE / 'fixture.json').read_text())
            schema = {'$ref': '#/components/schemas/Node'}
            document['paths'] = {f'/items/{n}': {'get': {'operationId': f'item{n}', 'responses': {'200': {'description': 'Shared recursive node', 'content': {'application/json': {'schema': schema}}}}}} for n in range(operations)}
            input_file = module / 'input.json'
            input_file.write_text(json.dumps(document) + '\n')
            for selected in [0, 1, 10]:
                extra = [arg for n in range(selected) for arg in ['--operation', f'item{n}']]
                hashes = []
                for sample in range(5):
                    self.generate(f'generation-{operations}-selected{selected}-sample{sample}', input_file, module, extra)
                    hashes.append(self.results[-1]['output']['sha256'])
                if len(set(hashes)) != 1:
                    raise SystemExit('Generation output changed between matched samples')
            # Keep selected10 as a compile check; large full output is covered separately.
            self.run(f'generation-{operations}-consumer', ['go', 'test', '-mod=readonly', '-run', '^$', './api/...'], module)

    def features(self):
        for name in ['small-json', 'regex-heavy', 'server']:
            module = self.module('features-' + name)
            document = {'openapi': '3.1.0', 'info': {'title': 'Fixed feature workload', 'version': '1'}, 'paths': {}}
            operations = 10 if name == 'small-json' else 1000
            pattern = r'(?i:^(?<word>\p{L}+)-\k<word>$)'
            for n in range(operations):
                schema = {'type': 'object', 'properties': {'id': {'type': 'string'}}}
                if name == 'regex-heavy':
                    schema = {'type': 'string', 'pattern': pattern}
                document['paths']['/items/' + str(n)] = {'get': {'operationId': 'feature' + str(n), 'responses': {'200': {'description': 'Feature cost', 'content': {'application/json': {'schema': schema}}}}}}
            input_file = module / 'input.json'
            input_file.write_text(json.dumps(document))
            for selected in [0, 1, 10]:
                extra = ['--with', 'server'] if name == 'server' else []
                extra += [arg for n in range(selected) for arg in ['--operation', 'feature' + str(n)]]
                hashes = []
                for sample in range(5):
                    self.generate(f'features-{name}-selected{selected}-sample{sample}', input_file, module, extra)
                    hashes.append(self.results[-1]['output']['sha256'])
                if len(set(hashes)) != 1:
                    raise SystemExit('Feature generation output changed between matched samples')
            self.run('features-' + name + '-consumer', ['go', 'test', '-mod=readonly', '-run', '^$', './api/...'], module)

    def providers(self):
        auditor = self.output / 'golang-compat'
        self.run('build-provider-auditor', ['go', 'build', '-o', auditor, './scripts/golang-compat'], ROOT)
        manifest = ROOT / 'test/compatibility/regression.json'
        entries = json.loads(manifest.read_text())['corpora']
        corpus_root = ROOT / '.tmp/compatibility-regression'
        env = {**self.environment, 'GOMEMLIMIT': '5GiB', 'GOMAXPROCS': '2'}
        for item in entries:
            emitted_routes = None
            for selected in [0, 1, 10]:
                hashes = []
                for sample in range(5):
                    label = f'provider-{item["id"]}-selected{selected}-sample{sample}'
                    report = self.output / (label + '.json')
                    selectors = [] if selected == 0 else [arg for route in emitted_routes[:selected] for arg in ['--route', route]]
                    result = self.run(label, [auditor, '--manifest', manifest, '--corpus-root', corpus_root, '--document', item['id'], '--server', '--mode', 'golang-generation', '--output', report, *selectors], ROOT, env=env)
                    data = json.loads(report.read_text())
                    if data['mode'] != 'golang-generation' or not data['pass']:
                        raise SystemExit('Provider generation did not complete: ' + label)
                    if selected == 0:
                        emitted_routes = sorted(data['golang'].get('routes', []))
                        if len(emitted_routes) < 10:
                            raise SystemExit('Provider has fewer than ten emitted routes: ' + item['id'])
                    result['auditorSha256'] = digest(auditor)
                    result['inputSha256'] = data['inputSha256']
                    result['reportSha256'] = digest(report)
                    result['generatedFiles'] = data['golang']['files']
                    result['generatedBytes'] = data['golang']['bytes']
                    result['generatedSha256'] = data['golang']['artifactSha256']
                    hashes.append(result['generatedSha256'])
                    (self.output / 'results.json').write_text(json.dumps(self.results, indent=2) + '\n')
                if len(set(hashes)) != 1:
                    raise SystemExit('Provider output changed between matched samples')

    def builds(self, variant=None):
        variants = {
            'full-static': ('api "example.test/goperf/api"', 'c,err:=api.NewClient(api.ClientOptions{BaseURL:"https://example.test",Transport:memoryTransport{}});if err!=nil{panic(err)};if _,err:=c.Operations.Todo().Do(ctx);err!=nil{panic(err)}', []),
            'selected': ('api "example.test/goperf/api"', 'c,err:=api.NewClient(api.ClientOptions{BaseURL:"https://example.test",Transport:memoryTransport{}});if err!=nil{panic(err)};if _,err:=c.Operations.Todo().Do(ctx);err!=nil{panic(err)}', ['--operation', 'todo']),
            'named': ('api "example.test/goperf/api/clients/only"', 'c,err:=api.NewClient(api.ClientOptions{BaseURL:"https://example.test",Transport:memoryTransport{}});if err!=nil{panic(err)};if _,err:=c.Operations.Todo().Do(ctx);err!=nil{panic(err)}', []),
            'dynamic': ('"os";"example.test/goperf/api/selective"', 'id:="todo";if len(os.Args)>1{id=os.Args[1]};ref,ok:=selective.LookupOperation(id);if !ok{panic("missing operation")};p,err:=selective.Prepare(ref);if err!=nil{panic(err)};c,err:=selective.NewClient(selective.ClientOptions{BaseURL:"https://example.test",Transport:memoryTransport{}},p);if err!=nil{panic(err)};operation,err:=selective.Bind(c,ref);if err!=nil{panic(err)};if _,err:=operation.Call(types.Value{}).Do(ctx);err!=nil{panic(err)}', []),
            'server': ('server "example.test/goperf/api/server/runtime"', 'raw,err:=types.ValueFrom(map[string]any{"types":[]any{"string"},"pattern":"^[a-zA-Z]+$"});if err!=nil{panic(err)};contract,err:=server.CompileSchema(raw,nil);if err!=nil{panic(err)};value,err:=types.ValueFrom("Go");if err!=nil{panic(err)};if err:=contract.Validate(ctx,value);err!=nil{panic(err)}', ['--with', 'server']),
        }
        for name, (imports, body, extra) in variants.items():
            if variant is not None and name != variant:
                continue
            module = self.module('build-' + name)
            if name == 'named':
                config = module / 'clients.toml'
                config.write_text('[clients.only.selection]\noperations=["todo"]\n[clients.other.selection]\noperations=["xml","upload","events","secure","node","pattern"]\n')
                extra = ['--config', str(config)]
            hashes = []
            for sample in range(5):
                self.generate(f'build-{name}-generation{sample}', HERE / 'fixture.json', module, ['--with', 'metadata', *extra])
                hashes.append(self.results[-1]['output']['sha256'])
            if len(set(hashes)) != 1:
                raise SystemExit('Consumer generation output changed between matched samples')
            (module / 'main.go').write_text((HERE / 'consumer.go.tmpl').read_text().replace('ENTRY_IMPORTS', imports).replace('ENTRY_BODY', body))
            for sample in range(5):
                cache = self.output / f'cache-{name}-{sample}'
                cache.mkdir()
                env = {**self.environment, 'GOCACHE': str(cache)}
                for state in ['cold', 'warm']:
                    binary = module / f'{state}-{sample}'
                    result = self.run(f'build-{name}-{state}{sample}', ['go', 'build', '-mod=readonly', '-buildvcs=false', '-o', binary, '.'], module, env=env)
                    result['binaryBytes'] = binary.stat().st_size
                    result['binarySha256'] = digest(binary)
                    self.run(f'run-{name}-{state}{sample}', [binary], module)
                # The next cold build starts with its own empty cache.
                shutil.rmtree(cache)
            self.run('deps-' + name, ['go', 'list', '-mod=readonly', '-buildvcs=false', '-deps', '.'], module)
            self.run('symbols-' + name, ['go', 'tool', 'nm', module / 'warm-4'], module)
            stripped = module / 'stripped'
            result = self.run('stripped-' + name, ['go', 'build', '-mod=readonly', '-buildvcs=false', '-ldflags=-s -w', '-o', stripped, '.'], module)
            result['binaryBytes'] = stripped.stat().st_size
            result['binarySha256'] = digest(stripped)
            self.run('run-stripped-' + name, [stripped], module)
            (self.output / 'results.json').write_text(json.dumps(self.results, indent=2) + '\n')

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['runtime', 'generation', 'builds', 'platform', 'providers', 'features'])
    parser.add_argument('--output', type=Path)
    parser.add_argument('--variant', choices=['full-static', 'selected', 'named', 'dynamic', 'server'], help='Only this consumer in builds mode')
    args = parser.parse_args()
    if args.variant and args.mode != 'builds':
        parser.error('--variant requires builds mode')
    os.umask(0o077)
    if args.output:
        args.output.mkdir(parents=True, exist_ok=False)
        output = args.output.resolve()
    else:
        parent = ROOT / '.tmp' / 'golang-perf'
        parent.mkdir(parents=True, exist_ok=True)
        output = Path(tempfile.mkdtemp(prefix=args.mode + '-', dir=parent))
    print(f'Private raw measurements: {output}', flush=True)
    measurement = Measurement(output)
    if args.mode == 'builds':
        measurement.builds(args.variant)
    else:
        getattr(measurement, args.mode)()
