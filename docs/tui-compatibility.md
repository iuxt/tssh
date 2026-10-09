# TUI 键盘和滚轮兼容性评估

主机列表按配置顺序平铺，每台主机只显示一次。`GroupLabels` 用于搜索筛选。主机列表、搜索、详情和表单的键盘操作可在受支持系统的 ANSI 终端上使用；鼠标还要求终端或中间层将事件交给应用，不能保证任意系统和任意终端组合均支持鼠标。

## 列表行为

- 主机按原配置顺序显示，多个 `GroupLabels` 不会重复显示主机。
- `←` / `→` 或 `PageUp` / `PageDown` 翻页，`Home` / `End` 跳到首尾。
- 搜索匹配别名、地址和所有分组标签，支持锁定筛选、取消当前输入与清空筛选。
- 保存配置后清空筛选，并选中已保存主机。

## 鼠标行为与平台条件

应用使用项目已锁定的 Bubble Tea v2.0.6 的 `MouseModeCellMotion` 和 `MouseWheelMsg`，只处理上下滚轮。鼠标点击、拖动、释放和带修饰键的事件不触发应用操作。鼠标坐标按终端字符格计算，中文宽度仍由 ANSI 宽度库处理。

主机选择、搜索和面板切换使用键盘，连接由 `Enter` 确认。滚轮在指针所在的列表、详情或帮助中浏览；列表每次移动三行。编辑表单中滚轮切换三项，输入光标移到字段末尾。边框、分隔线及越界滚轮不会切换其他行；极小终端保留键盘操作。

| 环境 | 键盘操作 | 滚轮条件 | 验证结论 |
| --- | --- | --- | --- |
| macOS，支持 ANSI 的终端 | 共用实现 | 终端需上报鼠标；iTerm2 需启用鼠标和滚轮上报 | 本机单元测试、构建与 PTY 协议检查；终端产品仍需实测 |
| Linux，支持 ANSI 的终端 | 共用实现 | 终端需上报 XTerm/SGR 鼠标事件 | 交叉构建、测试二进制编译；未在 Linux 图形终端实测 |
| Windows 10/11，Windows Terminal 或现代控制台 | 共用实现 | Bubble Tea 的 Windows 输入层处理控制台事件；终端设置仍可能拦截鼠标 | 交叉构建、测试二进制编译；未在 Windows 实机运行 |
| Windows 的 Cygwin / MSYS2 / Git Bash 等路径 | 依赖已有 VT / stty 回退 | 若进入 `isRunningOnOldWindows` 字节流回退，应用关闭鼠标捕获 | 回退状态有单元测试；不同终端组合需实测 |
| Windows 7、旧 Windows | 依赖专用工具链、兼容构建和可用终端 | 旧输入回退只提供键盘操作 | 本次未验证专用 Win7 工具链及实机，不承诺运行兼容 |
| SSH / tmux / screen 嵌套 | 终端转发正常时可用 | 每一层都需正确转发鼠标事件，外层绑定或复制模式可能截获 | 需按实际组合实测 |
| 无 ANSI 支持的终端、非交互管道 | 不承诺 TUI | 无可用的鼠标事件来源 | 使用显式目标主机的命令行连接 |

Windows 的最低系统要求还取决于 Go 运行时。官方 Go 自 1.21 起要求 Windows 10 或 Windows Server 2016 及以上；仓库的 Win7 发布任务使用额外的兼容工具链，因此“能交叉编译 Windows 二进制”不能证明它能在 Win7 上运行。[Go 官方说明](https://go.dev/doc/go1.21#windows)

终端条件依据：[Windows Terminal 鼠标输入](https://github.com/MicrosoftDocs/terminal/blob/main/TerminalDocs/tips-and-tricks.md)、[iTerm2 鼠标和滚轮上报设置](https://iterm2.com/documentation-preferences-profiles-terminal.html)、[XTerm 鼠标协议](https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking)。这些资料说明协议和设置条件，不能替代实机验证。

## 验证范围

当前本机工具链为 Go 1.27.1；项目 `go.mod` 声明 Go 1.25.0。仓库现有 `.github/workflows/gotest.yml` 在 Linux、macOS、Windows 上运行测试，并含 Win7 兼容工具链构建任务。此次本地检查不等同于已运行这些 CI 任务。

平铺列表恢复后已通过 `go test ./... -count=1`、`go vet ./...` 和 `git diff --check`。

此前滚轮功能接入时还通过以下验证：

- `go test ./... -count=1`、`go vet ./...`、`git diff --check`。
- `CGO_ENABLED=0` 的 11 个构建目标：macOS `amd64` / `arm64`，Windows `386` / `amd64` / `arm64`，Linux `386` / `amd64` / `armv6` / `armv7` / `arm64` / `loong64`。
- Windows 和 Linux `amd64` 的 `go test -c ./tssh` 测试二进制编译。
- macOS 伪终端启动实际二进制，注入 SGR / X10 鼠标点击、滚轮和键盘输入，验证点击不触发操作、滚轮选中、正常退出，以及关闭鼠标捕获、退出备用屏幕、恢复终端模式。此项验证输入协议与应用行为，不等同于 iTerm2、Terminal.app 等终端产品的实测。

平铺列表恢复后，回归测试覆盖多标签主机只显示一次、带标签列表的左右键翻页、标签搜索、列表和详情滚轮、点击不触发操作、保存后的主机选中、表单布局、窄屏以及旧 Windows 键盘回退。Windows 和 Linux 的测试二进制编译仅验证编译兼容。

## 实机验收步骤

1. 创建多标签、无标签及超过一页的主机配置，启动 `tssh -F <测试配置>`，确认主机按配置顺序显示，每台主机只出现一次，顶部计数正确。
2. 用 `←`、`→` 翻页，用键盘和滚轮切换主机，确认鼠标点击不触发操作，`Enter` 确认连接。
3. 按 `/` 搜索主机的任意标签，确认筛选正确；按 `Ctrl+E` 清空后恢复完整列表。
4. 调整窗口至宽屏、窄屏和很小的尺寸，测试分页、中文列宽、详情和帮助的滚动。
5. 在编辑表单中用键盘或滚轮切换字段，确认点击字段不改变焦点，取消编辑；另用临时配置保存主机，确认选中已保存主机。
6. 退出 TUI，确认鼠标不再被应用捕获、终端回显恢复。分别在目标终端、SSH 和实际使用的 tmux / screen 组合中重复上述步骤。
