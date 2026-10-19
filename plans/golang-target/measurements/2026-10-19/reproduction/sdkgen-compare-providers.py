import importlib.util,json,os,time,hashlib,statistics,datetime
from pathlib import Path
ROOT=Path('/home/jinyongp/workspace/jinyongp/openapi-sdkgen')
output=ROOT/'.tmp'/('provider-target-comparison-'+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ'))
output.mkdir(parents=True,exist_ok=False)
source=(ROOT/'scripts/golang-compat/main.go').read_text()
source=source.replace('if *mode != "parity" && *mode != "golang-generation" {','if *mode != "parity" && *mode != "golang-generation" && *mode != "typescript-generation" {')
source=source.replace('if !*goOnly && *mode == "parity" {','if *mode == "typescript-generation" { runtime.GC() }\n\tif !*goOnly && (*mode == "parity" || *mode == "typescript-generation") {')
marker='\tif *partialOutput != "" {'
extra='''\tif *mode == "typescript-generation" {
        result.Pass = !result.TypeScript.Blocked
        encoded, err := json.MarshalIndent(result, "", "  ")
        if err != nil { return err }
        if err := os.WriteFile(*output, append(encoded, '\\n'), 0600); err != nil { return err }
        if !result.Pass { return fmt.Errorf("TypeScript generation was blocked") }
        return nil
    }
'''
assert source.count(marker)==1
source=source.replace(marker,extra+marker)
auditor_source=output/'auditor';auditor_source.mkdir();(auditor_source/'main.go').write_text(source)
spec=importlib.util.spec_from_file_location('measurement',ROOT/'scripts/perf/golang/measure.py');mod=importlib.util.module_from_spec(spec);spec.loader.exec_module(mod)
m=mod.Measurement(output);auditor=output/'typescript-auditor'
m.run('build-typescript-auditor',['go','build','-o',auditor,auditor_source/'main.go'],ROOT)
prior=Path('/tmp/sdkgen-go-implementation/w11-final-providers/results.json');baseline=json.loads(prior.read_text())
m.basis.update({'purpose':'Go and TypeScript provider generation comparison, matched original input, server addon, sink-only render/format/hash; local diagnostics.', 'typescriptHarnessSourceSha256':mod.digest(auditor_source/'main.go'),'typescriptAuditorSha256':mod.digest(auditor),'baselineResults':str(prior),'baselineResultsSha256':mod.digest(prior),'baselineSourceManifest':str(ROOT/'plans/golang-target/w11-verified-code.json'),'sampling':'5 fresh TypeScript processes per full/10-route case; existing 5 Go samples; sequential execution; baseline from earlier run, no interleaving','deadlineSeconds':1200})
(output/'basis.json').write_text(json.dumps(m.basis,indent=2)+'\n')
Path('/tmp/sdkgen-provider-comparison-location').write_text(str(output))
env={**m.environment,'GOMEMLIMIT':'5GiB','GOMAXPROCS':'2'}
started=time.monotonic();summaries=[]
# Complete ordinary providers first; record the largest provider last.
providers=['github','stripe','cloudflare','gitlab','digitalocean','twilio','microsoft-graph-beta']
for provider in providers:
 for selected in [0,10]:
  old=[x for x in baseline if x['label'].startswith(f'provider-{provider}-selected{selected}-sample')]
  assert len(old)==5,(provider,selected,len(old))
  selectors=[]
  argv=old[0]['argv']
  if '--route' in argv:selectors=argv[argv.index('--route'):]
  results=[];hashes=[]
  for sample in range(5):
   remaining=1200-(time.monotonic()-started)
   if remaining<=1:raise SystemExit('20 minute measurement budget exhausted; completed samples preserved')
   report=output/f'{provider}-selected{selected}-sample{sample}.json'
   x=m.run(f'typescript-{provider}-selected{selected}-sample{sample}',[auditor,'--manifest',ROOT/'test/compatibility/regression.json','--corpus-root',ROOT/'.tmp/compatibility-regression','--document',provider,'--server','--mode','typescript-generation','--output',report,*selectors],ROOT,env=env,timeout=min(900,remaining))
   d=json.loads(report.read_text());assert d['pass'] and d['mode']=='typescript-generation';v=d['typescript']
   assert d['inputSha256']==old[sample]['inputSha256']
   x.update(inputSha256=d['inputSha256'],report=str(report),reportSha256=mod.digest(report),generatedFiles=v['files'],generatedBytes=v['bytes'],generatedSha256=v['artifactSha256'])
   (output/'results.json').write_text(json.dumps(m.results,indent=2)+'\n');results.append(x);hashes.append(v['artifactSha256'])
  assert len(set(hashes))==1,'Non-deterministic TypeScript generation'
  def reduce(xs):
   seconds=[x['metrics']['elapsedSeconds'] for x in xs]
   return {'samples':len(xs),'secondsMedian':statistics.median(seconds),'secondsMin':min(seconds),'secondsMax':max(seconds),'peakRSSKiBMax':max(x['metrics']['maximumRSSKiB'] for x in xs),'files':xs[0]['generatedFiles'],'bytes':xs[0]['generatedBytes'],'artifactSha256':xs[0]['generatedSha256']}
  summaries.append({'provider':provider,'selected':selected or 'full','typescript':reduce(results),'golang':reduce(old)})
  (output/'comparison.json').write_text(json.dumps({'basis':str(output/'basis.json'),'metric':'GNU time child elapsed seconds and maximum RSS; excludes 0.5 second monitor polling delay; Go raw metrics reaggregated identically.','scope':'Full and 10-route selection. No disk publication, SDK compile, TypeScript compiler or bundler.','workloads':summaries},indent=2)+'\n')
print('COMPARISON COMPLETE '+str(output),flush=True)
