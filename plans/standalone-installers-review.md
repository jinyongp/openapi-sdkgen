# Standalone installer delivery review

## Contract

- Provide shell installers for macOS/Linux and a PowerShell installer for Windows,
  covering the existing amd64/arm64 native archives.
- Make installers the primary path in the product home, README and both SDKs'
  English/Korean getting-started guides. Keep direct downloads, npm and Homebrew.
- Support a released version and install-directory override. Defaults need no
  administrator access. Do not change persistent PATH or execution policy.
- Download exact-version GitHub assets, verify SHA-256 before extraction, and
  stage the executable beside its destination before replacing it. Download,
  checksum and extraction failures must preserve an existing executable.
- Stamp the release version into both scripts, publish them with checksums,
  and merge only that exact release's scripts into the matching documentation.
- Commit each unit; review each unit and the combined result. Do not publish.

## Units and review results

| Area | Result | Evidence and corrections |
| --- | --- | --- |
| Installer arguments and asset selection | Reviewed | POSIX fixtures cover Linux/macOS and amd64/arm64; PowerShell covers amd64/arm64 and native architecture from an emulated process. Exact URLs asserted by download fixtures. Invalid arguments and unsupported architectures fail. |
| Integrity and replacement | Reviewed and rechecked | Download failure, changed archive hash, missing/duplicate checksums and invalid archives preserve existing files. Reinstallation leaves no staged files. PowerShell replacement uses a backup beside the destination, including custom-volume installs; a null backup argument failed portable .NET verification and was corrected. |
| Release build and retries | Reviewed | Scripts are stamped after GoReleaser, included in checksums and immutable assets. Retries download and verify those existing scripts rather than regenerating them. Version smoke checks compare the complete output. |
| Release and CI gates | Reviewed | Ordinary CI includes required Linux/macOS fixtures and Windows PowerShell 5.1/7 fixtures. Published installers execute on Linux/macOS/Windows before stable documentation deployment. Linux also runs the generated Go consumer. Workflow lint and existing release tests passed. |
| Site composition | Reviewed and exercised | Pinned Releaseway `distribute` uses exact tag/commit and the existing site baseline. Executed its actual v0.5.0 bundle locally and compared every built documentation file and both stamped scripts byte-for-byte. The baseline is preserved. |
| Documentation and locale parity | Reviewed | Same commands in English/Korean; independent executable installs are first. PATH setup, pinned version/directory, upgrade by downloading a new installer and uninstall by deleting the executable are documented. Windows command permits this process's script without persistent policy changes. |
| Browser entry routes and layout | Reviewed | Product home in both locales and all four SDK entry routes show installer instructions. Existing getting-started redirects remain. Desktop and 390px mobile checks found no page-wide overflow; long shell commands scroll inside code blocks. |

## Local validation

- `devtools run test:install`: 11 POSIX fixture cases passed.
- PowerShell fixture suite passed using a disposable official PowerShell 7.6.7
  Linux runtime, verified against its published SHA-256. Windows environment and
  download responses were fixtures; this is not execution on Windows.
- Built the actual CLI with an explicit fixture version, archived and installed
  it with the real shell installer, verified exact version output, then ran
  `scripts/release/smoke-go.sh`: SDK generation and local HTTP calls passed.
  Only the release-download boundary was replaced with local archive fixtures.
- `devtools run build:install 1.2.3 OUTPUT` and shell syntax check passed.
- `devtools run test:release`: dispatch and CI-reuse tests passed.
- Actionlint passed for all four affected workflows.
- `devtools run validate:docs`, `devtools run build:docs`, documentation type
  checks and built-route/historical-anchor checks passed.
- Real Releaseway distribution bundle preserved the built site and script bytes.
- Scoped diffs and combined callers were reviewed; no unresolved in-scope defect.

## Execution limits

No release or Pages deployment was performed, and no user installation was changed.
Native Windows PowerShell 5.1 and macOS runs remain for the added required GitHub CI
jobs. Actual public-release downloads and site delivery are verified by the added
release gates at publication. The public installer URLs become available when the
next stable release publishes its matching documentation. Existing downloaded
installers retain their embedded version; fetching a new script selects the newly
published stable release. Archive checksums provide integrity over HTTPS, not an
additional signing mechanism.
