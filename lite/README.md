# zero-ego (lite)

Ego Lite-only build of [zero-ego](../README.md): **16 tools**, actions return only
`page/url/title`, no accessibility tree by default.

| | full (main) | this build |
| --- | --- | --- |
| tools | 49 | 16 |
| action result | accessibility tree | page/url/title |
| binary | 3.28 MB | 3.02 MB |

## Tools

`open` · `snapshot` (`full:"true"` for the tree) · `click` · `fill` · `press` · `eval` · `wait` · `scroll` · `upload` · `dialog` · `download` · `tabs` · `switch` · `screenshot` · `run` · `finish`

`run` keeps the full Ego API available (`page`, `task` in scope).

## Install

```powershell
.\install.ps1 -ApplyProfile      # Windows
./install.sh --apply             # macOS / Linux
```

The installer detects the lite binary and writes `zero-ego-lite-<version>.exe`
outside the DSH workspace. Reload the profile and the `mcp__ego_lite__*` tools
appear.

## Build

```powershell
go build -trimpath -ldflags "-s -w" -o zero-ego-lite.exe .
```

MIT
