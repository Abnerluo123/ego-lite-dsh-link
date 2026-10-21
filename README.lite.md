# zero-ego：Ego 原生发行包

lite 表示只面向 Ego Lite 及官方 skill，**不表示功能精简**。本包与标准包共享完整工具集与 Ego 后端，动作都默认简短返回。

安装：Windows 用 `install.ps1 -ApplyProfile`；macOS/Linux 用 `bash install.sh --apply`。

需要页面树时显式调用 `ego_lite_snapshot`；需要完整动作返回时用 `ego_lite_output(brief:false)`。截图、上传下载、弹窗、frames、claim、evaluate、run、CDP 均保留。

本包不兼容其他浏览器。标准包当前也使用 Ego 后端；没有虚构的 Edge 兼容层。

构建：`go build -tags egoonly -trimpath -ldflags "-s -w" .`

参见 `docs/AGENT_GUIDE.md` 的权限边界、完整用法与已知限制。
