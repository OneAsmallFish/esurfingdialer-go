# esurfingdialer

> **把宿舍那根网线，变成两根。**
>
> 纯 Go 重写的天翼校园网（CCTP）认证客户端：单文件静态二进制，无 JVM、无 GUI，
> 掉线自动重连，支持多 WAN / 单线多拨 / 开机自启。

[![ci](https://github.com/OneAsmallFish/esurfingdialer-go/actions/workflows/ci.yml/badge.svg)](https://github.com/OneAsmallFish/esurfingdialer-go/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/OneAsmallFish/esurfingdialer-go?sort=semver)](https://github.com/OneAsmallFish/esurfingdialer-go/releases)
[![license](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![go](https://img.shields.io/badge/go-1.24%2B-00ADD8.svg)](go.mod)

[English](README_EN.md) · [多拨指南](docs/multi-dial.md) · [协议说明](docs/protocol.md) · [排障](docs/troubleshooting.md) · [FAQ](docs/faq.md)

---

## 它是什么

协议级实现，不是壳也不是代理：自己完成「探测门户 → 协商算法 → 取票据 → 登录 → 保活
→ 下线」的完整链路。

| 痛点 | 做法 |
|---|---|
| 官方客户端是 App，路由器上跑不了 | 纯 Go 交叉编译出 MIPS / ARM / x86 单文件 |
| 断线要手动重连 | 常驻保活 + 指数退避重认证 + procd 守护 |
| 想叠加线路 | 原生绑定接口 / 源地址 / fwmark，适配 mwan3 |
| 多实例互相干扰 | 一个配置一个进程，procd 精确重启 |

支持 AES-CBC/ECB、DESede-CBC/ECB、SM4-CBC/ECB、ZUC、ModXTEA 等算法，唯一第三方依赖是
[gmsm](https://github.com/emmansun/gmsm)（提供 SM4 与 ZUC）。

它 **不** 做破解、绕过计费或伪造他人身份——只用自己的账号把自己的设备接上网。

## 快速开始

```sh
git clone https://github.com/OneAsmallFish/esurfingdialer-go.git
cd esurfingdialer-go

go build -trimpath -ldflags "-s -w" -o esurfingdialer ./cmd/esurfingdialer
make release        # 或一次编出全部目标架构，产物在 dist/

./esurfingdialer -u 账号 -p 密码
```

成功时日志出现：

```text
[INFO] The login has been authorized.
```

之后进程自行保活，`SIGTERM` 会主动下线。参数总览见 `./esurfingdialer --help`。

## 配置

覆盖顺序：配置文件 → 环境变量 → 命令行参数。

```json
{
  "user": "账号",
  "password": "密码",
  "log_level": "info",

  "bind_interface": "eth1",
  "bind_fwmark": "0x100",

  "portal_url": "",
  "portal_auto_login": false
}
```

- `bind_interface` / `bind_fwmark` / `bind_source_ip`：多出口绑定，见多拨指南第 7 章。
- `portal_host` 留空时使用内置默认值（与原实现一致），其他学校可在此覆盖，无需重新编译。
- 代码里没有任何写死的账号或凭据。

## 路由器多拨

### 👉 [`docs/multi-dial.md`](docs/multi-dial.md)

三种架构对比（多物理口 / macvlan 单线 / netns+veth）、从零实操、可直接抄的 UCI 配置、
策略路由读法、排障矩阵与回滚步骤。[`deploy/openwrt/`](deploy/openwrt/) 里有配套的
procd 服务、hotplug 与 macvlan 多拨管理器。

最小路径：

```sh
scp esurfingdialer root@192.168.1.1:/usr/bin/
scp -r deploy/openwrt/* root@192.168.1.1:/
ssh root@192.168.1.1
vi /etc/esurfingdialer/wan.json      # 填账号
/etc/init.d/esurfingdialer enable && /etc/init.d/esurfingdialer start
logread -f -e esurfingdialer
```

## 兼容性

| 目标 | GOOS / GOARCH | 备注 |
|---|---|---|
| ARM64 软路由 | `linux/arm64` | 推荐 |
| MT7621 / MT7628 | `linux/mipsle` + `GOMIPS=softfloat` | 硬浮点会 SIGILL |
| 旧 MIPS | `linux/mips` + `GOMIPS=softfloat` | |
| ARMv7 | `linux/arm` + `GOARM=7` | |
| x86 / NAS | `linux/amd64` | |
| Windows / macOS | `windows/amd64`、`darwin/*` | 仅本机调试 |

用 `uname -m` 和 `opkg print-architecture` 确认真实架构。

## 文档

| 文档 | 内容 |
|---|---|
| [docs/multi-dial.md](docs/multi-dial.md) | 路由器多拨完全指南 |
| [docs/protocol.md](docs/protocol.md) | 认证链路、ZSM、Client-ID 生命周期、日志脱敏 |
| [docs/troubleshooting.md](docs/troubleshooting.md) | 客户端侧排障 |
| [docs/faq.md](docs/faq.md) | 常见问题 |
| [deploy/openwrt/README.md](deploy/openwrt/README.md) | OpenWrt 部署资产说明 |
| [CHANGELOG.md](CHANGELOG.md) | 版本变更 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 贡献指南 |

## 免责声明

- 仅供 **学习网络协议与个人正常接入** 使用。
- 必须使用 **自己有权使用的账号**，并遵守所在学校 / 运营商的网络规定与服务条款。
  多拨等行为可能违反相关规定，风险自负。
- 作者不对账号封禁、服务中断、设备损坏等后果负责。
- 与任何电信运营商、设备厂商或官方客户端 **没有任何关联**，也未获其授权或认可。
- 仓库不包含任何第三方客户端的分发内容。

## 许可证

[GNU General Public License v3.0](LICENSE)。

Copyright (C) 2026 esurfingdialer contributors

本程序是自由软件：你可以依据自由软件基金会发布的 GNU 通用公共许可证（第 3 版或更新版本）
条款重新发布和/或修改它。本程序基于“有用”的目的分发，但 **不提供任何担保**，
甚至不包含适销性或特定用途适用性的默示担保。详情见 LICENSE。
