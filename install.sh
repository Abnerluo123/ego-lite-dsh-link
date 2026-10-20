#!/usr/bin/env bash
# Installs zero-ego on macOS or Linux and prints the DSH profile entry.
#
#   ./install.sh                       # full build if present, else lite
#   ./install.sh --ego /path/to/ego-browser --apply
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ego_cli=""
apply=0

while [ $# -gt 0 ]; do
  case "$1" in
    --ego) ego_cli="$2"; shift 2 ;;
    --apply) apply=1; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

os="$(uname -s)"
arch="$(uname -m)"
case "$os/$arch" in
  Darwin/arm64) suffix="darwin-arm64" ;;
  Darwin/x86_64) suffix="darwin-amd64" ;;
  Linux/x86_64) suffix="linux-amd64" ;;
  Linux/aarch64|Linux/arm64) suffix="linux-arm64" ;;
  *) echo "unsupported platform: $os/$arch" >&2; exit 1 ;;
esac

src=""
for candidate in \
  "$here/zero-ego-$suffix" \
  "$here/dist/zero-ego-$suffix" \
  "$here/zero-ego-lite-$suffix" \
  "$here/dist/zero-ego-lite-$suffix"; do
  if [ -f "$candidate" ]; then src="$candidate"; break; fi
done
[ -n "$src" ] || { echo "no zero-ego binary for $suffix in $here" >&2; exit 1; }

case "$(basename "$src")" in
  *lite*) name="zero-ego-lite" ;;
  *) name="zero-ego" ;;
esac

dest_dir="$HOME/.local/share/zero-ego"
mkdir -p "$dest_dir"
dest="$dest_dir/$name"
pkill -f "$dest" 2>/dev/null || true
cp -f "$src" "$dest"
chmod 755 "$dest"

if [ -z "$ego_cli" ]; then
  for candidate in "$HOME/.local/share/ego/active_version_dir/ego-browser" \
                   "/Applications/ego lite.app/Contents/MacOS/ego-browser" \
                   "$(command -v ego-browser 2>/dev/null || true)"; do
    if [ -n "$candidate" ] && [ -x "$candidate" ]; then ego_cli="$candidate"; break; fi
  done
fi
[ -n "$ego_cli" ] || { echo "ego-browser not found; pass --ego <path>" >&2; exit 1; }

echo "installed: $dest"
echo "ego cli  : $ego_cli"

block=$(cat <<YAML
- insert:
    - id: mcp-ego-lite
      name: "@deepseek-ai/dsh-mcp-client"
      config:
        serverName: ego_lite
        transport: stdio
        command: "$dest"
        cwd: "$dest_dir"
        env:
          EGO_BROWSER_CLI: "$ego_cli"
        toolCallTimeoutMs: 180000
        failOnStartupError: false
YAML
)

profile_dir="${DSH_PROFILE_DIR:-$HOME/.dsh/profiles/desktop}"
patch="$profile_dir/cordis.patch.yml"

if [ "$apply" -eq 1 ]; then
  [ -f "$patch" ] || { echo "profile patch not found: $patch" >&2; exit 1; }
  if grep -q "mcp-ego-lite" "$patch"; then
    echo "profile already contains mcp-ego-lite; left unchanged"
  else
    printf '\n%s\n' "$block" >> "$patch"
    echo "appended mcp-ego-lite to $patch"
  fi
else
  echo
  echo "Add this to $patch"
  echo
  echo "$block"
fi
