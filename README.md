# esurfingdialer-go

用于天翼校园网的 Go 拨号程序，主要在 OpenWrt 路由器上使用。支持掉线重连，以及为不同出口分别运行拨号实例。

先跑通一条线路，再看[多拨教程](docs/multi-dial.md)。多拨是否可用取决于学校和账号的接入限制，程序不能保证增加会话后带宽就会叠加。

## 下载

在 [Releases](https://github.com/OneAsmallFish/esurfingdialer-go/releases/latest) 下载与你的系统、CPU 架构匹配的文件。路由器上可以先运行 `uname -m` 查看架构。

| 常见架构 | 下载文件名中的部分 |
| --- | --- |
| ARM64 / aarch64 | `linux-arm64` |
| x86_64 | `linux-amd64` |
| ARMv7 | `linux-armv7` |
| 小端 MIPS，如部分 MT7621 设备 | `linux-mipslesf` |
| 大端 MIPS | `linux-mipssf` |

MIPS 文件使用软浮点构建。请同时核对固件架构，不要只凭路由器型号选择文件。下载后可用 Release 中的 `SHA256SUMS` 校验。

## 运行

下面以 OpenWrt 为例；路由器管理地址、出口设备名需要换成自己的。

1. 把下载的文件重命名为 `esurfingdialer`，上传到路由器：

   ```sh
   scp esurfingdialer root@192.168.1.1:/tmp/esurfingdialer.new
   ssh root@192.168.1.1
   ```

2. 首次安装时，把程序放到 `/usr/bin`：

   ```sh
   chmod 755 /tmp/esurfingdialer.new
   cp /tmp/esurfingdialer.new /usr/bin/esurfingdialer
   mkdir -p /etc/esurfingdialer
   umask 077
   vi /etc/esurfingdialer/wan.local.json
   ```

   已经在用旧版本的，请先备份二进制和配置，再安排停机更新，不要直接覆盖正在使用的程序。

3. 在配置文件中填写账号、密码：

   ```json
   {
     "user": "你的拨号账号",
     "password": "你的拨号密码",
     "log_level": "info",
     "bind_interface": "eth1"
   }
   ```

   `eth1` 是示例。用下面的命令查看 WAN 实际对应的设备名：

   ```sh
   ubus call network.interface.wan status | jsonfilter -e '@.l3_device'
   ```

4. 确认 WAN 已通过 DHCP 获得地址，再启动：

   ```sh
   chmod 600 /etc/esurfingdialer/wan.local.json
   /usr/bin/esurfingdialer -c /etc/esurfingdialer/wan.local.json
   ```

   日志出现 `The login has been authorized.` 后，还要实际打开网页确认能上网。程序需要一直运行；停止时用 `Ctrl-C`。

开机自启、多物理口和单线多拨的配置见[多拨教程](docs/multi-dial.md)。不要上传包含真实账号密码的配置文件或完整调试日志。

## 可选配置

完整示例在 [examples/config.json.example](examples/config.json.example)。常用字段如下：

| 字段 | 什么时候用 |
| --- | --- |
| `bind_interface` | 指定出口设备，多出口时每个实例填自己的设备名 |
| `bind_fwmark` | 按已有策略路由选出口，值需与路由器上的规则对应 |
| `bind_source_ip` | 需要指定源地址时使用；DHCP 地址变化后要同步更新 |
| `sms_code` | 登录需要短信验证码时填写 |
| `portal_auto_login`、`portal_url` | 学校还有独立网页登录门户时使用，默认关闭 |
| `portal_user`、`portal_password`、`portal_mac` | 独立门户的账号、密码和所用出口 MAC；门户账号可能与拨号账号不同 |

不要仅因为网络慢就打开 `portal_auto_login`。先确认是否被重定向到校园门户，再填写对应参数。当前没有 `portal_host` 配置项。

配置文件、环境变量、命令行参数都可使用，优先级是命令行 > 环境变量 > 配置文件。参数列表可运行 `esurfingdialer -h` 查看。多出口绑定功能仅在 Linux 上实现。

## 自己编译

需要 Go 1.24 或更新版本。

```sh
git clone https://github.com/OneAsmallFish/esurfingdialer-go.git
cd esurfingdialer-go
go test ./...
go build -trimpath -ldflags "-s -w" -o esurfingdialer ./cmd/esurfingdialer
```

例如为 ARM64 路由器编译（在 Linux、macOS 或 Git Bash 中执行）：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o esurfingdialer ./cmd/esurfingdialer
```

Windows PowerShell 则先设置 `$env:CGO_ENABLED='0'`、`$env:GOOS='linux'`、`$env:GOARCH='arm64'`，再执行 `go build`。

维护者推送 `v*` 标签后，GitHub Actions 会运行测试、构建各平台文件并发布 Release。手动构建可用 `VERSION=v1.0.1 sh scripts/build-release.sh`，产物在 `dist/`。

## 许可

[GPL-3.0](LICENSE)。本项目不是官方客户端；请使用有权使用的账号，并遵守所在学校和运营商的接入规定。
