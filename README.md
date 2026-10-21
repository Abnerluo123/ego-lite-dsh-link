# zero-ego

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**把 Ego Lite 接进 DeepSeek Harness，零授权接管浏览器。**

Agent 直接调用 `mcp__ego_lite__*` 工具开网页、点击、填写、读页面——不再逐次确认权限。

## 安装

把 Release 里的 zip 交给 AI，说一句：

> 解压后运行 `install.ps1 -ApplyProfile`（Windows）或 `./install.sh --apply`，装到 DSH 工作区之外，确认 `mcp__ego_lite__*` 可用。

手动步骤见 [AGENTS.md](AGENTS.md)。

## 能做什么

- 50 个工具，覆盖 Ego 官方 skill 的全部 Page / TaskSpace API
- 可访问性树 + 稳定 refs，比截图省 token；`brief` 模式只回 `page/url/title`
- 网络抓包、页面状态读取、上传下载、多标签自动跟随
- `ego_lite_run` 是逃生舱：`page.evaluate()`、`page.cdp()` 等都能直接写

## 两个构建

同一份代码，两个构建标签，**工具完全一样（50 个）**，区别只在默认输出：

| | 通用版 | Ego 原生版（`-tags egoonly`） |
| --- | --- | --- |
| 工具 | 50 | **50**（同一个工具集） |
| 动作默认返回 | 可访问性树 | `page/url/title` |
| 面向 | 通用浏览器自动化，后续接 Edge/CDP | **只面向 Ego Lite 与官方 skill** |
| 二进制 | 3.3 MB | 3.0 MB |

Ego 原生版不是阉割版：上传、下载、弹窗、网络抓包、页面状态、多标签、`run` 全都在，只是默认不吐整棵树，更贴近官方 skill 的“先看再动”，也更省 token。需要元素 refs 时调 `ego_lite_snapshot` 或 `ego_lite_output(brief:"false")`。

## 实测笔记（30 个站点）

- 30 个常见站点全部可加载（Bing / GitHub / Wikipedia / YouTube / B 站 / 淘宝 / BBC / NYT …），SPA 站点 `readyState` 常停在 `interactive`，属正常。
- 上传空文件、滚动、拖拽、下载（GitHub Release 资产）、弹窗、多标签、iframe 均验证通过。
- **防粘贴**：站点 `preventDefault` 掉 paste 时，用 `keyboard.insertText` 可绕过（已实测 blocked=true、值仍写入）。
- **弹窗与文件选择器**：`page.waitForEvent` 只支持 popup/download，但 dialog 有 CDP 事件（`Page.javascriptDialogOpening`）、文件选择器有官方 `page.waitForFileChooser()`。已封装为 `ego_lite_dialog(trigger=...)` 与 `ego_lite_filechooser(trigger=...)`：点按钮、捕获事件、自动处理，返回 `handled/type/message` 或写入的文件。
- 验证发现并修复：`fetch` 曾在 Node 上下文读 `document`、`wait_event` 的 popup adopt 用错 handle、`state` 未走 `page.evaluate`。

## 构建

```powershell
.\build.ps1 -GoExe <go>   # 同时产出完整版与 ego-dsh-lite-*
```

MIT
