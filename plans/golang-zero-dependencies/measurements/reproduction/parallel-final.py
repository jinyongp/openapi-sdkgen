import importlib.util,json,shutil
from pathlib import Path
root=Path.cwd();spec=importlib.util.spec_from_file_location('measure',root/'scripts/perf/golang/measure.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
out=root/'.tmp/golang-zero-final-parallel-v2';out.mkdir(exist_ok=False)
measurement=m.Measurement(out)
bench=root/'scripts/perf/golang/regex_parallel_bench.go.tmpl'
modules={'baseline':Path('/tmp/sdkgen-zero/paired-baseline-runtime-v2/runtime'),'candidate':root/'.tmp/golang-zero-candidate-runtime-final/runtime'}
inputs={}
for name,module in modules.items():
 dest=module/'api'/('regex_parallel_test.go' if name=='baseline' else 'regex_parallel_bench_test.go')
 shutil.copyfile(dest,out/(name+'-prior-benchmark.go'))
 shutil.copyfile(bench,dest)
 shutil.copyfile(bench,out/(name+'-matched-benchmark.go'))
 inputs[name]={'sdk':m.tree(module/'api'),'moduleSha256':m.digest(module/'go.mod'),'moduleSumSha256':m.digest(module/'go.sum') if (module/'go.sum').exists() else None,'benchmarkSha256':m.digest(dest),'originBasis':str(module.parent/'basis.json'),'originBasisSha256':m.digest(module.parent/'basis.json')}
(out/'inputs.json').write_text(json.dumps(inputs,indent=2)+'\n')
assert inputs['baseline']['benchmarkSha256']==inputs['candidate']['benchmarkSha256']
for name,module in modules.items():
 measurement.run('parallel-'+name,['go','test','-mod=readonly','-run','^$','-bench','^BenchmarkRegexConcurrent$','-benchmem','-benchtime=100ms','-count=5','./api'],module)
print('Matched parallel cases completed')
