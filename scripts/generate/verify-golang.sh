#!/usr/bin/env bash
# Owns only the disposable SDK copied by check.sh; never modifies a host module.
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
checked="$1"
python3 - "$checked" <<'PY'
import json,re,sys
from pathlib import Path
root=Path(sys.argv[1])
client=(root/'client.go').read_text()
match=re.search(r'"([^"\n]+)/internal/core"',client)
package=re.search(r'^package ([A-Za-z_][A-Za-z_0-9]*)$',client,re.M)
if not match or not package:raise SystemExit('generated Go client has no canonical import root or package')
module=match[1]
(root/'go.mod').write_text('module '+module+'\n\ngo 1.27.0\n')
probe='''package PACKAGE_test
import("context";"testing";sdk MODULE;TYPES)
func TestGeneratedConsumer(t *testing.T){client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"https://example.test"});if err!=nil||client==nil{t.Fatal(err)};value,err:=types.ParseValue([]byte("{\\"count\\":2}"));if err!=nil{t.Fatal(err)};encoded,err:=value.MarshalJSON();if err!=nil||string(encoded)!="{\\"count\\":2}"{t.Fatal(string(encoded),err)};body:=types.BinaryFromBytes([]byte("payload"));reader,err:=body.Open(context.Background());if err!=nil{t.Fatal(err)};if err:=reader.Close();err!=nil{t.Fatal(err)}}
'''.replace('PACKAGE',package[1]).replace('MODULE',json.dumps(module)).replace('TYPES',json.dumps(module+'/types'))
(root/'generate_check_test.go').write_text(probe)
PY
status=0
# GOWORK prevents an enclosing worktree from selecting a user's module.
GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0 GOMODCACHE="$LOG_DIR/module-cache" run_step 'compile and run generated Go SDK' "${SDKGEN_GENERATE_CHECK_GO_BIN:-go}" -C "$checked" test -mod=readonly -timeout=45s ./... || status=$?
if ((status == 0)); then
  GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0 GOMODCACHE="$LOG_DIR/module-cache" run_step 'verify generated Go module graph' python3 - "$checked" "${SDKGEN_GENERATE_CHECK_GO_BIN:-go}" <<'PYGRAPH' || status=$?
import json,subprocess,sys
root,go=sys.argv[1:]
modules=subprocess.check_output([go,'-C',root,'list','-m','-json','all'],text=True)
module=json.loads(modules)
if not module.get('Main') or module.get('Replace'):raise SystemExit('generated consumer must have exactly one main module')
packages=subprocess.check_output([go,'-C',root,'list','-deps','-f','{{if not .Standard}}{{.ImportPath}}{{end}}','./...'],text=True).splitlines()
for package in packages:
 if package and package!=module['Path'] and not package.startswith(module['Path']+'/'):raise SystemExit('unexpected generated SDK dependency: '+package)
print('Generated SDK uses one main module and only standard-library dependencies.')
PYGRAPH
fi
script_note "Go consumer exit: $status"
exit "$status"
