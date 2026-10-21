# zero-ego

把 Ego Lite 接进 DeepSeek Harness。动作默认返回简短状态，需要元素 refs 时显式调用 `ego_lite_snapshot`。

## 安装

从 [Releases](https://github.com/Abnerluo123/zero-ego/releases) 选一个 ZIP，解压后：

```powershell
.\install.ps1 -ApplyProfile
```

macOS / Linux：`bash ./install.sh --apply`。升级会备份并更新同一个 MCP 配置项，不会杀掉其他会话。

| 下载 | 定位 |
| --- | --- |
| `zero-ego-<version>.zip` | 标准发行包 |
| `zero-ego-lite-<version>.zip` | Ego 原生发行包，完整功能，与官方 skill 配合 |

**lite 不减功能。** 两包目前共享同一个 Ego 后端和完整工具集；都需要 Ego Lite，不宣称已兼容 Edge 等其他浏览器。

## 常用能力

- 任务空间恢复、显式 claim、页面指针回落；关闭页面会报告新的活动页。
- 原生 CSS / text / role / ref / XPath 选择器；可选 `nth`、`visible`、`frame` 筛选有明确边界，不猜目标。
- 表达式、函数和 Promise 统一求值；`run` 保留完整 Ego API，使用 ESM `await import()`。
- 截图支持 `selector`、`clip`、`scale`、`fullPage`、`saveAs`；最小化错误时尝试还原该窗口。
- 上传、动态文件选择器、下载、弹窗、网络捕获、二进制 `fetch(saveAs)`、显式本地文件打开。
- 长结果保留合法 JSON；data URL 预览、完整超限结果文件、按整行截断的快照、可选调试报错。
- `ego_lite_do(steps)` 顺序执行已有工具，遇错停止并报告已完成步骤，避免盲目重放。

详细使用、真实能力边界和安全说明见 [Agent guide](docs/AGENT_GUIDE.md)；逐项反馈处理见 [Feedback status](docs/FEEDBACK_STATUS.md)。

## 构建与测试

```powershell
.\build.ps1 -GoExe <go>         # 两种构建、六个平台
.\test-install-package.ps1 -GoExe <go> -BashExe <bash>
node .\browser_helpers.test.mjs
```

两个构建分别用普通 Go 构建与 `-tags egoonly` 构建。测试用临时 profile，不改真实用户配置。

MIT
