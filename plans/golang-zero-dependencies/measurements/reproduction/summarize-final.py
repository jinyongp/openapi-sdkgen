import importlib.util,json,tarfile,statistics,re,hashlib
from pathlib import Path
root=Path.cwd();out=root/'plans/golang-zero-dependencies/measurements';out.mkdir(exist_ok=True)
spec=importlib.util.spec_from_file_location('compare',root/'plans/golang-zero-dependencies/compare_measurements.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
base_provider=Path('/tmp/sdkgen-zero/provider-baseline-evidence');base_provider.mkdir(exist_ok=True)
archive=root/'plans/golang-target/measurements/2026-10-19/raw/w11-final-providers.tar.gz'
with tarfile.open(archive,'r:gz') as t:
 for name in ['results.json','basis.json']:
  matches=[x for x in t.getmembers() if x.name.endswith('/'+name) or x.name==name];assert len(matches)==1
  (base_provider/name).write_bytes(t.extractfile(matches[0]).read())
pairs={
 'runtime':('/tmp/sdkgen-zero/paired-baseline-runtime-v2','.tmp/golang-zero-candidate-runtime-final'),
 'features':('/tmp/sdkgen-zero/paired-baseline-features','.tmp/golang-zero-candidate-features-final'),
 'generation':('.tmp/golang-zero-baseline-generation','.tmp/golang-zero-candidate-generation-final'),
 'builds':('.tmp/golang-zero-baseline-builds','.tmp/golang-zero-candidate-builds-final'),
 'server':('/tmp/sdkgen-zero/paired-baseline-server','.tmp/golang-zero-candidate-server-final'),
 'providers':(str(base_provider),'.tmp/golang-zero-candidate-providers-final')}
results={}
for name,(before,after) in pairs.items():
 results[name]=m.compare(m.measurements(Path(before)),m.measurements(Path(after)))
 assert not results[name]['beforeFailures'] and not results[name]['afterFailures'],name
(out/'comparison.json').write_text(json.dumps(results,indent=2)+'\n')
parallel_roots={}
for label,path in [('before','/tmp/sdkgen-zero/regex-parallel-baseline.txt'),('initial','/tmp/sdkgen-zero/regex-parallel-initial.txt')]:
 parallel_roots[label]=m.benchmarks(Path(path).read_text())
final_rows=json.loads(Path(pairs['runtime'][1]+'/results.json').read_text());raw=next(x['stdout'] for x in final_rows if x['label']=='runtime-benchmark');parallel_roots['final']={k:v for k,v in m.benchmarks(Path(raw).read_text()).items() if k.startswith('BenchmarkRegexConcurrent/')}
assert all(len(v)==6 for v in parallel_roots.values())
parallel={label:{case:{unit:m.summary(values) for unit,values in metrics.items()} for case,metrics in cases.items()} for label,cases in parallel_roots.items()}
assert all(value['n']==5 for cases in parallel.values() for units in cases.values() for value in units.values())
(out/'parallel-comparison.json').write_text(json.dumps({'scope':'Shared-pattern harness, five calibrated samples per case. Before/initial were separate focused benchmark commands whose complete invocation metadata was not retained; final is included in the expanded runtime command. These historical comparisons are diagnostic only. Preserve them separately from the final matched-command comparison. No interleaving or statistical significance claim.','results':parallel},indent=2)+'\n')

parity=[]
with tarfile.open(archive,'r:gz') as t:
 members={Path(x.name).name:x for x in t.getmembers() if x.isfile()}
 labels=[x['label'] for x in json.loads(Path(pairs['providers'][1]+'/results.json').read_text()) if x['label'].startswith('provider-') and x['label'].endswith('-sample0')]
 def contracts(report):
  return sorted(json.dumps({k:d.get(k) for k in ['severity','code','phase','rule','action']}|{'pointer':d.get('location',{}).get('pointer')},sort_keys=True) for d in report.get('diagnostics', []))
 for label in labels:
  before=json.load(t.extractfile(members[label+'.json']));after=json.loads(Path(pairs['providers'][1],label+'.json').read_text());left,right=before['golang'],after['golang']
  assert before['inputSha256']==after['inputSha256'],label
  assert sorted(left['routes'])==sorted(right['routes']),label
  assert left['surfaces']==right['surfaces'],label
  assert contracts(left)==contracts(right),label
  parity.append({'case':label,'inputSha256':after['inputSha256'],'routes':len(right['routes']),'surfaceEntries':len(right['surfaces']),'diagnosticContracts':len(right.get('diagnostics', [])),'sameRoutesSurfacesDiagnosticContracts':True})
assert len(parity)==21
(out/'provider-parity.json').write_text(json.dumps({'cases':parity,'scope':'One/ten/full exact routes for seven pinned providers. Five samples per case have identical generated hashes. Diagnostic source paths and free-text messages/hints are excluded; severity/code/phase/rule/action/pointer are compared.'},indent=2)+'\n')

# Verify all final runs used the same production source. Added server harness and
# maintainer documentation differ; they are not generator/runtime source.
def production(name):return name in ['go.mod','go.sum','scripts/golang-compat/main.go'] or (name.startswith(('cmd/','internal/')) and name.endswith(('.go','.tmpl')) and not name.endswith('_test.go'))
bases={name:json.loads((Path(after)/'basis.json').read_text()) for name,(_,after) in pairs.items()}
anchor={k:v for k,v in bases['providers']['source'].items() if production(k)}
for name,basis in bases.items():
 if name=='runtime':
  assert {k:v for k,v in basis['source'].items() if k.startswith('internal/goruntime/') and production(k)}=={k:v for k,v in anchor.items() if k.startswith('internal/goruntime/') and production(k)},name
 else:assert {k:v for k,v in basis['source'].items() if production(k)}==anchor,name
for name,digest in anchor.items():
 import hashlib
 assert hashlib.sha256((root/name).read_bytes()).hexdigest()==digest,name
old=json.loads((base_provider/'basis.json').read_text())
old_production={k:v for k,v in old['source'].items() if production(k)}
for name,digest in old_production.items():assert hashlib.sha256((Path('/tmp/sdkgen-zero/baseline')/name).read_bytes()).hexdigest()==digest,name
assert old['go']==bases['providers']['go'] and old['node']==bases['providers']['node']
(old_source:=out/'provider-baseline-basis.json').write_text(json.dumps(old,indent=2)+'\n')
provenance={'finalProductionFiles':len(anchor),'productionFilter':'go.mod/go.sum, non-test cmd/internal Go/template source, scripts/golang-compat/main.go','providerBaselineProductionFilesVerified':len(old_production),'providerBaselineProductionDifferencesFromCab3cb8':0,'providerBaselineArchiveSha256':hashlib.sha256(archive.read_bytes()).hexdigest(),'sameProductionSourceAcrossFinalProducerModes':True,'runtimeEngineSourceMatchesFinalProducer':True,'go':bases['providers']['go'],'node':bases['providers']['node'],'providerBaselineArchive':str(archive.relative_to(root)),'providerBaselineBasis':old_source.name,'runtimeNote':'Final runtime measurements precede Go-target-only pattern interning and prepared-source reuse; runtime production sources match every final mode.','sampling':'Five sequential samples, no interleaving; setup commands separate. GNU time maximumRSSKiB is max process/child RSS, not total allocated or live heap. Provider observed group RSS is separately sampled. Off-host binaries cross-built only.','scope':'Internal local measurements, not public numeric claims.'}
(out/'provenance.json').write_text(json.dumps(provenance,indent=2)+'\n')
lines=['# Go 외부 런타임 의존 제거 전후의 로컬 비용 비교','', '외부 의존이 있던 기준 버전과 현재 후보를 같은 입력·Go/Node·옵션에서 순차 측정했다. 반복한 시간·RSS 비교는 5회 중앙값이며 전체 값과 최소/최대는 `comparison.json`에 있다. 디버그 정보를 제거한 바이너리 크기는 별도 단일 빌드 결과다. 준비·심볼·플랫폼 검사는 반복 시간 비교에 포함하지 않는다. 순서를 교차하지 않았고 통계적 유의성을 주장하지 않는다. 공개 성능 수치가 아니다.','', '| 종류 | 시나리오 | 이전 중앙값 | 현재 중앙값 | 단위 |','| --- | --- | ---: | ---: | --- |']
chosen={
 'features':[('features-small-json-selected0','elapsedSeconds'),('features-regex-heavy-selected0','elapsedSeconds'),('features-server-selected0','elapsedSeconds')],
 'generation':[('generation-10000-selected0','elapsedSeconds'),('generation-10000-selected1','elapsedSeconds'),('generation-10000-selected10','elapsedSeconds')],
 'builds':[(f'build-{v}-{state}','elapsedSeconds') for v in ['full-static','selected','named','dynamic'] for state in ['cold','warm']],
 'server':[('build-server-cold','elapsedSeconds'),('build-server-warm','elapsedSeconds')],
 'runtime':[('BenchmarkRegex/ascii/execute','ns/op'),('BenchmarkRegex/unicode/execute','ns/op'),('BenchmarkURL/ascii','ns/op'),('BenchmarkURL/long-idn','ns/op'),('BenchmarkNormalization/ascii','ns/op'),('BenchmarkNormalization/long-combining','ns/op')]}
for mode,cases in chosen.items():
 for case,unit in cases:
  row=results[mode]['comparisons'][case][unit];lines.append(f"| `{mode}` | `{case}` | {row['before']['median']:,.4g} | {row['after']['median']:,.4g} | {'초' if unit=='elapsedSeconds' else unit} |")
lines+=['','| 제공자 전체 생성 | 이전 시간(초) | 현재 시간(초) | 이전 RSS(MiB) | 현재 RSS(MiB) | 현재 소스(MiB) |','| --- | ---: | ---: | ---: | ---: | ---: |']
for case,metrics in results['providers']['comparisons'].items():
 if case.endswith('-selected0'):
  value=lambda unit,side:metrics[unit][side]['median']
  lines.append(f"| {case.removeprefix('provider-').removesuffix('-selected0')} | {value('elapsedSeconds','before'):.2f} | {value('elapsedSeconds','after'):.2f} | {value('maximumRSSKiB','before')/1024:.1f} | {value('maximumRSSKiB','after')/1024:.1f} | {value('generatedBytes','after')/1048576:.2f} |")
lines+=['','| 소비자 | 이전 바이너리(MiB) | 현재 바이너리(MiB) |','| --- | ---: | ---: |']
for mode,variants in [('builds',['full-static','selected','named','dynamic']),('server',['server'])]:
 for variant in variants:
  v=results[mode]['comparisons']['stripped-'+variant]['binaryBytes'];lines.append(f"| {variant} | {v['before']['median']/1048576:.2f} | {v['after']['median']/1048576:.2f} |")
lines+=['', '작은 명세는 자체 URL/Unicode 소스를 준비하는 고정 비용이 남는다. 중복 정렬 제거로 초기 후보보다 줄었지만 이전 버전보다 느린 경우도 유지해서 기록했다. 단순 정규식도 자체 VM 실행 비용이 남으며 모든 시나리오의 개선을 주장하지 않는다. 병적 정규식의 작업 한도 도달 시간은 서로 다른 엔진의 작업 단위 차이가 있으므로 정상 처리 속도 비교로 해석하지 않는다.','', '반복 패턴 공유·NFC 빠른 경로·임시 상태 캐시 경합 제거의 전후 자료는 초기·중간 후보의 압축 기록과 프로파일에 있다. HTTP 압축 해제 제거 전후의 벤치마크는 책임 범위가 달라 직접 비교하지 않는다. 최초 항목 시간·처리량·할당량·GC 전후 힙 자료는 실행 벤치마크의 원시 출력에 보존했다.','', '6개 플랫폼 교차 빌드, Linux에서의 네이티브 실행, Playground WASM 빌드 및 전체·선택·독립·계산 조회·서버 소비자 빌드가 성공했다. 서버 빌드 사례는 동적 스키마 컴파일과 검증을 실행한다. 실제 HTTP 핸들러는 별도 통합 검사에서 검증한다. 게시된 npm의 빈 네이티브 캐시 검사는 실제 게시 후 실행할 항목이며 이 로컬 결과에 포함하지 않는다.','']
(out/'README.md').write_text('\n'.join(lines))
print('Wrote comparison/provenance/report; final production files',len(anchor),'baseline basis keys',list(old))
