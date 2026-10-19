import json,subprocess,time
from pathlib import Path
root=Path.cwd(); pending=root/'.tmp/golang-zero-candidate-server-final/results.json'
for _ in range(720):
 try:r=json.loads(pending.read_text())
 except (FileNotFoundError,json.JSONDecodeError):r=[]
 if any(x['exitCode'] for x in r):raise SystemExit('server measurement failed; verification not started')
 if len(r)==30:break
 time.sleep(10)
else:raise SystemExit('measurement not complete; verification not started')
commands=[('go-ci',['devtools','run','ci:dev','go']),('race',['devtools','run','race:dev']),('fuzz',['devtools','run','fuzz:dev']),('docs-build',['devtools','run','build:docs']),('docs-verify',['devtools','run','verify:docs']),('playground',['devtools','run','--','bash','scripts/verification/playground.sh']),('release-cross',['devtools','run','cross-build:release'])]
results=[]
for label,cmd in commands:
 path=Path('/tmp/sdkgen-zero/w11-'+label+'.log')
 start=time.monotonic()
 with path.open('w') as f:r=subprocess.run(cmd,stdout=f,stderr=subprocess.STDOUT)
 results.append({'label':label,'argv':cmd,'exitCode':r.returncode,'wallSeconds':time.monotonic()-start,'log':str(path)})
 Path('/tmp/sdkgen-zero/w11-checks.json').write_text(json.dumps(results,indent=2)+'\n')
 print(label,r.returncode,flush=True)
 if r.returncode:raise SystemExit(r.returncode)
