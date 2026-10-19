import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, chmodSync, readFileSync, rmSync, symlinkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
for (const mode of ['success', 'stale', 'missing', 'eventual', 'shasum']) {
  test(`served installers: ${mode}`, t => {
    const root = mkdtempSync(join(tmpdir(), 'sdkgen-site-'));
    t.after(() => rmSync(root, {recursive:true, force:true}));
    const scripts = { 'install.sh':'shell installer\n', 'install.ps1':'PowerShell installer\n' };
    for (const [name, bytes] of Object.entries(scripts)) writeFileSync(join(root, name), bytes);
    writeFileSync(join(root, 'count'), '0');
    writeFileSync(join(root, 'curl'), `#!/bin/sh
set -eu
while [ "$#" -gt 0 ]; do
 case "$1" in --output) output="$2"; shift 2 ;; https://*) url="$1"; shift ;; *) shift ;; esac
done
count=$(cat "$SDKGEN_SITE_FIXTURE/count"); count=$((count+1)); echo "$count" > "$SDKGEN_SITE_FIXTURE/count"
case "$url" in https://example.test/openapi-sdkgen/install.sh|https://example.test/openapi-sdkgen/install.ps1) ;; *) exit 9 ;; esac
[ "$SDKGEN_SITE_MODE" != missing ] || exit 22
if [ "$SDKGEN_SITE_MODE" = stale ] || { [ "$SDKGEN_SITE_MODE" = eventual ] && [ "$count" -le 2 ]; }; then
 printf 'old release' > "$output"
else cp "$SDKGEN_SITE_FIXTURE/\${url##*/}" "$output"; fi
`);
    writeFileSync(join(root, 'sleep'), '#!/bin/sh\nexit 0\n');
    for (const name of ['curl','sleep']) chmodSync(join(root, name), 0o755);
    if (mode === 'shasum') {
      for (const name of ['bash', 'dirname', 'mkdir', 'mktemp', 'basename', 'python3', 'rm', 'cat', 'cp', 'shasum']) {
        const resolved = spawnSync('bash', ['-c', 'command -v "$1"', 'fixture', name], {encoding:'utf8'});
        assert.equal(resolved.status, 0, `Missing fixture tool: ${name}`);
        symlinkSync(resolved.stdout.trim(), join(root, name));
      }
    }
    const hash = name => createHash('sha256').update(scripts[name]).digest('hex');
    const result = spawnSync('bash', [resolve(import.meta.dirname, 'verify-site.sh')], {encoding:'utf8', env:{...process.env, PATH:mode === 'shasum' ? root : `${root}:${process.env.PATH}`, PAGE_URL:'https://example.test/openapi-sdkgen/', SHELL_SHA:hash('install.sh'), POWERSHELL_SHA:hash('install.ps1'), SDKGEN_SITE_FIXTURE:root, SDKGEN_SITE_MODE:mode}});
    if (mode === 'success' || mode === 'eventual' || mode === 'shasum') assert.equal(result.status, 0, result.stderr);
    else { assert.notEqual(result.status, 0); assert.match(result.stderr, /do not match the release/); }
    const requests = Number(readFileSync(join(root, 'count'), 'utf8'));
    assert.ok(requests >= 2 && requests <= 12, `Unbounded or absent HTTP checks: ${requests}`);
    if (mode === 'eventual') assert.ok(requests > 2);
  });
}
