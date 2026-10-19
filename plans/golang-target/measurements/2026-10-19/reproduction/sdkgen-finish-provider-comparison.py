import json,hashlib,statistics
from pathlib import Path
root=Path('/home/jinyongp/workspace/jinyongp/openapi-sdkgen');out=Path(Path('/tmp/sdkgen-provider-comparison-location').read_text());report=json.loads((out/'comparison.json').read_text());raw=json.loads((out/'results.json').read_text())
# Existing full correctness audit checks the same byte output, independently of performance.
audit=Path('/tmp/sdkgen-go-implementation/w11-corpus-final-fixed');verified=[];missing=[]
for x in raw:
 if not x['label'].startswith('typescript-'):continue
 d=json.loads(Path(x['report']).read_text());v=d['typescript'];selection=d.get('selection',[])
 absent=sorted(set(selection)-set(v.get('routes',[])))
 if absent:missing.append({'label':x['label'],'routes':absent})
for w in report['workloads']:
 if w['selected']=='full':
  original=json.loads((audit/('regression-'+w['provider']+'.json')).read_text())
  for target in ['typescript','golang']:
   assert w[target]['artifactSha256']==original[target]['artifactSha256'],(w['provider'],target)
  verified.append(w['provider'])
assert len(report['workloads'])==14,len(report['workloads'])
assert len([x for x in raw if x['label'].startswith('typescript-')])==70
assert not missing,missing
report['validation']={'typescriptSamples':70,'golangReusedSamples':70,'inputHashesMatched':True,'fiveSampleDeterminismPerWorkload':True,'fullArtifactHashesMatchPriorCorrectnessAudit':verified,'selectedDirectRoutesAllEmitted':not missing,'cpuMetrics':'GNU time userSeconds/systemSeconds retained per sample in results.json, not used as elapsed time.'}
report['limitations']=['Earlier Go run versus fresh TypeScript run; not interleaved. Same production source files, pinned inputs and runtime flags; uncontrolled host load may differ.','Five samples per case; memory is maximum process RSS, not allocations or retained Go heap.','Both enable server addon; metadata addon disabled. Exact first ten previously emitted Go route keys used on both targets.','Render/format/hash only, no generated source disk publication, consumer compilation, TypeScript type checking or bundling.','Time uses GNU time elapsedSeconds for both targets, excluding the 0.5 second monitor polling delay included in the earlier prose table.']
(out/'comparison.json').write_text(json.dumps(report,indent=2)+'\n')
lines=['# Provider generation comparison','',*['- '+x for x in report['limitations']],'']
for mode in ['full',10]:
 lines+=['## '+('Full generation' if mode=='full' else 'Ten exact routes'),'', '| Provider | TypeScript seconds median [min–max] | Go seconds median [min–max] | TypeScript peak RSS MiB | Go peak RSS MiB | Go / TypeScript time | TypeScript source MiB | Go source MiB |','| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |']
 for w in report['workloads']:
  if w['selected']!=mode:continue
  ts,go=w['typescript'],w['golang']
  def t(v):return f"{v['secondsMedian']:.2f} [{v['secondsMin']:.2f}–{v['secondsMax']:.2f}]"
  lines.append(f"| {w['provider']} | {t(ts)} | {t(go)} | {ts['peakRSSKiBMax']/1024:.0f} | {go['peakRSSKiBMax']/1024:.0f} | {go['secondsMedian']/ts['secondsMedian']:.2f}× | {ts['bytes']/1024**2:.1f} | {go['bytes']/1024**2:.1f} |")
 lines.append('')
(out/'comparison.md').write_text('\n'.join(lines)+'\n');print('\n'.join(lines));print('SOURCE '+str(out/'comparison.md'))
