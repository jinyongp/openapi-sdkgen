import importlib.util,json,hashlib,shutil
from pathlib import Path
root=Path.cwd();out=root/"plans/golang-zero-dependencies/measurements";out.mkdir(exist_ok=True)
spec=importlib.util.spec_from_file_location("archive",root/"plans/golang-zero-dependencies/record_measurements.py");m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
runs={
"baseline-runtime-failed":"/tmp/sdkgen-zero/paired-baseline-runtime",
"baseline-runtime":"/tmp/sdkgen-zero/paired-baseline-runtime-v2",
"baseline-features":"/tmp/sdkgen-zero/paired-baseline-features",
"baseline-server":"/tmp/sdkgen-zero/paired-baseline-server",
}
for name in ["runtime-initial","runtime","runtime-final","features-initial","features","features-final","generation","generation-final","builds","builds-final","platform","platform-final","providers","providers-final","server-final"]:
 runs["candidate-"+name]=".tmp/golang-zero-candidate-"+name
entries=[]
for name,path in runs.items():
 entry=m.record(out,name,Path(path));entries.append(entry);print(name,entry["files"],flush=True)
extra=Path("/tmp/sdkgen-zero/w10-evidence-bundle");extra.mkdir(exist_ok=False)
private=Path("/tmp/sdkgen-zero")
for p in private.iterdir():
 if p.is_file() and (p.name.startswith(("w10-","nfc-","regex-")) or p.name=="baseline-measure-before-server.py") and p.suffix!=".test":
  shutil.copyfile(p,extra/p.name)
for name in ["runtime_bench.go.tmpl","runtime-fixture.json","measure.py"]:
 shutil.copyfile(private/"baseline/scripts/perf/golang"/name,extra/("baseline-"+name))
for p in (root/".tmp/runs/go.sh.2yU0V9").rglob("*"):
 if p.is_file():shutil.copyfile(p,extra/("initial-all-tests-"+p.name))
entries.append(m.record(out,"diagnostics",extra))
(out/"index.json").write_text(json.dumps({"archives":entries,"scope":"Local diagnostics. Historical and interrupted runs are preserved, not passing final samples. JSON whitespace is normalized; original bytes/hash and lossless parsed reconstruction are recorded."},indent=2)+"\n")
