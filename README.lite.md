# zero-ego (Ego 原生版)

同一套代码用 `-tags egoonly` 构建的变体，**只面向 Ego Lite 与官方 skill**，不追求兼容 Edge 等其他浏览器。

和通用版的唯一区别是默认值，不是功能：

| | 通用版 | Ego 原生版 |
| --- | --- | --- |
| 工具数 | 50 | **50**（完全相同） |
| 动作默认返回 | 可访问性树 | `page/url/title`（`brief` 默认开） |
| 自动跟随新标签 | 开 | 开 |
| `ego_lite_snapshot` | 返回树 | 需显式 `brief:"false"` 或调 snapshot |

这样每次动作只回三个字段，更贴近官方 skill 的“先看再动”，token 开销低，但 50 个工具一个不少：上传、下载、弹窗、网络抓包、页面状态、多标签、`run` 全都在。

## 安装

```powershell
.\install.ps1 -ApplyProfile      # Windows
./install.sh --apply             # macOS / Linux
```

安装脚本会自动识别 lite 二进制，装成 `zero-ego-lite-<version>.exe`，放在 DSH 工作区之外，然后在 profile 中写入 `mcp-ego-lite`。

## 构建

```powershell
go build -trimpath -tags egoonly -ldflags "-s -w" -o zero-ego-lite.exe .
```

MIT
