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
pinned={'github.com/andybalholm/brotli':'v1.2.6','github.com/klauspost/compress':'v1.20.3','github.com/go-quickjs/go-quickjs':'v0.27.0','github.com/nlnwa/whatwg-url':'v0.6.2','golang.org/x/net':'v0.61.0','golang.org/x/text':'v0.43.0','github.com/bits-and-blooms/bitset':'v1.20.0'}
imports=set()
for p in root.rglob('*.go'):
 text=p.read_text()
 for block in re.findall(r'(?ms)^import \((.*?)^\)',text):
  imports.update(re.findall(r'"([^"\n]+)"',block))
 imports.update(re.findall(r'(?m)^import (?:[\w.]+ )?"([^"\n]+)"',text))
required={}
for imported in imports:
 if imported.startswith(module+'/') or '.' not in imported.split('/')[0]:continue
 owners=[name for name in pinned if imported==name or imported.startswith(name+'/')]
 if not owners:raise SystemExit('generated Go dependency has no pinned verifier version: '+imported)
 for name in owners:required[name]=pinned[name]
# These are the selected URL parser's transitive requirements, pinned as well.
if 'github.com/nlnwa/whatwg-url' in required:
 for name in ['golang.org/x/net','golang.org/x/text','github.com/bits-and-blooms/bitset']:required[name]=pinned[name]
(root/'go.mod').write_text('module '+module+'\n\ngo 1.27.0\n\nrequire (\n'+''.join(' '+name+' '+version+'\n' for name,version in sorted(required.items()))+')\n')
probe='''package PACKAGE_test
import("context";"testing";sdk MODULE;TYPES)
func TestGeneratedConsumer(t *testing.T){client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"https://example.test"});if err!=nil||client==nil{t.Fatal(err)};value,err:=types.ParseValue([]byte("{\\"count\\":2}"));if err!=nil{t.Fatal(err)};encoded,err:=value.MarshalJSON();if err!=nil||string(encoded)!="{\\"count\\":2}"{t.Fatal(string(encoded),err)};body:=types.BinaryFromBytes([]byte("payload"));reader,err:=body.Open(context.Background());if err!=nil{t.Fatal(err)};if err:=reader.Close();err!=nil{t.Fatal(err)}}
'''.replace('PACKAGE',package[1]).replace('MODULE',json.dumps(module)).replace('TYPES',json.dumps(module+'/types'))
(root/'generate_check_test.go').write_text(probe)
PY
status=0
# GOWORK prevents an enclosing worktree from selecting a user's module.
GOWORK=off GOTOOLCHAIN=local run_step 'compile and run generated Go SDK' "${SDKGEN_GENERATE_CHECK_GO_BIN:-go}" -C "$checked" test -mod=mod -timeout=45s ./... || status=$?
script_note "Go consumer exit: $status"
exit "$status"
