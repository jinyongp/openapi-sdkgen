import importlib.util,json,shutil,hashlib
from pathlib import Path
root=Path.cwd();out=root/'plans/golang-zero-dependencies/measurements'
def module(name,file):
 s=importlib.util.spec_from_file_location(name,file);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
archive=module('archive',out.parent/'record_measurements.py');compare=module('compare',out.parent/'compare_measurements.py')
index=json.loads((out/'index.json').read_text())
for name,path in [('parallel-final-failed',root/'.tmp/golang-zero-final-parallel'),('parallel-final',root/'.tmp/golang-zero-final-parallel-v2')]:index['archives'].append(archive.record(out,name,path))
checks=json.loads(Path('/tmp/sdkgen-zero/w11-checks.json').read_text());assert len(checks)==7 and all(x['exitCode']==0 for x in checks)
bundle=Path('/tmp/sdkgen-zero/final-verification-bundle');bundle.mkdir(exist_ok=False)
shutil.copyfile('/tmp/sdkgen-zero/w11-checks.json',bundle/'checks.json')
for entry in checks:shutil.copyfile(entry['log'],bundle/Path(entry['log']).name)
profile_binaries=[]
for name in ['nfc-before.test','regex-before.test']:
 p=Path('/tmp/sdkgen-zero')/name;profile_binaries.append({'path':str(p),'bytes':p.stat().st_size,'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'archivePolicy':'Executable remains in disposable directory; source snapshots and symbolized profiles are archived.'})
(bundle/'profile-binaries.json').write_text(json.dumps(profile_binaries,indent=2)+'\n')
index['archives'].append(archive.record(out,'verification',bundle));(out/'index.json').write_text(json.dumps(index,indent=2)+'\n')
repro=out/'reproduction';repro.mkdir(exist_ok=False)
for name in ['archive-all.py','summarize-final.py','parallel-final.py','final-checks.py']:shutil.copyfile(Path('/tmp/sdkgen-zero')/name,repro/name)
rows=json.loads((root/'.tmp/golang-zero-final-parallel-v2/results.json').read_text());values={}
for side in ['baseline','candidate']:
 row=next(x for x in rows if x['label']=='parallel-'+side);assert row['exitCode']==0
 data=compare.benchmarks(Path(row['stdout']).read_text());assert len(data)==6
 values[side]={name:{unit:compare.summary(samples) for unit,samples in units.items()} for name,units in data.items()}
 assert all(v['n']==5 for units in values[side].values() for v in units.values())
(out/'matched-parallel-comparison.json').write_text(json.dumps({'scope':'Identical command and benchmark source; five calibrated 100ms samples; sequential, after all CI checks. Only per-case benchmark metrics are compared.','results':values},indent=2)+'\n')
with (out/'README.md').open('a') as f:
 f.write('\n## 동일 명령으로 재검증한 동시 실행\n\n이전의 별도 진단 실행은 정확한 명령 메타데이터가 남지 않아 진단 자료로만 보존했다. 모든 CI가 끝난 뒤 동일한 소스·명령으로 다시 측정했다. 각 값은 5회 중앙값이다. 전체 범위·할당량은 `matched-parallel-comparison.json`, 정확한 명령·소스 해시는 `parallel-final.tar.gz`에 있다.\n\n| 패턴·동시 작업 수 | 이전 ns/op | 현재 ns/op |\n| --- | ---: | ---: |\n')
 for name,units in values['baseline'].items():f.write(f"| `{name.removeprefix('BenchmarkRegexConcurrent/')}` | {units['ns/op']['median']:.2f} | {values['candidate'][name]['ns/op']['median']:.2f} |\n")
 f.write('\n## 원시 기록과 재현\n\n`index.json`은 초기·중간·최종·실패·의도적 중단·검증 기록의 압축 파일과 해시를 연결한다. 실패한 명령은 최종 성공 표본에 포함하지 않았다. `record_measurements.py --verify measurements/*.tar.gz`로 파일·해시·JSON 복원을 확인할 수 있다. JSON 공백은 정규화했으며 원래 바이트 수·해시와 복원한 값의 해시를 함께 보존했다. stdout·stderr·프로파일은 원시 바이트를 보존한다. 생성 바이너리·빌드 캐시는 임시 디렉터리에 남기고 해시와 크기를 기록했다.\n\n각 압축 파일은 별도 디렉터리에 풀어 사용한다. 예를 들어 두 runtime 기록을 푼 디렉터리에 `compare_measurements.py BEFORE AFTER comparison.json`을 적용하면 원래 임시 경로 없이 사례별 통계를 재현할 수 있다. 빌드·규모별 생성의 기준 기록은 `../baseline`, 제공자 기준 기록은 `../../golang-target/measurements/2026-10-19/raw/w11-final-providers.tar.gz`를 재사용한다. 제공자 기준의 생산 코드331개가 `cab3cb8` 체크아웃과 일치함을 확인했다. `provenance.json`에 비교 범위와 버전을 기록했다.\n\n`reproduction`에는 실제 기록에 사용한 실행 스크립트를 보존했다. 원래 임시 경로가 포함되므로 새 환경에서는 입력·출력 디렉터리를 바꿔 실행한다. 이전 소비자에만 기존 go.sum을 복사했고, 이전 정규화 벤치마크는 기존 Unicode 정규화 모듈, BodyLease는 이전 Encoded 인자를 사용했다. 소비자 cold 빌드는 각각 빈 GOCACHE에서 시작하며 외부 모듈 다운로드 시간은 측정하지 않았다. 제공자 생성은 포맷·해시하는 sink를 사용해 대규모 소스 쓰기를 제외한다. 최종 생성 모드의 생산 코드는 동일하다. 실행 벤치마크는 이후 생성 대상의 공유·방출 보완보다 앞서 실행했지만 자체 엔진 Go 소스는 최종 코드와 동일하다.\n')
evidence={'scope':'W10 candidate implementation and verification; local performance data, not a public benchmark claim.','finalCommands':{'runtime':9,'features':49,'generation':33,'builds':117,'platform':10,'providers':107,'server':30,'matchedParallel':3},'archives':len(index['archives']),'sourceProvenance':'measurements/provenance.json','comparison':'measurements/comparison.json','providerParity':'measurements/provider-parity.json','matchedParallel':'measurements/matched-parallel-comparison.json','checks':checks,'reviewedAreas':['measurement inputs/toolchain/source identity','all seven providers and full/one/ten selection','generated surfaces and diagnostic contracts','cold/warm consumers including server schema','actual dependency/source/symbol reachability','normalization and Unicode generation','matcher cancellation/budget/anchoring/concurrency','prepared source ownership and deterministic emission','header decoder callers and static program sharing','HTTP/stream allocation, latency and lifetime diagnostics','six consumer and six release cross-builds; native and WASM integration'],'releaseOnlyChecks':[{'check':'Published npm with empty native cache','status':'not run before publication','reason':'Requires the actual published candidate; native candidate and release workflow wiring passed. No push/publication/remote dispatch authorized or performed.'}]}
(out.parent/'w10-evidence.json').write_text(json.dumps(evidence,indent=2)+'\n')
print('Archived',len(index['archives']),'bundles; all final checks and matched parallel cases passed')
