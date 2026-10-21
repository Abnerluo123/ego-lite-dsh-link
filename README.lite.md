# zero-ego · Ego 原生版

配合 Ego Lite 官方 skill 使用，**完整功能，不是精简版**；与标准包共享 Ego 后端，两包都需要 Ego Lite。

[下载](https://github.com/Abnerluo123/zero-ego/releases/latest) 后解压：Windows 用 `.\install.ps1 -ApplyProfile`；macOS/Linux 用 `bash ./install.sh --apply`。

`start → snapshot → 操作与验证 → finish → 等待释放回执 → 回复用户`

成功结束必须 `finish`，默认关闭 Agent 标签页。仅当结果需要保留时用 `keep:"all"` 或 `keep:"p2"`；仍须确认 `agentControlReleased:true`。false/null 用 `control_status(spaceId)` 检查，不宣称已释放。

用户操作、暂停或报错用 `handoff` 后停止；关标签、最终回复或断开 MCP 不会自动完成任务。不要清理其他 Agent 的空间。

[Agent 快速指南](AGENTS.md) · [工具速查](docs/AGENT_GUIDE.md) · [详细参考](docs/REFERENCE.md)
