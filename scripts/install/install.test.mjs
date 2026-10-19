import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync, chmodSync, existsSync, readdirSync, symlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import test from "node:test";

const installer = resolve(import.meta.dirname, "install.sh");
function fixture(t, os = "Linux", arch = "x86_64") {
  const root = mkdtempSync(join(tmpdir(), "sdkgen-install-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const bin = join(root, "bin");
  const files = join(root, "files");
  const home = join(root, "home with spaces");
  for (const dir of [bin, files, home]) mkdirSync(dir);
  const executable = "#!/bin/sh\nprintf 'fixture CLI 1.2.3\\n'\n";
  writeFileSync(join(files, "openapi-sdkgen"), executable);
  chmodSync(join(files, "openapi-sdkgen"), 0o755);
  assert.equal(spawnSync("tar", ["-czf", join(root, "archive.tar.gz"), "-C", files, "openapi-sdkgen"]).status, 0);
  const asset = `openapi-sdkgen_1.2.3_${os === "Darwin" ? "darwin" : "linux"}_${["arm64", "aarch64"].includes(arch) ? "arm64" : "amd64"}.tar.gz`;
  const digest = createHash("sha256").update(readFileSync(join(root, "archive.tar.gz"))).digest("hex");
  writeFileSync(join(root, "checksums.txt"), `${digest}  ${asset}\n`);
  const mocks = {
    uname: '#!/bin/sh\ncase "$1" in -s) echo "$FIXTURE_OS" ;; -m) echo "$FIXTURE_ARCH" ;; *) exit 1 ;; esac\n',
    curl: `#!/bin/sh
set -eu
output=''
while [ "$#" -gt 0 ]; do
  case "$1" in --output) output="$2"; shift 2 ;; https://*) url="$1"; shift ;; *) shift ;; esac
done
[ "\${FIXTURE_FAIL:-}" != download ] || exit 22
case "$url" in
  "https://github.com/jinyongp/openapi-sdkgen/releases/download/v1.2.3/checksums.txt") cp "$FIXTURE_ROOT/checksums.txt" "$output" ;;
  "https://github.com/jinyongp/openapi-sdkgen/releases/download/v1.2.3/$FIXTURE_ASSET") cp "$FIXTURE_ROOT/archive.tar.gz" "$output" ;;
  *) echo "Unexpected URL: $url" >&2; exit 1 ;;
esac
`,
  };
  for (const [name, text] of Object.entries(mocks)) { writeFileSync(join(bin, name), text); chmodSync(join(bin, name), 0o755); }
  const env = { ...process.env, HOME: home, PATH: `${bin}:${process.env.PATH}`, FIXTURE_ROOT: root, FIXTURE_ASSET: asset, FIXTURE_OS: os, FIXTURE_ARCH: arch };
  const run = (...args) => spawnSync("sh", [installer, ...args], { env, encoding: "utf8" });
  return { root, home, env, run, executable, asset, digest, destination: join(home, ".local/bin/openapi-sdkgen") };
}

for (const [os, arch] of [["Linux", "x86_64"], ["Linux", "aarch64"], ["Darwin", "x86_64"], ["Darwin", "arm64"]]) {
  test(`installs ${os}/${arch} at the user default and can reinstall`, t => {
    const f = fixture(t, os, arch);
    for (let i = 0; i < 2; i++) {
      const result = f.run("--version", "v1.2.3");
      assert.equal(result.status, 0, result.stderr);
      let previous = -1;
      for (const message of [`Installing openapi-sdkgen 1.2.3`, `Destination: ${f.destination}`, 'Downloading release checksums', `Downloading ${f.asset}`, 'Verifying SHA-256', 'Extracting executable', 'Installing executable', 'Installed openapi-sdkgen 1.2.3', 'Run now:']) {
        const position = result.stdout.indexOf(message);
        assert.ok(position > previous, `Missing or out-of-order installer guidance: ${message}`);
        previous = position;
      }
      assert.equal(readFileSync(f.destination, "utf8"), f.executable);
      assert.equal(spawnSync(f.destination, [], { encoding: "utf8" }).stdout.trim(), "fixture CLI 1.2.3");
      assert.deepEqual(readdirSync(join(f.home, ".local/bin")), ["openapi-sdkgen"]);
    }
  });
}
test("stamped installer defaults to its release version and accepts a custom directory", t => {
  const f = fixture(t);
  const stamped = join(f.root, "install.sh");
  const build = spawnSync("bash", [resolve(import.meta.dirname, "build.sh"), "1.2.3", f.root], { encoding: "utf8" });
  assert.equal(build.status, 0, build.stderr);
  assert.equal(readFileSync(join(f.root, "install.ps1"), "utf8").includes("@VERSION@"), false);
  const result = spawnSync("sh", ["-s", "--", "--install-dir", "relative path"], { input: readFileSync(stamped), cwd: f.home, env: f.env, encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(join(f.home, "relative path/openapi-sdkgen"), "utf8"), f.executable);
});
for (const mode of ["download", "tamper", "missing", "duplicate", "bad-archive"]) {
  test(`${mode} failure preserves the old installation`, t => {
    const f = fixture(t);
    mkdirSync(join(f.home, ".local/bin"), { recursive: true });
    writeFileSync(f.destination, "old installation");
    if (mode === "download") f.env.FIXTURE_FAIL = "download";
    if (mode === "tamper") writeFileSync(join(f.root, "archive.tar.gz"), "tampered");
    if (mode === "missing") writeFileSync(join(f.root, "checksums.txt"), `${f.digest}  different.tar.gz\n`);
    if (mode === "duplicate") writeFileSync(join(f.root, "checksums.txt"), `${f.digest}  ${f.asset}\n${f.digest}  ${f.asset}\n`);
    if (mode === "bad-archive") {
      writeFileSync(join(f.root, "archive.tar.gz"), "not an archive");
      const hash = createHash("sha256").update("not an archive").digest("hex");
      writeFileSync(join(f.root, "checksums.txt"), `${hash}  ${f.asset}\n`);
    }
    const result = f.run("--version", "1.2.3");
    assert.notEqual(result.status, 0);
    assert.equal(result.stdout.includes('Installed openapi-sdkgen'), false);
    if (mode === 'download') assert.match(result.stderr, /connection.*release version/);
    if (mode === 'tamper') assert.match(result.stderr, /Checksum mismatch/);
    assert.equal(readFileSync(f.destination, "utf8"), "old installation");
    assert.deepEqual(readdirSync(join(f.home, ".local/bin")), ["openapi-sdkgen"]);
  });
}
test("rejects unsupported platforms, bad arguments and link destinations", t => {
  const f = fixture(t);
  for (const args of [[], ["--version"], ["--version", "../1.2.3"], ["--install-dir"], ["--unknown"]]) assert.notEqual(f.run(...args).status, 0);
  f.env.FIXTURE_OS = "FreeBSD";
  assert.notEqual(f.run("--version", "1.2.3").status, 0);
  f.env.FIXTURE_OS = "Linux";
  f.env.FIXTURE_ARCH = "i686";
  assert.notEqual(f.run("--version", "1.2.3").status, 0);
  assert.equal(existsSync(f.destination), false);
  f.env.FIXTURE_ARCH = "x86_64";
  mkdirSync(join(f.home, ".local/bin"), { recursive: true });
  const target = join(f.root, "old"); writeFileSync(target, "old"); symlinkSync(target, f.destination);
  assert.notEqual(f.run("--version", "1.2.3").status, 0);
  assert.equal(readFileSync(target, "utf8"), "old");
});
test("printed commands handle quotes and spaces and select the new installation ahead of an old command", t => {
  const f = fixture(t);
  const directory = join(f.root, "tools ' $literal");
  const oldCommand = join(f.root, "bin/openapi-sdkgen");
  writeFileSync(oldCommand, '#!/bin/sh\necho old\n'); chmodSync(oldCommand, 0o755);
  f.env.PATH += `:${directory}`;
  const result = f.run('--version', '1.2.3', '--install-dir', directory);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /Another openapi-sdkgen currently takes precedence/);
  const lines = result.stdout.split('\n');
  const immediate = lines[lines.indexOf('Run now:') + 1].trim();
  const immediateRun = spawnSync('sh', ['-c', immediate], {env: f.env, encoding:'utf8'});
  assert.equal(immediateRun.status, 0, immediateRun.stderr);
  assert.equal(immediateRun.stdout.trim(), 'fixture CLI 1.2.3');
  const pathCommand = lines.find(line => line.trimStart().startsWith('export PATH=')).trim();
  const byName = spawnSync('sh', ['-c', `${pathCommand}\nopenapi-sdkgen --version`], {env:f.env, encoding:'utf8'});
  assert.equal(byName.status, 0, byName.stderr);
  assert.equal(byName.stdout.trim(), 'fixture CLI 1.2.3');
  assert.equal(spawnSync(oldCommand, [], {encoding:'utf8'}).stdout.trim(), 'old');
});
test("an active install directory needs no PATH setup", t => {
  const f = fixture(t);
  f.env.PATH = `${join(f.home, '.local/bin')}:${f.env.PATH}`;
  const result = f.run('--version', '1.2.3');
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /Verify installation:\n  openapi-sdkgen --version/);
  assert.equal(result.stdout.includes('export PATH='), false);
});
test("an unwritable destination reports a directory option and no success", t => {
  const f = fixture(t);
  const blocked = join(f.root, 'blocked'); writeFileSync(blocked, 'retain');
  const result = f.run('--version', '1.2.3', '--install-dir', blocked);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /writable directory with --install-dir/);
  assert.equal(result.stdout.includes('Installed openapi-sdkgen'), false);
  assert.equal(readFileSync(blocked, 'utf8'), 'retain');
});
