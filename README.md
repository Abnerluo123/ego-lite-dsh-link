# zero-ego

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**把 Ego Lite 接进 DeepSeek Harness，零授权接管浏览器。**

Agent 直接调用 `mcp__ego_lite__*` 工具开网页、点击、填写、读页面——不再逐次确认权限。

## 安装

把 Release 里的 zip 交给 AI，说一句：

> 解压后运行 `install.ps1 -ApplyProfile`（Windows）或 `./install.sh --apply`，装到 DSH 工作区之外，确认 `mcp__ego_lite__*` 可用。

手动步骤见 [AGENTS.md](AGENTS.md)。

## 能做什么

- 49 个工具，覆盖 Ego 官方 skill 的全部 Page / TaskSpace API
- 可访问性树 + 稳定 refs，比截图省 token；`brief` 模式只回 `page/url/title`
- 网络抓包、页面状态读取、上传下载、多标签自动跟随
- `ego_lite_run` 是逃生舱：`page.evaluate()`、`page.cdp()` 等都能直接写

## 两个版本

| | main | lite 分支 |
| --- | --- | --- |
| 工具 | 49 | 12 |
| 动作返回 | 可访问性树 | 仅 page/url/title |
| 二进制 | 3.28 MB | 3.02 MB |

## 构建

```powershell
.\build.ps1 -GoExe <go>   # 同时产出完整版与 ego-dsh-lite-*
```

MIT
