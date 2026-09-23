## tssh: 高度兼容 OpenSSH 并提供丰富扩展功能的 SSH 客户端

[![MIT License](https://img.shields.io/badge/license-MIT-green.svg?style=flat)](https://choosealicense.com/licenses/mit/)
[![GitHub Release](https://img.shields.io/github/v/release/trzsz/trzsz-ssh)](https://github.com/trzsz/trzsz-ssh/releases)
[![WebSite](https://img.shields.io/badge/WebSite-https%3A%2F%2Ftrzsz.github.io%2Ftssh-blue?style=flat)](https://trzsz.github.io/tssh)
[![中文文档](https://img.shields.io/badge/%E4%B8%AD%E6%96%87%E6%96%87%E6%A1%A3-https%3A%2F%2Ftrzsz.github.io%2Fcn%2Ftssh-blue?style=flat)](https://trzsz.github.io/cn/tssh)

tssh 设计为 ssh 客户端的直接替代品，提供与 openssh 完全兼容的基础功能，同时实现其他有用的扩展功能。

### 为什么做

- 服务器太多，记不住所有别名，`tssh` 内置登录界面，支持搜索和选择服务器登录。

- `tssh` 登录服务器后，内置支持 lrzsz zmodem ( rz / sz )，传文件无需另外新开窗口。

- 有些服务器不支持公钥登录，`tssh` 支持记住密码，支持自动交互，提升登录的效率。

### 安装方法


- 用 Go 自己编译（ 要求 go 1.25 以上 ）

  <details><summary><code>sudo make install</code></summary>

  ```sh
  git clone --depth 1 https://github.com/iuxt/tssh.git
  cd tssh
  go build .
  ```

  </details>

### 登录界面

- 使用之前，需要配置好 `~/.ssh/config` ( Windows 是 `C:\Users\xxx\.ssh\config`, `xxx` 换成用户名 )。

- 关于如何配置 `~/.ssh/config`，请参考 [openssh](https://manpages.debian.org/bookworm/openssh-client/ssh_config.5.en.html) ( `Match` 中的 `exec` 暂时要参考下文配置 `UseOpenSSHConfig` 才支持 )，或参考 tssh wiki [SSH基本配置](https://github.com/trzsz/trzsz-ssh/wiki/SSH%E5%9F%BA%E6%9C%AC%E9%85%8D%E7%BD%AE)。

- 直接无参数运行 `tssh` 命令就会打开登录界面，或者有除目标机器外的其他参数也会打开登录界面。

- 如果目标机器参数是 `~/.ssh/config` 中别名的一部分，不能完全匹配某个别名，也会打开登录界面。

- 如果配置了 `HideHost yes`，或者别名中含有 `*` 或 `?` 通配符时，则不会显示在登录界面中。

- `tssh` 支持很多快捷键和搜索功能。

  | 操作      | 全局快捷键                      | 非搜索快捷键 | 快捷键描述      |
  | --------- | ------------------------------- | ------------ | --------------- |
  | Confirm   | Enter                           |              | 确认并登录      |
  | Quit/Exit | Ctrl+C Ctrl+Q                   | q Q          | 取消并退出      |
  | Move Prev | Ctrl+K Shift+Tab ↑              | k K          | 往上移光标      |
  | Move Next | Ctrl+J Tab ↓                    | j J          | 往下移光标      |
  | Page Up   | Ctrl+H Ctrl+U Ctrl+B PageUp ←   | h H u U b B  | 往上翻一页      |
  | Page Down | Ctrl+L Ctrl+D Ctrl+F PageDown → | l L d D f F  | 往下翻一页      |
  | Goto Home | Home                            | g            | 跳到第一行      |
  | Goto End  | End                             | G            | 跳到最尾行      |
  | EraseKeys | Ctrl+E                          | e E          | 擦除搜索关键字  |
  | TglSearch | /                               |              | 切换搜索功能    |
  | Tgl Help  | ?                               |              | 切换帮助信息    |

### 主题风格

- `tssh` 支持多种主题风格，在 `$XDG_CONFIG_HOME/tssh/tssh.conf` ( 或 `~/.tssh.conf` ) 中配置 `PromptThemeLayout` 选用。欢迎一起来创造更多更好看的。

- 每种主题风格都支持自定义颜色，在 `$XDG_CONFIG_HOME/tssh/tssh.conf` ( 或 `~/.tssh.conf` ) 中配置 `PromptThemeColors`，只要配置非默认的颜色即可。

- 请为你喜欢的主题风格[❤️投票❤️](https://github.com/trzsz/trzsz-ssh/issues/75)，得票数最高的主题风格将会在下个版本被设置为默认主题。

#### tiny 小巧风

- 在 `$XDG_CONFIG_HOME/tssh/tssh.conf` ( 或 `~/.tssh.conf` ) 中配置 `PromptThemeLayout = tiny` 选用 `tiny 小巧风`。
  ![tssh tiny](https://trzsz.github.io/images/tssh_tiny.gif)

- 在 `$XDG_CONFIG_HOME/tssh/tssh.conf` ( 或 `~/.tssh.conf` ) 中配置 `PromptThemeColors`，要求配置成一行。`tiny 小巧风` 支持以下配置项：

  <details><summary><code>tiny 颜色配置项和默认值：</code></summary>

  ```json
  {
    "help_tips": "faint",
    "shortcuts": "faint",
    "label_icon": "blue",
    "label_text": "default",
    "cursor_icon": "green|bold",
    "active_alias": "cyan|bold",
    "active_host": "magenta|bold",
    "active_group": "blue|bold",
    "inactive_alias": "cyan",
    "inactive_host": "magenta",
    "inactive_group": "blue",
    "details_title": "default",
    "details_name": "faint",
    "details_value": "default"
  }
  ```

  </details>

  <details><summary><code>tiny 支持的颜色枚举，可用 `|` 连接多个：</code></summary>

  ```
  default
  black
  red
  green
  yellow
  blue
  magenta
  cyan
  white
  bgBlack
  bgRed
  bgGreen
  bgYellow
  bgBlue
  bgMagenta
  bgCyan
  bgWhite
  bold
  faint
  italic
  underline
  ```

  </details>

#### simple 简约风

- 在 `$XDG_CONFIG_HOME/tssh/tssh.conf` ( 或 `~/.tssh.conf` ) 中配置 `PromptThemeLayout = simple` 选用 `simple 简约风`。
  ![tssh simple](https://trzsz.github.io/images/tssh_simple.gif)

- `simple 简约风` 支持的颜色配置项、默认值和颜色枚举，和 `tiny 小巧风` 完全相同，请参考前文。

#### table 表格风

- 在 `$XDG_CONFIG_HOME/tssh/tssh.conf` ( 或 `~/.tssh.conf` ) 中配置 `PromptThemeLayout = table` 选用 `table 表格风`。
  ![tssh table](https://trzsz.github.io/images/tssh_table.gif)

- 在 `$XDG_CONFIG_HOME/tssh/tssh.conf` ( 或 `~/.tssh.conf` ) 中配置 `PromptThemeColors`，要求配置成一行。`table 表格风` 支持以下配置项：

  <details><summary><code>table 颜色配置项和默认值：</code></summary>

  ```json
  {
    "help_tips": "faint",
    "shortcuts": "faint",
    "table_header": "10",
    "default_alias": "6",
    "default_host": "5",
    "default_group": "4",
    "default_border": "8",
    "selected_border": "10",
    "details_name": "4",
    "details_value": "3",
    "details_border": "8"
  }
  ```

  </details>

- 支持的颜色枚举请参考 [lipgloss](https://github.com/charmbracelet/lipgloss#colors)，除了 `help_tips` 和 `shortcuts` 与前文 `tiny 小巧风` 相同。

### 支持 lrzsz zmodem

- `rz / sz` 功能默认开启。

- 除了服务器，本地电脑也要安装 `lrzsz`，Windows 可以从 [lrzsz-win32](https://github.com/trzsz/lrzsz-win32/releases) 下载，解压并加到 `PATH` 环境变量中，也可以如下安装：

  ```
  scoop install lrzsz / choco install lrzsz / winget install lrzsz
  ```

- 在 `~/.ssh/config` 或 `ExConfigPath` 配置文件中，配置 `EnableZmodem` 为 `No` 禁用 `rz / sz` 功能。

  ```
  Host no_zmodem
    # 如果该文件也会被标准 ssh 使用，请将 tssh 专有配置放到 `ExConfigPath` 中
    EnableZmodem No
  ```

- 关于 `rz / sz` 进度条，己传大小和传输速度会有一点偏差，它的主要作用只是指示传输正在进行中。

- 可在命令行中使用 `sz` 直接下载文件到本地，如：

  ```sh
  tssh -t xxx_server 'sz /path/to/file1 /path/to/file2'
  ```

### 支持 scp sftp

- 使用了 `tssh` 记住密码的功能，登录时不用手工输入密码了，`scp` 和 `sftp` 也一样可以不用手工输入密码。

- 只要 `scp` 和 `sftp` 使用 `-S` 选项指定 `tssh`，或者配置个 alias 即可使用 `tssh` 提供的一些功能，如：

  ```sh
  sftp -S tssh xxx
  scp -S tssh xxx @xxx:/tmp/
  alias tscp='scp -S tssh'
  alias tsftp='sftp -S tssh'
  ```

### 分组标签

- 如果服务器数量很多，分组标签 `GroupLabels` 可以在按 `/` 搜索时，快速找到目标服务器。

- 按 `/` 输入分组标签后，`回车`可以锁定；再按 `/` 可以输入另一个分组标签，`回车`再次锁定。

- 在非搜索模式下，按 `E` 可以清空当前搜索标签；在搜索模式下按 `Ctrl + E` 也是同样效果。

- 支持在一个 `GroupLabels` 中以空格分隔，配置多个分组标签；支持配置多个 `GroupLabels`。

- 支持以通配符 \* 的形式，在多个 Host 节点配置分组标签，`tssh` 会将所有的标签汇总起来。

  ```
  # 以下 testAA 具有标签 group1 group2 label3 label4 group5
  Host test*
      GroupLabels group1 group2
      GroupLabels label3
  Host testAA
      GroupLabels label4 group5
  ```

### 自动交互

- 支持类似 `expect` 的自动交互功能，在登录服务器之后，自动匹配服务器的输出，然后自动输入。

  ```
  Host auto
      ExpectCount 2  # 配置自动交互的次数，默认是 0 即无自动交互
      ExpectTimeout 30  # 配置自动交互的超时时间（单位：秒），默认是 30 秒
      ExpectPattern1 *assword  # 配置第一个自动交互的匹配表达式
      ExpectSendText1 123456\r  # 配置第一个自动输入（明文），需要指定 \r 才会发送回车
      ExpectPattern2 hostname*$  # 配置第二个自动交互的匹配表达式
      ExpectSendText2 echo tssh expect\r  # 配置第二个自动输入（明文），需要指定 \r 才会发送回车
  ```

- 在每个 `ExpectPattern?` 匹配之前，如果遇到可选的匹配则自动输入，用法如下：

  ```
  Host case
      ExpectCount 1  # 配置自动交互的次数，默认是 0 即无自动交互
      ExpectPattern1 hostname*$  # 配置第一个自动交互的匹配表达式
      ExpectSendText1 ssh xxx\r  # 配置第一个自动输入
      ExpectCaseSendText1 yes/no y\r  # 在 ExpectPattern1 匹配之前，若遇到 yes/no 则发送 y 并回车
      ExpectCaseSendText1 y/n yes\r   # 在 ExpectPattern1 匹配之前，若遇到 y/n 则发送 yes 并回车
  ```

- 在匹配到指定输出时，自动生成 `totp` 2FA 双因子验证码，然后自动输入，用法如下：

  ```
  Host totp
      ExpectCount 1  # 配置自动交互的次数，默认是 0 即无自动交互
      ExpectPattern1 token:  # 配置第一个自动交互的匹配表达式
      ExpectSendTotp1 xxxxx  # 配置 totp 的 secret（明文），一般可通过扫二维码获得
  ```

- 在匹配到指定输出时，执行指定的命令获取动态密码，然后自动输入，用法如下：

  ```
  Host otp
      ExpectCount 1  # 配置自动交互的次数，默认是 0 即无自动交互
      ExpectPattern1 token:  # 配置第一个自动交互的匹配表达式
      ExpectSendOtp1 oathtool --totp -b xxxxx  # 配置获取动态密码的命令（明文）
  ```

- 可能有些服务器不支持连着发送数据，如输入 `1\r`，要求在 `1` 之后有一点延迟，然后再 `\r` 回车，则可以用 `\|` 间开。

  ```
  Host sleep
      ExpectCount 2  # 配置自动交互的次数，默认是 0 即无自动交互
      ExpectSleepMS 100  # 当要间开输入时，sleep 的毫秒数，默认 100ms
      ExpectPattern1 x>  # 配置第一个自动交互的匹配表达式
      ExpectSendText1 1\|\r  # 配置第一个自动输入，在发送 1 之后，先 sleep 100ms，再发送 \r 回车
      ExpectPattern2 y>  # 配置第二个自动交互的匹配表达式
      ExpectSendText2 \|1\|\|\r  # 先 sleep 100ms，然后发送 1，再 sleep 200ms，最后发送 \r 回车
  ```

- 有些服务器连密码也不支持连着发送，则需要配置 `ExpectPassSleep`，默认为 `no`，可配置为 `each` 或 `enter`：

  - 配置 `ExpectPassSleep each` 则每输入一个字符就 sleep 一小段时间，默认 100 毫秒，可配置 `ExpectSleepMS` 进行调整。
  - 配置 `ExpectPassSleep enter` 则只是在发送 `\r` 回车之前 sleep 一小段时间，默认 100 毫秒，可配置 `ExpectSleepMS` 进行调整。

- 如果不知道 `ExpectPattern2` 如何配置，可以先将 `ExpectCount` 配置为 `2`，然后使用 `tssh --debug` 登录，就会看到 `expect` 捕获到的输出，可以直接复制输出的最后部分来配置 `ExpectPattern2`。把 `2` 换成其他任意的数字也适用。

### 记住密码

- 推荐使用公钥认证登录，可参考 openssh 的文档，或者参考 tssh wiki [公钥认证登录](https://github.com/trzsz/trzsz-ssh/wiki/%E5%85%AC%E9%92%A5%E8%AE%A4%E8%AF%81%E7%99%BB%E5%BD%95)。

- 如果只能使用密码登录，建议至少设置一下配置文件的权限，如：

  ```sh
  chmod 700 ~/.ssh && chmod 600 ~/.ssh/password ~/.ssh/config
  ```

- 下面配置 `test1` 和 `test2` 的密码是 `123456`，其他以 `test` 开头的密码是 `111111`：

  ```
  # 如果该文件也会被标准 ssh 使用，请将 tssh 专有配置放到 `ExConfigPath` 中
  Host test1 test2
      Password 123456

  # ~/.ssh/config 和 ~/.ssh/password 是支持通配符的，tssh 会使用第一个匹配到的值。
  # 这里希望 test2 使用区别于其他 test* 的密码，所以将 test* 放在了 test2 的后面。

  Host test*
      Password 111111
  ```

- 如果记住密码后还是要求输入密码，可能是需要[记住答案](#%E8%AE%B0%E4%BD%8F%E7%AD%94%E6%A1%88)，可配置 `QuestionAnswer1` 试试：

  ```
  Host test1
      QuestionAnswer1 123456
  ```

- 如果启用了 `ControlMaster` 多路复用，或者是在旧版本 `Warp` 终端，需要使用前面 `自动交互` 的方式实现记住密码的效果。配置方式请参考前面 `自动交互`，加上 `Ctrl` 前缀即可，如：

  ```
  Host ctrl
      CtrlExpectCount 1  # 配置自动交互的次数，一般只要输入一次密码
      CtrlExpectPattern1 *assword    # 配置密码提示语的匹配表达式
      CtrlExpectSendText1 123456\r  # 配置明文密码并发送回车
  ```

- 支持记住私钥的`Passphrase`（ 推荐使用 `ssh-agent` ）。支持与 `IdentityFile` 一起配置, 支持使用私钥文件名代替 Host 别名设置通用密钥的 `Passphrase`。举例：

  ```
  # IdentityFile 和 Passphrase 一起配置
  Host test1
      IdentityFile /path/to/id_rsa
      Passphrase 123456

  # 在 ~/.ssh/config 中配置通用私钥 ~/.ssh/id_ed25519 对应的 Passphrase
  # 可以加上通配符 * 以避免 tssh 搜索和选择时，文件名出现在服务器列表中。
  Host id_ed25519*
      Passphrase 111111

  # 在 ~/.ssh/password 中配置则不需要通配符*，也不会出现在服务器列表中。
  Host id_rsa
      Passphrase 111111
  ```

- `记住密码`之后还提示输入密码？可能服务器的认证方式是 `keyboard interactive`，请参考下文`记住答案`。

### 外部密码管理器

- 对于任何密钥配置项（如 `Password`、`Passphrase`、`QuestionAnswer1`、`TotpSecret1`），可以在配置项名称后加 `Command` 后缀，通过外部命令在运行时获取密钥。命令的标准输出（去除首尾空白）将作为密钥值。

- 命令中支持以下 token，执行前会自动展开：

  | Token | 展开为 |
  | ----- | ------ |
  | `%n`  | Host 别名（ssh config 中的 `Host` 值） |
  | `%h`  | 远程主机名（`HostName`） |
  | `%r`  | 远程用户名（`User`） |
  | `%p`  | 远程端口（`Port`） |
  | `%%`  | 字面量 `%` |

- 优先级：`enc{Key}`（加密）> `{Key}Command`（外部命令）> `{Key}`（明文）。

- 各种密码管理器配置示例：

  ```
  # gopass (https://github.com/gopass-io/gopass)
  Host server1
      PasswordCommand gopass show -o ssh/%n
      PassphraseCommand gopass show -o ssh/%n/passphrase

  # pass (https://www.passwordstore.org)
  Host server2
      PasswordCommand pass show ssh/%n

  # 1Password CLI
  Host server3
      PasswordCommand op read "op://Vault/ssh-%n/password"

  # macOS 钥匙串
  Host server4
      PasswordCommand security find-generic-password -a %r -s %n -w

  # Bitwarden CLI
  Host server5
      PasswordCommand bw get password ssh-%n

  # HashiCorp Vault
  Host server6
      PasswordCommand vault kv get -field=password secret/ssh/%n

  # 为所有主机使用统一命令
  Host *
      PasswordCommand gopass show -o ssh/%n
  ```

### 记住答案

- 除了私钥和密码，还有一种登录方式，英文叫 keyboard interactive ，是服务器返回一些问题，客户端提供正确的答案就能登录，很多自定义的一次性密码就是利用这种方式实现的。

- 对于只有一个问题，且答案（密码）固定不变的，只要配置 `QuestionAnswer1` 即可。对于有多个问题的，可以按问题的序号进行配置，也可以按问题的 hex 编码进行配置。

- 使用 `tssh --debug` 登录，会输出问题的 hex 编码，从而知道该如何使用 hex 编码进行配置。配置举例：

  ```
  # 如果该文件也会被标准 ssh 使用，请将 tssh 专有配置放到 `ExConfigPath` 中
  Host test1
      QuestionAnswer1 答案一
  Host test2
      QuestionAnswer1 答案一
      QuestionAnswer2 答案二
      QuestionAnswer3 答案三
  Host test3
      6e616d653a20 my_name  # 其中 `6e616d653a20` 是问题 `name: ` 的 hex 编码
      636f64653a20 my_code  # 其中 `636f64653a20` 是问题 `code: ` 的 hex 编码, `my_code` 是明文答案
  ```

- 对于 `totp` 2FA 双因子验证码，则可以如下配置（同样支持按序号或 hex 编码进行配置）：

  ```
  Host totp
      TotpSecret1 xxxxx  # 按序号配置 totp 的 secret（明文），一般可通过扫二维码获得
      totp636f64653a20 xxxxx  # 按 `code: ` 的 hex 编码 `636f64653a20` 配置 totp 的 secret（明文）
  ```

- 对于可以通过命令行获取到的动态密码，则可以如下配置（同样支持按序号或 hex 编码进行配置）：

  ```
  Host otp
      OtpCommand1 oathtool --totp -b xxxxx  # 按序号配置获取动态密码的命令
      otp636f64653a20 oathtool --totp -b xxxxx  # 按 `code: ` 的 hex 编码 `636f64653a20` 配置获取动态密码的命令
  ```

- 可以自己实现获取动态密码的程序，指定 `%q` 参数可以得到问题内容，将动态密码输出到 stdout 并正常退出即可，调试信息可以输出到 stderr （ `tssh --debug` 运行时可以看到 ）。配置举例（序号代表第几个问题，一般只有一个问题，只需配置 `OtpCommand1` 即可）：

  ```
  Host custom_otp_command
      OtpCommand1 /path/to/your_own_program %q
      OtpCommand2 python C:\your_python_code.py %q
  ```

- 如果启用了 `ControlMaster` 多路复用，或者是在旧版本 `Warp` 终端，请参考前面 `自动交互` 加 `Ctrl` 前缀来实现。

  ```
  Host ctrl_totp
      CtrlExpectCount 1  # 配置自动交互的次数
      CtrlExpectPattern1 code:  # 配置密码提示语的匹配表达式（这里以 2FA 验证码举例）
      CtrlExpectSendTotp1 xxxxx  # 配置 totp 的 secret（明文），一般可通过扫二维码获得

  Host ctrl_otp
      CtrlExpectCount 1  # 配置自动交互的次数
      CtrlExpectPattern1 token:  # 配置密码提示语的匹配表达式（这里以动态密码举例）
      CtrlExpectSendOtp1 oathtool --totp -b xxxxx  # 配置获取动态密码的命令（明文）
  ```

### 个性配置

- 支持在 `$XDG_CONFIG_HOME/tssh/tssh.conf` ( 或 `~/.tssh.conf`，Windows 是 `C:\Users\your_name\.tssh.conf` ）中进行以下自定义配置：

  ```
  # SSH 配置路径，默认为 ~/.ssh/config
  ConfigPath = ~/.ssh/config

  # 扩展配置路径，默认为 ~/.ssh/password
  ExConfigPath = ~/.ssh/password

  # 上传时，对话框打开的路径，为空时打开上次的路径， 默认为空
  DefaultUploadPath = ~/Downloads

  # 下载时，自动保存的路径，为空时弹出对话框手工选择，默认为空
  DefaultDownloadPath = ~/Downloads

  # 传输进度条将从第一种颜色渐变到第二种颜色。注意不要带 `#`。
  ProgressColorPair = B14FFF 00FFA3

  # tssh 搜索和选择服务器时，配置主题风格和自定义颜色
  PromptThemeLayout = simple
  PromptThemeColors = {"active_host": "magenta|bold", "inactive_host": "magenta"}

  # tssh 搜索和选择服务器时，每页显示的记录数，默认为 10
  PromptPageSize = 10

  # tssh 搜索和选择服务器时，默认是类似 vim 的 normal 模式，想默认进入搜索模式可如下配置：
  PromptDefaultMode = search

  # tssh 搜索和选择服务器时，详情中显示的配置列表，默认如下：
  PromptDetailItems = Alias Host Port User GroupLabels IdentityFile ProxyCommand ProxyJump RemoteCommand

  # tssh 搜索和选择服务器时，可以自定义光标和选中的图标：
  PromptCursorIcon = 🧨
  PromptSelectedIcon = 🍺

  # 登录后自动设置终端标题，退出后不会重置，你需要参考下文在本地 shell 中设置 PROMPT_COMMAND
  SetTerminalTitle = Yes

  # 使用 `ssh -G` 解析 OpenSSH 配置，包括 `Match` 规则
  UseOpenSSHConfig = Yes
  ```

### 配置注释

- `tssh` 配置中的注释基本与 `openssh` 一致，详见下表：

  | 注释                  | openssh |  tssh  |
  | :-------------------- | :-----: | :----: |
  | `#` 开头的配置行      | 是注释  | 是注释 |
  | `Key Value # Comment` | 看情况  | 是注释 |
  | `Key=Value # Comment` | 看情况  | 非注释 |

- `#` 开头的配置行，`openssh` 和 `tssh` 都会认为是注释。
- `Key Value # Comment` 配置（没有 `=` 号），`openssh` 有些情况认为 `#` 后的内容是注释，有些情况认为不是注释；`tssh` 一律认为 `#` 后的内容是注释。
- `Key=Value # Comment` 配置（有 `=` 号），`openssh` 有些情况认为 `#` 后的内容是注释，有些情况认为不是注释；`tssh` 一律认为 `#` 后的内容不是注释。

### Wayland 集成

- 在 `~/.ssh/config` 或 `ExConfigPath` 配置文件中，配置 `EnableWaypipe` 为 `Yes` 启用 Wayland (waypipe) 集成功能。

  ```
  Host xxx
    # 如果该文件也会被标准 ssh 使用，请将 tssh 专有配置放到 `ExConfigPath` 中
    EnableWaypipe Yes
  ```

- 启用 Wayland (waypipe) 集成功能后，无需再显式使用 waypipe 程序，tssh 将在后台自动运行 waypipe 程序。

- 如果客户端 waypipe 程序在 PATH 路径下找不到，可以通过 `WaypipeClientPath` 配置指定 waypipe 程序的路径。

- 如果服务端 waypipe 程序在 PATH 路径下找不到，可以通过 `WaypipeServerPath` 配置指定 waypipe 程序的路径。

- 可以根据需要，通过 `WaypipeClientOption` 和 `WaypipeServerOption` 配置指定 waypipe 程序的一些参数，注意不要指定 `-s`、`--socket`、`--login-shell`、`--display`、`client`、`server` 这些参数，配置举例：

  ```
  Host xxx
    EnableWaypipe Yes
    WaypipeClientPath /usr/bin/waypipe
    WaypipeServerPath /usr/bin/waypipe
    WaypipeClientOption -c lz4
    WaypipeServerOption -c lz4
  ```

### 剪贴板集成

- 在 `~/.ssh/config` 或 `ExConfigPath` 配置文件中，配置 `EnableOSC52` 为 `Yes` 启用剪贴板集成功能。

  ```
  Host *
    # 如果该文件也会被标准 ssh 使用，请将 tssh 专有配置放到 `ExConfigPath` 中
    EnableOSC52 Yes
  ```

- 启用剪贴板集成功能后，支持远程服务器通过 OSC52 序列写入本地剪贴板。

- 在 Linux 系统，剪贴板集成功能需要安装 `xclip` 或 `xsel` 命令。

### SSH 控制台

- `tssh` 控制台是类似 OpenSSH escape sequences 的功能，计划提供更友好、更强大的 SSH 控制功能。目前已支持的功能有：

  - 发送转义字符 '~' ( ~ : 相当于输入 `~`，可作为控制台误触发后的补救措施 )。
  - 暂停当前 SSH 进程 ( ^Z : 相当于 `Ctrl + Z`，不是作用于远程服务器上的进程，而是作用于 `tssh` 自身 )。
  - 退出当前 SSH 会话 ( . : 相当于 Exit / Kill，当因为网络等原因导致 `tssh` 卡死时，可通过此功能退出 )。

- 上面 `(` 与 ``:` 之间的字符是快捷键，兼容 OpenSSH escape sequences，例如回车后 `~.` 可以快速退出当前 SSH 会话。

- 可通过 `EscapeChar` 选项配置进入 SSH 控制台的转义字符（ 默认是 `~` ），只支持一个字符，或者 ^ 带一个字母，并且不能与其他快捷键冲突。

- 可通过 `ConsoleEscapeTime` 选项配置按下 `回车` 键后多少秒内按下 `~` 键即进入 SSH 控制台，默认值是 `1` 秒，可以配置为 `0` 禁用控制台功能：

  ```
  Host xxx
    EscapeChar ~
    ConsoleEscapeTime 1
  ```

### 其他功能

- 关于修改终端标题，其实无需 `tssh` 就能实现，只要在服务器的 shell 配置文件中（如`~/.bashrc`）配置：

  ```sh
  # 设置固定的服务器标题
  PROMPT_COMMAND='echo -ne "\033]0;固定的服务器标题\007"'

  # 根据环境变量动态变化的标题
  PROMPT_COMMAND='echo -ne "\033]0;${USER}@${HOSTNAME}: ${PWD}\007"'
  ```

  - 如果在 `$XDG_CONFIG_HOME/tssh/tssh.conf` ( 或 `~/.tssh.conf` ) 中设置了 `SetTerminalTitle = Yes`，则会在登录后自动设置终端标题，但是服务器上的 `PROMPT_COMMAND` 会覆盖 `tssh` 设置的标题。
  - 在 `tssh` 退出后不会重置为原来的标题，你需要在本地 shell 中设置 `PROMPT_COMMAND`，让它覆盖 `tssh` 设置的标题。

- 支持 DNS SRV，假设你家里有多台主机，但你只有一个公网 IP，你可以像下面这样设置 SRV 记录，并在 `~/.ssh/config` 中类似配置：

  ```sh
  $ dig +short _ssh._tcp.myhost.mydomain.com SRV
  1 1 22029 gateway.mydomain.com.
  ```

  ```
  Host xxx
    DnsSrvName myhost.mydomain.com
  ```

### 故障排除

- 在旧版本 Warp 终端，分块 Blocks 的功能需要将 `tssh` 重命名为 `ssh`，推荐建个软链接（ 对更新友好 ）：

  ```
  sudo ln -sv $(which tssh) /usr/local/bin/ssh
  ```

  - 软链后，`ssh -V` 应输出 `tssh` 加版本号，如果不是，说明软链不成功，或者在 `PATH` 中 `openssh` 的优先级更高，你要软链到另一个地方或者调整 `PATH` 的优先级。

  - 为了让 `tssh` 搜索登录也支持分块 Blocks 功能，需要在 `~/.bash_profile` ( bash ) 或 `~/.zshrc` ( zsh ) 中建一个 `tssh` 函数：

    ```sh
    tssh() {
        if [ $# -eq 0 ]; then
            ssh FAKE_DEST_IN_WARP
        else
            ssh "$@"
        fi
    }
    ```

- 如果你在使用 Windows7 或者旧版本的 Windows10 等，遇到 `enable virtual terminal failed` 的错误。

  - 可以尝试在 [Cygwin](https://www.cygwin.com/)、[MSYS2](https://www.msys2.org/) 或 [Git Bash](https://www.atlassian.com/git/tutorials/git-bash) 内使用 `tssh`。

  - 从 `v0.1.21` 起，默认的 Windows 版本不再支持 Windows7，需要在 [Releases](https://github.com/trzsz/trzsz-ssh/releases) 中下载带有 `win7` 关键字的版本来使用。

- 如果在 `~/.ssh/config` 中配置了 `tssh` 特有的配置项后，标准 `ssh` 报错 `Bad configuration option`。

  - 请将 tssh 专有配置移动到 `ExConfigPath`，或者移动到仅供 `tssh` 使用的配置文件中。配置项直接使用 `Key Value` 格式书写。

### 联系方式

有什么问题可以发邮件给作者 <lonnywong@qq.com>，也可以提 [Issues](https://github.com/trzsz/trzsz-ssh/issues) 。欢迎加入 QQ 群：318578930。

### 赞助打赏

[❤️ 赞助 trzsz ❤️](https://github.com/trzsz)，请作者喝杯咖啡 ☕ ? 谢谢您们的支持！
