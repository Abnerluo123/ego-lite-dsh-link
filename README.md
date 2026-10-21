# zero-ego

把 Ego Lite 接进 DeepSeek Harness；动作简短返回，页面树按需读取。

[下载最新版](https://github.com/Abnerluo123/zero-ego/releases/latest) · [Agent 快速指南](AGENTS.md) · [详细参考](docs/REFERENCE.md)

## 安装：解压发布包后运行

- Windows：`.\install.ps1 -ApplyProfile`
- macOS / Linux：`bash ./install.sh --apply`

安装器备份并更新现有 MCP 配置，不杀其他会话。

| 包 | 定位 |
|---|---|
| `zero-ego-lite-<版本>.zip` | Ego 原生版，配合官方 skill，功能完整 |
| `zero-ego-<版本>.zip` | 标准发行包，当前使用同一 Ego 后端 |

**lite 不是精简版；两包都需要 Ego Lite，不兼容其他浏览器。**

## 完成任务，也要退出接管

`start → snapshot → 操作与验证 → finish → 等待回执 → 回复用户`

- 成功完成后，必须调用 `ego_lite_finish`；默认关闭 Agent 标签页。
- 仅当结果需要留给用户时，用 `keep:"all"` 或具体标签（如 `keep:"p2"`）；保留页面仍结束任务。
- 确认 `agentControlReleased:true` 才能说已退出接管；false/null 用 `control_status(spaceId)` 检查并如实报告。
- 暂停、报错或请用户操作时用 `handoff`，不要 `finish`；不要清理其他 Agent 的空间。
- MCP 无法知道模型何时结束回复，关标签或停止调用不会自动完成任务。

按需用 `snapshot` 获取 refs，截图用于视觉验证。`run` 使用 ESM；Node/CDP 拥有宿主权限，不是文件沙箱。

[工具速查](docs/AGENT_GUIDE.md) · [能力与权限边界](docs/REFERENCE.md) · [反馈记录](docs/FEEDBACK_STATUS.md) · MIT
