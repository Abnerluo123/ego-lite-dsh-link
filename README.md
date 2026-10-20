# Ego Browser DSH Link

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

把 **Ego Lite** 专用浏览器接进 [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) 的 MCP 连接器。装上之后，Agent 直接调用 `mcp__ego_lite__*` 工具操作浏览器，**不需要每次授权**。

- **主版本**：Go，单文件、零运行时依赖，6 个平台预编译（约 3 MB）。
- **参考版本**：`python/`，同等 47 个工具，方便阅读或改造。

## 优势

| 对比 | Ego Lite + 本连接器 | Edge/Chrome + CDP |
| --- | --- | --- |
| 授权 | 装好后 Agent 直接调用，无逐次确认 | 需要开 `--remote-debugging-port`、独立 profile，Chrome 136+ 还禁止默认 profile 调试 |
| 启动 | 浏览器常驻，调用即用 | 常需重启一个干净实例 |
| 页面理解 | 内置可访问性树 + 稳定 refs | 自己拉 AXTree、维护 ref 映射 |
| 多标签 | 任务空间 + 自动跟随新标签 | 自己管 targetId 与会话 |
| 分发 | 一个 exe，拷走就能用 | 依赖 Playwright/CDP 运行时 |

代价：Ego Lite 是独立的 agent 浏览器，日常账号需要单独登录一次；它不替代你的主力浏览器。

## 一句话安装（发给 AI）

> 把这个 zip 交给你的 AI：解压后按系统运行 `install.ps1 -ApplyProfile`（Windows）或 `./install.sh --apply`（macOS/Linux），确保连接器装到 DSH 工作区之外，然后确认 `mcp__ego_lite__*` 工具可用。

更完整的安装步骤见 [AGENTS.md](AGENTS.md)，它就是写给 Agent 看的。

## 两个版本

| | 完整版（main） | Lite 版（`lite` 分支） |
| --- | --- | --- |
| 工具数 | 49 | 12 |
| 动作返回 | 可访问性树（可用 `brief` 关掉） | 只有 `page/url/title` |
| 典型 token | 几百到几千 | 几十 |
| 二进制 | 3.2–3.3 MB | 2.8–3.0 MB |
| 适用 | 网络抓包、上传下载、多空间、通用能力 | 只想让 Agent 开网页、点、填、读 |

Lite 保留 `run` 作为逃生舱，能力不会丢，只是默认更省。`build.ps1` 会同时产出 `ego-dsh-lite-*`。

## 能用上 Ego 的哪些能力

`ego_lite_run` 是完整逃生舱：它把 `task`（当前任务空间）和 `page`（当前页）放进作用域，再执行你给的任意脚本，因此 Ego Lite 文档里的 API 都能用——`page.evaluate()`、`page.cdp()`、上传 `setInputFiles`、下载 `waitForEvent("download")`、多标签页、鼠标键盘、各种 wait。

| 工具 | 覆盖 |
| --- | --- |
| `ego_lite_start` / `ego_lite_finish` | 任务空间生命周期（`keep` 可保留结果页） |
| `ego_lite_navigate` / `ego_lite_reload` | 导航 / 刷新 + 快照 |
| `ego_lite_snapshot` / `ego_lite_page_info` | 可访问性树 / 页面身份与尺寸 |
| `ego_lite_click` / `dblclick` / `hover` / `fill` / `press` / `select` / `focus` | 元素交互 |
| `ego_lite_mouse_click` / `ego_lite_mouse` | 坐标点击 / move、down、up |
| `ego_lite_scroll` | 滚轮（dy/dx） |
| `ego_lite_type` / `ego_lite_key` / `ego_lite_keyboard` | 输入、按键、down/up/insert/paste |
| `ego_lite_drag` | 元素拖拽 |
| `ego_lite_wait_for` / `wait_url` / `wait_function` / `wait_timeout` / `wait_event` | 五种等待 + download/popup/dialog/filechooser 事件 |
| `ego_lite_dialog` | accept / dismiss 原生弹窗 |
| `ego_lite_upload` / `ego_lite_download` | 上传文件 / 点击并保存下载 |
| `ego_lite_evaluate` / `ego_lite_cdp` / `ego_lite_fetch` | 页面 JS / 裸 CDP / 带 cookie 与自动 csrf 的页内请求 |
| `ego_lite_network_start` / `network_stop` | 抓 CDP Network 事件，看应用真正调了哪些 API |
| `ego_lite_state` | 读 `window.__INITIAL_STATE__` 等页面状态，比爬 DOM 便宜 |
| `ego_lite_output` | `brief`（不返回可访问性树）、`maxChars`（截断）、`autoSwitch`（跟随新标签） |
| `ego_lite_tabs` / `switch` / `new_page` / `close_page` / `adopt_user_page` / `release_page` | 多标签与用户标签接管 |
| `ego_lite_screenshot` | 截图到临时目录 |
| `ego_lite_handoff` | 把控制权交回用户 |
| `ego_lite_run` | 完整 API 逃生舱 |

共 47 个工具，覆盖官方指南列出的 TaskSpace / Page / Pointer / Keyboard / Dialogs / evaluate / fetch / cdp 全部接口。

> 新标签默认自动跟随：动作后如果只多出一个标签，工具会切过去并在结果里带 `autoSwitchedTo`。要关掉就调 `ego_lite_output(autoSwitch:"false")`。
>
> 页面复杂时先 `ego_lite_output(brief:"true")`，动作只回 `page/url/title`，等需要元素 refs 时再 `brief:"false"` 或单独调 `ego_lite_snapshot`。

## 安装

Windows：

```powershell
.\install.ps1 -ApplyProfile
```

macOS / Linux：

```bash
./install.sh --apply
```

脚本会：挑对应平台的二进制 → 复制到工作区**外**（Windows 是 `%LOCALAPPDATA%\ego-dsh-link\`）→ 把 `mcp-ego-lite` 追加到 profile 的 `cordis.patch.yml`（已存在则不动）。

**必须放在工作区外**：DSH 的 Windows 沙箱会拦截工作区内可执行文件的 Ego bootstrap IPC，否则每次调用都报 `Failed to connect to ego_cli bootstrap`。

改完 profile 后，DSH 会自动重载；工具以 `mcp__ego_lite__*` 出现。

## 怎么用

装好之后不用做别的，直接在对话里说你要做什么：

- “打开 example.com 看看标题”
- “去 httpbin 的表单页，把 Customer name 填成 X，然后提交”
- “截图给我看看这个页面”
- “登录后台，把订单列表导出成 CSV”（会用到 `ego_lite_run` 收表格）

Agent 会自己按 `start → 操作 → finish` 的节奏调用，结束时关闭自己开的页面。你不需要批准，也不需要切 Full Access。

想让它停，直接说“停”即可；任务空间会在会话结束时清理。

## 移除

删掉 profile 里 `mcp-ego-lite` 那个 `insert` 项，重载即可。连接器不改任何全局沙箱、审批或权限设置。

## 移植到另一台电脑

1. 拷 `dist/` 里对应平台的二进制（或整个 zip）。
2. 运行对应平台的安装脚本。
3. 若自动找不到 `ego-browser`，用 `-EgoCli` / `--ego` 指定。

也可以把整个 zip 丢给别的 Agent，让它照 `AGENTS.md` 一句话装完。

## 构建

需要 Go 1.22+：

```powershell
.\build.ps1 -GoExe <go 路径>   # 先跑 go test，再出 6 个平台
.\package.ps1                  # 打成 dist/ego-browser-dsh-link-<版本>.zip
```

`build.ps1` 会把 GOCACHE/GOTMPDIR 放到项目内，绕开某些 Windows 配置档下 `%LOCALAPPDATA%\go-build` 无权限的问题。

## 两个 Windows 坑（已在代码里处理）

1. **二进制别放工作区**（见上）。
2. **脚本走 stdin，不走 `-e`**。非 shell 父进程下 `ego-browser nodejs -e "..."` 会静默挂死。

另外 Ego 把脚本的 `console.log` 写到 **stderr**，成功时也可能返回非零退出码，所以成功判定是扫描两个流里的 `__EGO_DSH_RESULT__` 标记，而不是看退出码。

## 关于性能和续航

连接器 99.9% 的时间阻塞在 stdin 上，不轮询、不占 CPU，所以对轻簿本的续航影响可以忽略。Go 版约 18 ms 启动、Python 版约 50 ms，这点差异只在每次调用时出现一次；真正耗时的是 `ego-browser` 启动和页面动作（几百毫秒到几秒）。换句话说：换语言是为了分发方便和启动干脆，不是为了省电。

## 验证记录（Ego Lite 0.6.0.16）

| 能力 | 证据 |
| --- | --- |
| 导航/快照 | example.com、httpbin 表单页、bilibili 首页 |
| 表单 | 提交后 httpbin 回显 `"custname": "Go connector"` |
| 点击 | 提交按钮、IANA 链接、B 站视频卡片（新开 p3） |
| 页面 JS | `ego_lite_run` 返回 `{title, links, viewport}` |
| 媒体页 | B 站视频页读到 `video` 状态：`duration 108s / readyState 4 / 播放中` |
| 多标签 | 搜索与视频各开一个标签，`ego_lite_switch` 切到 p2 / p3 |
| 等待/悬停 | `wait_for` 等视频卡片；`hover` 成功，歧义选择器会返回候选列表 |
| 坐标点击 | 注入按钮后按 (384,494) 点击，事件坐标精确回传 |
| 滚动 | 滚轮 dy=700，`scrollY` 585 → 1285 |
| 键盘输入 | `type` 写入 `hello 世界 123` 并回读；Wikipedia 搜索框输入后回车跳转 |
| 拖拽 | 指针拖动 `#drag`：left 0→60px、top 0→100px |
| 按键 | 页面级 `Tab` 移动焦点；`Enter` 触发表单跳转 |
| 截图 | 1728×979 PNG，可直接查看 |
| 清理 | `finish` 后任务空间列表为空 |

### 真实站点巡检（2026-10-21）

| 站点 / 任务 | 结果 |
| --- | --- |
| Bing 搜索 | 搜到 5 条“captcha demo”结果 |
| GitHub | `/trending` 取到前 3 个仓库名 |
| 微信安装包 | 从 `pc.weixin.qq.com` 下载 `WeChatWin_4.1.15.exe`，244.7 MB 落盘 |
| DeepSeek 网页版 | 未登录直接进登录页，无输入框，无法打招呼 |
| 豆包网页版 | 输入区被登录弹窗遮挡（点击被 dialog 拦截） |
| B 站登录态 | `api.bilibili.com/x/web-interface/nav` 返回 `isLogin:false (code -101)`，只有匿名 cookie |
| reCAPTCHA v2 演示 | iframe 内复选框可定位并点击，但演示站提示“已超出 Enterprise 免费配额”，未出现挑战图 |
| Cloudflare Turnstile 演示 | 组件未渲染（无 iframe），无法挑战 |
| 多开 | `new_page` 新开 p2、切回 p1、关闭 p2；另起二号空间 16 跑 example.org 后正常关闭 |

`go test ./...`、`go vet`、Python 的 13 个 unittest 均通过；Go 与 Python 版本工具数均为 47。

## 已知可改进点

- **超长 URL 噪声**：广告/埋点的 `href` 仍可能很长，目前只能靠 `brief` 或 `maxChars` 规避，还没有按 URL 长度裁剪。
- **自动跟随只处理“恰好一个新标签”**：一次开出两个标签时不会切，需要 `ego_lite_switch`。
- **快照仍是整体文本**：`maxChars` 是字符截断，不是按节点裁剪。
