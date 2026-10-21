#!/usr/bin/env bash
# Installs zero-ego without stopping running connectors; --apply safely updates its profile entry.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ego_cli=""
apply=0
version="1.6.0"
architecture="auto"
dest_dir="${HOME}/.local/share/zero-ego"
src=""
while [ $# -gt 0 ]; do
  case "$1" in
    --ego|--version|--arch|--install-dir|--binary)
      [ $# -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }
      case "$1" in
        --ego) ego_cli="$2" ;; --version) version="$2" ;; --arch) architecture="$2" ;;
        --install-dir) dest_dir="$2" ;; --binary) src="$2" ;;
      esac
      shift 2 ;;
    --apply) apply=1; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
case "$version" in ''|*[!A-Za-z0-9._-]*|[._-]*) echo "invalid version: $version" >&2; exit 2 ;; esac
os="$(uname -s)"
[ "$architecture" != auto ] || architecture="$(uname -m)"
case "$architecture" in arm64|aarch64) architecture=arm64 ;; amd64|x86_64) architecture=amd64 ;; *) echo "unsupported architecture: $architecture" >&2; exit 2 ;; esac
case "$os" in Darwin) suffix="darwin-$architecture" ;; Linux) suffix="linux-$architecture" ;; *) echo "unsupported platform: $os" >&2; exit 1 ;; esac
if [ -z "$src" ]; then
  for candidate in "$here/zero-ego-$suffix" "$here/dist/zero-ego-$suffix" "$here/zero-ego-lite-$suffix" "$here/dist/zero-ego-lite-$suffix"; do
    if [ -f "$candidate" ]; then src="$candidate"; break; fi
  done
fi
[ -f "$src" ] || { echo "no zero-ego binary for $suffix" >&2; exit 1; }
if [ -z "$ego_cli" ]; then
  for candidate in "$HOME/.local/share/ego/active_version_dir/ego-browser" "/Applications/ego lite.app/Contents/MacOS/ego-browser" "$(command -v ego-browser 2>/dev/null || true)"; do
    if [ -n "$candidate" ] && [ -x "$candidate" ]; then ego_cli="$candidate"; break; fi
  done
fi
[ -f "$ego_cli" ] && [ -x "$ego_cli" ] || { echo "ego-browser not found or not executable; pass --ego <path>" >&2; exit 1; }
ego_cli="$(cd "$(dirname "$ego_cli")" && pwd)/$(basename "$ego_cli")"
for value in "$ego_cli" "$dest_dir"; do
  case "$value" in *$'\n'*|*$'\r'*) echo "paths must not contain newlines" >&2; exit 2 ;; esac
done
mkdir -p "$dest_dir"
dest_dir="$(cd "$dest_dir" && pwd)"
case "$(basename "$src")" in *lite*) name=zero-ego-lite ;; *) name=zero-ego ;; esac
dest="$dest_dir/$name-$version"
if [ ! -f "$dest" ] || ! cmp -s "$src" "$dest"; then
  # Copy then rename: a running image keeps its old inode, including same-version reinstalls.
  binary_tmp="$(mktemp "$dest_dir/.install.XXXXXX")"
  trap 'rm -f "$binary_tmp"' EXIT
  cp "$src" "$binary_tmp"
  chmod 755 "$binary_tmp"
  cmp -s "$src" "$binary_tmp" || { echo "copy verification failed" >&2; exit 1; }
  mv -f "$binary_tmp" "$dest"
  trap - EXIT
fi
echo "installed: $dest"
echo "ego cli  : $ego_cli"
quote_yaml() { local value="${1//\'/\'\'}"; printf "'%s'" "$value"; }
command_value="$(quote_yaml "$dest")"
cwd_value="$(quote_yaml "$dest_dir")"
cli_value="$(quote_yaml "$ego_cli")"
block="- insert:
    - id: mcp-ego-lite
      name: \"@deepseek-ai/dsh-mcp-client\"
      config:
        serverName: ego_lite
        transport: stdio
        command: $command_value
        cwd: $cwd_value
        env:
          EGO_BROWSER_CLI: $cli_value
        toolCallTimeoutMs: 360000
        failOnStartupError: false"
profile_dir="${DSH_PROFILE_DIR:-$HOME/.dsh/profiles/desktop}"
patch="$profile_dir/cordis.patch.yml"
if [ "$apply" -eq 0 ]; then printf '\nAdd this to %s\n\n%s\n' "$patch" "$block"; exit 0; fi
[ -f "$patch" ] || { echo "profile patch not found: $patch" >&2; exit 1; }
profile_tmp="$(mktemp "$patch.tmp.XXXXXX")"
trap 'rm -f "$profile_tmp"' EXIT
# Preserve permissions before filling the temporary profile. Only the exact id is updated.
cp -p "$patch" "$profile_tmp"
EGO_INSTALL_COMMAND="$command_value" EGO_INSTALL_CWD="$cwd_value" EGO_INSTALL_CLI="$cli_value" EGO_INSTALL_BLOCK="$block" awk '
function indent(s) { match(s, /^ */); return RLENGTH }
function meaningful(s) { return s !~ /^[ \t]*(#.*)?$/ }
function insert(at, text,    j) { for (j=n+1;j>at;j--) lines[j]=lines[j-1]; lines[at]=text; n++ }
function fail(message) { print message > "/dev/stderr"; exit 2 }
{
  sub(/\r$/, "")
  lines[++n]=$0
  if ($0 ~ /^[ ]*-[ \t]+id:[ \t]*[\042\047]?mcp-ego-lite[\042\047]?[ \t]*(#.*)?$/) { first=n; matches++ }
}
END {
  if (matches>1) fail("multiple mcp-ego-lite entries found; refusing an ambiguous profile update")
  if (!matches) { for(i=1;i<=n;i++) print lines[i]; if(n) print ""; print ENVIRON["EGO_INSTALL_BLOCK"]; exit }
  base=indent(lines[first]); last=n+1
  for(i=first+1;i<=n;i++) if(meaningful(lines[i]) && indent(lines[i])<=base) { last=i; break }
  config=0
  for(i=first+1;i<last;i++) if(indent(lines[i])==base+2 && lines[i] ~ /^[ ]*config:[ \t]*(#.*)?$/) { config=i; break }
  if(!config) fail("mcp-ego-lite must have a block config mapping")
  ci=base+2; fi=ci+2; configEnd=last
  for(i=config+1;i<last;i++) if(meaningful(lines[i]) && indent(lines[i])<=ci) { configEnd=i; break }
  command=0; cwd=0; env=0
  for(i=config+1;i<configEnd;i++) if(indent(lines[i])==fi) {
    if(lines[i] ~ /^[ ]*command:/) { if(command) fail("duplicate command in mcp-ego-lite config"); command=i }
    if(lines[i] ~ /^[ ]*cwd:/) { if(cwd) fail("duplicate cwd in mcp-ego-lite config"); cwd=i }
    if(lines[i] ~ /^[ ]*env:/) { if(env) fail("duplicate env in mcp-ego-lite config"); if(lines[i] !~ /^[ ]*env:[ \t]*(#.*)?$/) fail("mcp-ego-lite env must be a block mapping"); env=i }
  }
  if(command) lines[command]=sprintf("%*scommand: %s",fi,"",ENVIRON["EGO_INSTALL_COMMAND"])
  if(cwd) lines[cwd]=sprintf("%*scwd: %s",fi,"",ENVIRON["EGO_INSTALL_CWD"])
  # Append missing fields at the end of config so existing offsets remain stable.
  if(!command) { insert(configEnd,sprintf("%*scommand: %s",fi,"",ENVIRON["EGO_INSTALL_COMMAND"])); configEnd++ }
  if(!cwd) { insert(configEnd,sprintf("%*scwd: %s",fi,"",ENVIRON["EGO_INSTALL_CWD"])); configEnd++ }
  if(!env) { env=configEnd; insert(configEnd,sprintf("%*senv:",fi,"")); configEnd++ }
  envEnd=configEnd
  for(i=env+1;i<configEnd;i++) if(meaningful(lines[i]) && indent(lines[i])<=fi) { envEnd=i; break }
  cli=0
  for(i=env+1;i<envEnd;i++) if(indent(lines[i])==fi+2 && lines[i] ~ /^[ ]*EGO_BROWSER_CLI:/) { if(cli) fail("duplicate EGO_BROWSER_CLI in mcp-ego-lite env"); cli=i }
  cliLine=sprintf("%*sEGO_BROWSER_CLI: %s",fi+2,"",ENVIRON["EGO_INSTALL_CLI"])
  if(cli) lines[cli]=cliLine; else insert(env+1,cliLine)
  for(i=1;i<=n;i++) print lines[i]
}' "$patch" > "$profile_tmp"
if cmp -s "$patch" "$profile_tmp"; then echo "profile already up to date: $patch"; exit 0; fi
backup="$patch.bak.$(date +%Y%m%d%H%M%S).$$"
cp -p "$patch" "$backup"
mv -f "$profile_tmp" "$patch"
trap - EXIT
echo "updated profile: $patch (backup: $backup)"
