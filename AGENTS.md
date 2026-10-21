# Installing this from another agent

This folder is a self-contained DSH connector for the **Ego Lite** browser. A user
can hand it to an agent and say one sentence:

> Install the Ego Lite browser bridge from this folder: run the install script for
> my OS with `-ApplyProfile` / `--apply`, make sure it lands outside the DSH
> workspace, then confirm the `mcp__ego_lite__*` tools are available.

## What the agent should do

1. **Pick the binary** for the current OS/arch from this folder (or `dist/`):
   `zero-ego-windows-amd64.exe`, `-windows-arm64.exe`, `-darwin-arm64`,
   `-darwin-amd64`, `-linux-amd64`, `-linux-arm64`.
2. **Install it outside the DSH workspace.** On Windows use
   `install.ps1 -ApplyProfile`; on macOS/Linux use `./install.sh --apply`.
   Never leave the binary inside the workspace: DSH's Windows sandbox blocks the
   Ego bootstrap IPC for executables there, and every call fails with
   `Failed to connect to ego_cli bootstrap`.
3. **Find `ego-browser`** (usually `~/.local/share/ego/active_version_dir/`) and
   pass it with `-EgoCli` / `--ego` if auto-detection fails.
4. **Verify**: after the profile reloads, `tools/list` should show 50
   `ego_lite_*` tools; `ego_lite_start` should return a `spaceId`.
5. **Record** the install path and how to remove it: delete the `mcp-ego-lite`
   `insert` item from the profile patch.

## Manual profile entry

```yaml
- insert:
    - id: mcp-ego-lite
      name: "@deepseek-ai/dsh-mcp-client"
      config:
        serverName: ego_lite
        transport: stdio
        command: "<install-dir>/zero-ego.exe"
        cwd: "<install-dir>"
        env:
          EGO_BROWSER_CLI: "<path to ego-browser>"
        toolCallTimeoutMs: 180000
        failOnStartupError: false
```

Top-level `- id:` entries are overrides and are skipped when the id is new; the
row must be inside `insert:`.

## Removing

Delete that item and let the profile reload. Nothing else is touched: no global
sandbox, approval, or permission settings are modified.
