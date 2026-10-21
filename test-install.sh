#!/usr/bin/env bash
# Isolated installer tests; argument is an existing temporary fixture directory.
set -euo pipefail
fixture="$(cd "$1" && pwd)"
installer="$fixture/install.sh"
default_version=$(sed -n 's/^version="\([^"]*\)".*/\1/p' "$installer")
[ -n "$default_version" ] || { echo "cannot read installer version" >&2; exit 1; }
test_root="$fixture/unix-test"
mkdir -p "$test_root/profile" "$test_root/home"
export HOME="$test_root/home"
export DSH_PROFILE_DIR="$test_root/profile"
export PATH="$(dirname "$BASH"):$PATH"
# Simulate supported OS/native CPU without relying on the test runner's host.
uname() { case "$1" in -s) printf '%s\n' "${TEST_OS:-Linux}" ;; -m) printf '%s\n' aarch64 ;; *) return 1 ;; esac; }
export -f uname
# Minimal Git distributions on Windows omit chmod; executable-bit semantics there are synthetic.
if ! command -v chmod >/dev/null; then chmod() { :; }; export -f chmod; fi
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
contains() { grep -Fq -- "$2" "$1" || fail "$3"; }
cli="$test_root/ego O'Brien.exe"
printf '#!/bin/sh\nexit 0\n' > "$cli"
chmod 755 "$cli"
patch="$DSH_PROFILE_DIR/cordis.patch.yml"
install_dir="$test_root/install O'Brien"
printf '# mcp-ego-lite only in a comment\n- insert: []\n' > "$patch"
cp "$patch" "$test_root/original"
"$BASH" "$installer" --ego "$cli" --install-dir "$install_dir" --apply > "$test_root/install.log" 2>&1
cmp -s "$install_dir/zero-ego-$default_version" "$fixture/dist/zero-ego-linux-arm64" || fail 'native ARM64 selection'
contains "$patch" "O''Brien" 'YAML apostrophe escaping'
contains "$patch" '- id: mcp-ego-lite' 'comment-only marker must not block insertion'
backups=("$patch".bak.*)
[ "${#backups[@]}" -eq 1 ] || fail 'one backup on initial apply'
cmp -s "$test_root/original" "${backups[0]}" || fail 'backup preserves original'
cp "$patch" "$test_root/installed-profile"
"$BASH" "$installer" --ego "$cli" --install-dir "$install_dir" --apply >> "$test_root/install.log" 2>&1
cmp -s "$patch" "$test_root/installed-profile" || fail 'idempotent reinstall'
backups=("$patch".bak.*)
[ "${#backups[@]}" -eq 1 ] || fail 'idempotent reinstall creates no backup'
printf '%s\n' '- insert:' '    - id: other' '      config:' '        command: keep-other' "    - id: 'mcp-ego-lite' # target" '      config:' '        command: old-command' '        cwd: old-cwd' '        env:' '          EGO_BROWSER_CLI: old-cli' '          KEEP_ME: retained' '        customSetting: retained' '    - id: other-after' '      config:' '        command: keep-after' > "$patch"
"$BASH" "$installer" --ego "$cli" --install-dir "$install_dir" --version 9.0.1 --arch amd64 --apply >> "$test_root/install.log" 2>&1
cmp -s "$install_dir/zero-ego-9.0.1" "$fixture/dist/zero-ego-linux-amd64" || fail 'explicit AMD64 selection'
contains "$patch" 'zero-ego-9.0.1' 'existing profile command upgrade'
contains "$patch" 'KEEP_ME: retained' 'preserve extra env'
contains "$patch" 'customSetting: retained' 'preserve extra config'
contains "$patch" 'command: keep-other' 'preserve preceding entry'
contains "$patch" 'command: keep-after' 'preserve following entry'
if grep -Eq 'old-command|old-cwd|old-cli' "$patch"; then fail 'all target fields must update'; fi
TEST_OS=Darwin "$BASH" "$installer" --ego "$cli" --install-dir "$install_dir" --version 9.0.2 --arch arm64 --apply >> "$test_root/install.log" 2>&1
cmp -s "$install_dir/zero-ego-9.0.2" "$fixture/dist/zero-ego-darwin-arm64" || fail 'Darwin ARM64 selection'
"$BASH" "$installer" --ego "$cli" --install-dir "$install_dir" --binary "$fixture/dist/zero-ego-lite-linux-arm64" --apply >> "$test_root/install.log" 2>&1
contains "$patch" "zero-ego-lite-$default_version" 'full-to-lite switch'
printf '%s\n' '- insert:' '    - id: mcp-ego-lite' '      config:' '        serverName: ego_lite' > "$patch"
"$BASH" "$installer" --ego "$cli" --install-dir "$install_dir" --apply >> "$test_root/install.log" 2>&1
contains "$patch" '        command:' 'insert missing command'
contains "$patch" '        cwd:' 'insert missing cwd'
contains "$patch" '        env:' 'insert missing env'
contains "$patch" '          EGO_BROWSER_CLI:' 'insert missing CLI'
printf '%s\n' '- insert:' '    - id: mcp-ego-lite' '      config:' '        command: one' '    - id: mcp-ego-lite' '      config:' '        command: two' > "$patch"
cp "$patch" "$test_root/ambiguous"
if "$BASH" "$installer" --ego "$cli" --install-dir "$install_dir" --apply >> "$test_root/install.log" 2>&1; then fail 'duplicate entries must be rejected'; fi
cmp -s "$patch" "$test_root/ambiguous" || fail 'ambiguous profile unchanged'
if "$BASH" "$installer" --ego "$cli" --install-dir "$install_dir" --version ../escape >> "$test_root/install.log" 2>&1; then fail 'invalid version rejected'; fi
printf 'PASS: Unix installer architecture, backups, upgrades, profile preservation and rejection cases\n'
