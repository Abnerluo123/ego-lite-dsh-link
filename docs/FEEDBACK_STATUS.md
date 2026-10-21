# Feedback status

## 1.7.1: task-space tools work on the current Ego CLI

- On ego lite 0.6.0.16 every task-space call (`start`, `spaces`, `claim_space`, `control_status`, `finish`, `run`) failed with `ReferenceError: await is not defined`. The generated wrapper opened with `const value=await(async()=>{…})();`, and ego-browser evaluates a stdin script as a plain script unless a top-level await is followed by an identifier, so `await` was bound as an identifier.
- Generated scripts now await a named async function (`const valueBody=async()=>{…};const value=await valueBody();`) and no longer depend on that heuristic. The shared helper is `bodyCall`.
- Regression test `TestGeneratedScriptsAvoidTopLevelAwaitParen` rejects any generated `=await(`.
- Cause was introduced in 1.6.0 by `runStandalone`; 1.4.0 was unaffected because its scripts opened with `await taskSpace(...)`.
- Verified live through the DSH MCP client: start, navigate, snapshot, click, fill, press, wait_url, evaluate, run, screenshot, new_page, tabs, close_page, switch, box, frames, page_info, do, control_status and finish. `finish` returned `agentControlReleased:true` with `verification:"space-registry"`.

## 1.7.0: task completion and takeover release

- Finish now verifies the addressed space in the registry and returns `agentControlReleased:true/false/null`; a dispatched finish is never automatically replayed.
- `keep:"all"` retains result tabs while ending agent ownership; unknown-origin user pages are preserved. Explicit `spaceId` recovery never claims user spaces or bulk-finishes other agents.
- `control_status` works without creating a task and reports other active agent spaces without touching them.
- Optional idle handoff is disabled by default, never closes pages or interrupts an active tool. A persistent MCP server cannot observe the model's final answer.
- Live exact generated SDK script: space 3 changed from `agent` to `user`, kept `p1`, then independent registry check returned `agentControlReleased:true` and `agentSpaces:[]`. This proves release for that space; it is not a global UI guarantee for other sessions.
- Regression cases cover malformed receipts, finish failure, unknown verification, repeated finish, other-space isolation, keep parsing, protected pages, idle opt-in and active-call exclusion.

## 1.6.0: real-task hardening

This checklist separates connector changes, verified SDK behavior, and upstream limits. Previous statements that all APIs/sites were fully verified were too broad: a homepage loading is not proof that every workflow works.

| Report | Resolution / boundary |
|---|---|
| CLI IPC blocked in sandbox while MCP works | Agent guide now prefers the configured MCP, identifies bootstrap/confinement errors and prohibits routing around explicit sandbox denial. Live standalone binary verification was blocked on this host; generated wrapper scripts were tested via the working MCP SDK path. |
| Current page points to a closed/missing label | Every implicit page wrapper resolves against actual managed tabs; explicit labels are not rewritten. List/close/release reconcile and report current page. Unit tests cover fallback; SDK helper exercised on missing-label fixture. |
| Space owned by user and no recovery tool | List works without start; numeric resume/claim identity; validated claim responses; start refreshes actual ownership. No auto-claim. Mock tests cover malformed claim/state preservation, non-p1 pages and restart identity. |
| Connector EOF/reload closes task | Removed implicit finish on EOF. Completion is explicit; handoff/user control is not completion. |
| Evaluate Promise/arrow expression failures | Evaluates once and invokes only actual function values. Node and live SDK tests cover Promise, callback expressions, arrow-containing strings, async functions and IIFEs. |
| require with top-level await | ESM usage documented; use await import. No unreliable source rewrite. |
| Only HTTP(S), local PDF awkward | Added explicit absolute-path open_local for PDFs/images/text/HTML; normal navigate stays HTTP(S). Local-file URL escaping handled by URL encoder. PDF text/OCR stays with document tooling. |
| Minimized window screenshot | Restore addressed page window, retry; real minimized-window test returned restoredWindow:true and a valid PNG. |
| PDF #page=N ignored | Documented toolbar page-number input + Enter workaround. No fragile viewer-version-specific global patch. |
| Errors too long | Bounded CLI buffers; concise cause + safety metadata, EGO_DSH_DEBUG expanded diagnostics. Unicode safe. |
| Every action dumps a tree | Both builds brief by default; explicit snapshot always returns a tree (this was a regression and is fixed). Supports viewport/full_page/subtree and per-call bound. |
| Base64 data URL output explosion | Marked previews; oversized global results remain valid JSON and point to complete artifact. Use screenshot/fetch(saveAs), not base64 round trips. |
| Screenshot crop/scale/saveAs missing | Added selector, clip, numeric CSS-relative scale, fullPage, saveAs. Live crop 200×100 at scale .5 produced 100×50; selector/full-page captures produced PNGs. |
| web_fetch misses rendered article | Browser body.innerText workflow documented. |
| Raw run file permissions unclear | Explicit trust-boundary documentation; Node/CDP use host permissions. No fake JS-only filesystem sandbox. True isolation requires host/OS support; per-tool checks cannot confine arbitrary Node. |
| Localhost access | Kept for authorized local QA/PDF tasks. Untrusted pages do not authorize probing local services. Blanket blocking would break real workflows and would not sandbox Node/CDP. |
| Login/session assumptions | Dedicated browser config only; do not silently select profiles or claim main-browser cookies. |
| Recommended scripts outside workspace | Corrected recommendation to workspace/ego/<goal-date>. |
| Upload selector mismatch / silent success | Normal operations use Ego canonical resolution. Removed invalid page-wide file counting and arbitrary last-input fallback. Native receipt retained; does not claim server acceptance. Existing shadow inputs and dynamic chooser verified live. |
| Click “success” but no state change | Receipt preserved; DOM hash includes values/checkbox/expanded state. noVisibleChange is explicitly heuristic, not success/failure. Best-effort observation errors cannot undo dispatched actions. Dialog receipts return without blocked page inspection. |
| Frame read/action mismatch | frames + main-world frame evaluate + shared CSS/text/href/XPath filters added. Main-world app globals tested. Out-of-process inaccessible contexts fail explicitly; native refs or known frame URL remain fallback. |
| nth/visible ambiguity | Official native terminal nth preserved; optional filters reject ambiguity. Ref/role filters are rejected rather than approximating accessible names. |
| Coordinate scroll mismatch | box scrolls then measures; frame rectangles explicitly local coordinates. Native click already scrolls. No ungrounded auto-scroll for raw x/y clicks. |
| Start blank page, user tab invisible | Start/resume reports actual task-space tabs. Auto-adopting all user tabs is deliberately not done; adopt is explicit. |
| PDF.js noisy tree | Scope/line-safe bounds reduce cost without deleting legitimate content or inventing refs. Viewer text internals remain upstream. |
| wait_function full tree/hard limit | Returns matched:true; timeout extended to 300s, new profile client timeout 360s. Existing custom timeout preserved. |
| Repetitive six-step workflows | Bounded do(steps) macro stops at failure and reports completed work. No automatic write replay, no transaction promise. |
| Fetch auto-CSRF leaks/corrupts bodies (audit) | Removed all automatic site-specific injection; transparent documented page.fetch, relative URLs, binary saveAs. JSON/body preservation tested. |
| Wrong keyboard.insert API (audit) | Maps insert action to insertText; Unicode anti-paste fixture verified. |
| Packaging/upgrade stale binary (audit) | Both tags and all root source/helper files packaged; Python historical reference excluded. Updaters back up and modify exact MCP block, preserve unrelated values, no broad process kill. Both-tag clean extraction and Windows/Unix temp profile upgrade tests. |

## Verification scope

- Go unit/regression tests for both build tags.
- JavaScript semantics tests executed in Node.
- Windows PowerShell 5.1 and Unix-shell installer fixtures; no real user profile edits.
- Clean extraction tests/builds for both variants.
- Real SDK fixture: expressions, shadow/nth/ambiguity, click receipts/control state, Unicode insertText, frame app globals, uploads, dynamic chooser, explicit snapshot, screenshot geometry and minimized restoration, direct/trigger dialogs, local HTML with spaces/Unicode, missing-page-label recovery and byte-exact binary saving.
- Standalone end-to-end MCP process test is included but could not complete on this host because the confined CLI had no IPC output. This is recorded, not counted as a pass.
