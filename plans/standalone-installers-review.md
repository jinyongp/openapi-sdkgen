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
- Installation must identify the release, platform and destination before
  downloading, show the active stage, distinguish failure from success, and print
  copyable verification/PATH commands. Existing commands taking precedence must
  not silently leave the user invoking a different installation.

## Units and review results

| Area | Result | Evidence and corrections |
| --- | --- | --- |
| Installer arguments and asset selection | Reviewed | POSIX fixtures cover Linux/macOS and amd64/arm64; PowerShell covers amd64/arm64 and native architecture from an emulated process. Exact URLs asserted by download fixtures. Invalid arguments and unsupported architectures fail. |
| Integrity and replacement | Reviewed and rechecked | Download failure, changed archive hash, missing/duplicate checksums and invalid archives preserve existing files. Reinstallation leaves no staged files. PowerShell replacement uses a backup beside the destination, including custom-volume installs; a null backup argument failed portable .NET verification and was corrected. |
| Release build and retries | Reviewed | Scripts are stamped after GoReleaser, included in checksums and immutable assets. Retries download and verify those existing scripts rather than regenerating them. Version smoke checks compare the complete output. |
| Release and CI gates | Reviewed | Ordinary CI includes required Linux/macOS fixtures and Windows PowerShell 5.1/7 fixtures. Published installers execute on Linux/macOS/Windows before stable documentation deployment. Linux also runs the generated Go consumer. Workflow lint and existing release tests passed. |
| Site composition | Reviewed and exercised | Pinned Releaseway `distribute` uses exact tag/commit and the existing site baseline. Executed its actual v0.5.1 bundle locally and compared every built documentation file and both stamped scripts byte-for-byte. The baseline is preserved. |
| Documentation and locale parity | Reviewed and simplified | Same commands in English/Korean; default installation is one line per OS (`curl ... \| sh`, `irm ... \| iex`). PATH guidance remains visible; version, directory and alternative methods are collapsed. Saved-file commands remain in the optional section. No persistent execution-policy changes are needed. |
| Browser entry routes and layout | Reviewed | Product home in both locales and all four SDK entry routes show installer instructions. Existing getting-started redirects remain. Desktop and 390px mobile checks found no page-wide overflow; long shell commands scroll inside code blocks. |
| Releaseway refresh | Reviewed | Official stable tags and commits verified: actions v0.5.1 / 7427110b565bedef79ab0f80e6821a706d7f21ed; homebrew-actions v0.4.1 / d3f1522212aca4702a32805159b0bd3e88561986. npm-actions v0.6.0 remains latest. All six changed references include docs distribution. Upstream changes preserve used inputs/outputs and refresh internal action dependencies. |
| Installation guidance | Reviewed and exercised | Version/platform/destination precede downloads, verification, extraction and installation. Success prints exact verification commands. PATH instructions are executable, safely quoted and prepend the new directory; tests execute emitted shell commands and parse/execute PowerShell PATH guidance for spaces, apostrophes and literal dollars. Existing command precedence is reported; an already active installation requires no PATH change. |
| PowerShell host state | Fixed and rechecked | `irm ... \| iex` previously left the caller's error preference changed. Installer preferences now live in a child scope; success and early-failure fixtures verify error preference preservation. Progress preference and TLS configuration also remain unchanged. |
| Served installation URLs | Reviewed and covered | Deployment records both hashes from the exact merged artifact, then verifies both served scripts. Matching, stale, missing and eventually updated responses have fixtures. Six bounded retries allow propagation; a failed check leaves the new site live and reports how to retry. |

## Local validation

- `devtools run test:install`: 14 POSIX installer and four served-site fixture
  cases passed. New checks cover stage order, copyable commands, PATH precedence,
  literal path characters, no false success on failure and writable-directory hints.
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
- Follow-up installation UX checks: stamped shell script passed on stdin, and
  the stamped PowerShell script passed via `Invoke-Expression`. Docs build,
  type/route checks and collapsed-section browser checks were repeated for the
  simplified instructions. The later guidance review updates both templates as
  described in the new review rows above.
- Current review rechecked download/integrity/extraction/replacement and their
  unchanged callers; added actionable download and destination errors. PowerShell
  failures identify the stage and preserve native error detail.
- Latest Releaseway tags were checked again after the updates. Four workflow
  lint checks passed, including the new exact-source sparse checkout, hash
  outputs and post-deployment verification step.
- The current Korean product home still shows exactly two single-line installation
  commands and closed alternatives, with no page-wide overflow. Browser console
  had an unrelated `/favicon.ico` 404 from the local server; no installer/UI error.
- Scoped diffs and combined callers were reviewed; no unresolved in-scope defect.

## Execution limits

No release or Pages deployment was performed, and no user installation was changed.
Native Windows PowerShell 5.1 and macOS runs remain for the added required GitHub CI
jobs. Public-release downloads are checked by release jobs, and deployed script
hashes are checked by the new post-deployment gate; neither live operation was
executed in this review. Site verification failure does not roll back deployment.
The public installer URLs become available when the
next stable release publishes its matching documentation. Existing downloaded
installers retain their embedded version; fetching a new script selects the newly
published stable release. Archive checksums provide integrity over HTTPS, not an
additional signing mechanism.
