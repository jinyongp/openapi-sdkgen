#!/bin/sh
set -eu

# Stamped when release assets are prepared. No runtime is needed to install the CLI.
version='@VERSION@'
install_dir="${HOME:?HOME is required}/.local/bin"
fail() { printf 'openapi-sdkgen: %s\n' "$*" >&2; exit 1; }
shell_quote() { printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"; }
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version|--install-dir)
      [ "$#" -ge 2 ] && [ -n "$2" ] || fail "$1 requires a value"
      case "$1" in --version) version="${2#v}" ;; --install-dir) install_dir="$2" ;; esac
      shift 2 ;;
    --help|-h)
      printf '%s\n' 'Usage: sh install.sh [--version VERSION] [--install-dir DIRECTORY]' 'Default directory: $HOME/.local/bin. Existing installations are replaced after verification.'
      exit 0 ;;
    *) fail "Unknown option: $1" ;;
  esac
done
printf '%s\n' "$version" | LC_ALL=C grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$' || fail 'Invalid version; use a released installer or --version X.Y.Z'
case "$(uname -s)" in Darwin) os=darwin ;; Linux) os=linux ;; *) fail 'Supported systems: macOS and Linux. Use install.ps1 on Windows.' ;; esac
case "$(uname -m)" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) fail 'Supported CPU architectures: amd64 and arm64' ;; esac
for tool in curl tar mktemp; do command -v "$tool" >/dev/null 2>&1 || fail "$tool is required"; done
if command -v sha256sum >/dev/null 2>&1; then hash_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then hash_tool=shasum
else fail 'sha256sum or shasum is required'; fi
case "$install_dir" in /*) ;; *) install_dir="$PWD/$install_dir" ;; esac
destination="$install_dir/openapi-sdkgen"
asset="openapi-sdkgen_${version}_${os}_${arch}.tar.gz"
base="https://github.com/jinyongp/openapi-sdkgen/releases/download/v${version}"
printf 'Installing openapi-sdkgen %s (%s/%s)\nDestination: %s\n' "$version" "$os" "$arch" "$destination"
work="$(mktemp -d)"
staged=''
cleanup() { rm -rf "$work"; if [ -n "$staged" ]; then rm -f "$staged"; fi; }
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
download() { curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 300 --output "$2" "$1"; }
printf 'Downloading release checksums...\n'
download "$base/checksums.txt" "$work/checksums.txt" || fail "Could not download checksums for v$version. Check your connection and the release version."
printf 'Downloading %s...\n' "$asset"
download "$base/$asset" "$work/$asset" || fail "Could not download $asset. Check your connection and the release version."
printf 'Verifying SHA-256...\n'
expected="$(awk -v name="$asset" '$2 == name { print $1; count++ } END { if (count != 1) exit 1 }' "$work/checksums.txt")" || fail 'Expected exactly one checksum for the release archive'
printf '%s\n' "$expected" | LC_ALL=C grep -Eq '^[0-9a-fA-F]{64}$' || fail 'Invalid SHA-256 checksum'
if [ "$hash_tool" = sha256sum ]; then actual="$(sha256sum "$work/$asset")"
else actual="$(shasum -a 256 "$work/$asset")"; fi
actual="${actual%% *}"
[ "$(printf '%s' "$actual" | tr A-F a-f)" = "$(printf '%s' "$expected" | tr A-F a-f)" ] || fail 'Checksum mismatch; installation was not changed'
printf 'Extracting executable...\n'
tar -xzf "$work/$asset" -C "$work" openapi-sdkgen || fail 'Could not extract executable; installation was not changed'
[ -f "$work/openapi-sdkgen" ] && [ ! -L "$work/openapi-sdkgen" ] || fail 'Archive does not contain a regular executable'
printf 'Installing executable...\n'
mkdir -p "$install_dir" || fail "Cannot create $install_dir. Choose a writable directory with --install-dir."
[ ! -L "$destination" ] || fail "Refusing to replace a symbolic link: $destination"
[ ! -d "$destination" ] || fail "Destination is a directory: $destination"
staged="$(mktemp "$install_dir/.openapi-sdkgen.XXXXXX")" || fail "Cannot write to $install_dir. Choose a writable directory with --install-dir."
cp "$work/openapi-sdkgen" "$staged" || fail 'Could not copy executable; installation was not changed'
chmod 755 "$staged" || fail 'Could not set executable permissions; installation was not changed'
mv -f "$staged" "$destination" || fail "Could not replace $destination. Check directory permissions and retry."
staged=''
printf 'Installed openapi-sdkgen %s to %s\n' "$version" "$destination"
resolved="$(command -v openapi-sdkgen 2>/dev/null || true)"
if [ "$resolved" = "$destination" ]; then
  printf 'Verify installation:\n  openapi-sdkgen --version\n'
else
  printf 'Run now:\n  %s --version\n' "$(shell_quote "$destination")"
  if [ -n "$resolved" ]; then printf 'Another openapi-sdkgen currently takes precedence: %s\n' "$resolved"; fi
  printf 'Put this directory first in PATH for the current shell:\n  export PATH=%s:"$PATH"\n' "$(shell_quote "$install_dir")"
  printf 'For future terminals, add that export to your shell startup file.\n'
fi
