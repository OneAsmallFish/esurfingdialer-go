# 实际案例：红米 AC2100 双网口、双账号

这份教程根据一位同学在红米 AC2100 上部署后整理的《多网口聚合通俗教程》改写。保留了他按顺序接管两条线路的做法，便于第一次配置的同学照着核对。

为方便公开使用，管理地址、门户地址和账号改成了示例，程序和服务文件名也统一为本仓库的写法。原教程使用的设备名是 `wan`、`lan1`，网络接口名是 `wan1`、`wan2`；换固件后这些名字可能不同。已经按原教程部署的，不要仅为了与本文一致而改动现有文件名。

**这里的“聚合”是把不同连接分给两个出口，不是把单个连接的带宽翻倍。** 本文依据同学提供的部署记录，不是对所有 AC2100 固件的兼容性保证。原记录没有提供固件版本，安装 mwan3 前请核对该版本的防火墙与软件包支持情况。

## 一、先接好线，留好备份

准备两根可用校园网线、两个有权使用的校园网账号和一台电脑。学校必须允许这种接入方式及并发登录。

| 接什么 | 插在哪里 | 这个案例里的名称 |
| --- | --- | --- |
| 第一条校园网线 | 原 WAN 口 | 接口 `wan1`，设备 `wan` |
| 第二条校园网线 | LAN1 口，改作第二个 WAN | 接口 `wan2`，设备 `lan1` |
| 电脑、手机 | Wi-Fi 或保留的 LAN2、LAN3 | 内网 `lan` |

```text
校园网线 A → WAN 口  → wan1（账号 A） ─┐
                                      ├→ 路由器 → LAN / Wi-Fi → 电脑、手机
校园网线 B → LAN1 口 → wan2（账号 B） ─┘
```

“接口”是设置页面里的连接名，“设备”是它使用的网口，所以 `wan2` 使用 `lan1` 并不矛盾。

浏览器打开自己的管理地址，例如 `http://192.168.1.1`。先在“系统 → 备份与升级”下载当前配置备份。如果路由器已经有人在用，安排维护时间并在本地操作，不能保证下面的切换完全不中断已有连接。

## 二、在后台设置两个 WAN

打开“网络 → 接口”，核对这两条连接：

| 项目 | 第一条连接 | 第二条连接 |
| --- | --- | --- |
| 接口名称 | `wan1` | `wan2` |
| 协议 | DHCP 客户端 | DHCP 客户端 |
| 使用设备 | `wan` | `lan1` |
| 网关跃点 / 路由跃点 | `10` | `20` |
| 防火墙区域 | `wan` | `wan` |

先编辑内网桥 `br-lan`，把准备改作 WAN2 的 `lan1` 移出桥，保留电脑所在的 LAN 口和无线网络。然后新建 `wan2`，使用设备 `lan1`。

防火墙的 `wan` 区域应包含 `wan1`、`wan2`，开启地址伪装，并允许 `lan` 转发到 `wan`。

保存并应用后，两条 WAN 都应拿到 IPv4 地址和网关。用 SSH 核对实际设备名：

```sh
ubus call network.interface.wan1 status | jsonfilter -e '@.l3_device'
ubus call network.interface.wan2 status | jsonfilter -e '@.l3_device'
```

输出不是 `wan`、`lan1` 时，后面的配置也要跟着改。原接口叫 `wan` 的机器，不必为了本文强行改名为 `wan1`，但 mwan3、服务触发器等位置必须使用同一个网络接口名。

## 三、准备程序与账号配置

安装 `mwan3`、`luci-app-mwan3`、`curl`、`ip-full`。使用 opkg 的固件可以执行：

```sh
opkg update
opkg install mwan3 luci-app-mwan3 curl ip-full
```

其他包管理器或新版防火墙请按对应固件说明安装，不要把旧版 mwan3 的安装命令当成通用兼容方案。

原部署使用 `linux-mipslesf` 程序。到 [Releases](https://github.com/OneAsmallFish/esurfingdialer-go/releases/latest) 下载并校验对应文件，重命名为 `esurfingdialer`，按 [README 的安装步骤](../README.md#运行) 放到 `/usr/bin/esurfingdialer`。本文不依赖额外的私有资料包。

在路由器终端建立配置目录：

```sh
mkdir -p /etc/esurfingdialer
chmod 700 /etc/esurfingdialer
umask 077
vi /etc/esurfingdialer/wan1.local.json
vi /etc/esurfingdialer/wan2.local.json
```

第一份配置：

```json
{
  "user": "账号 A",
  "password": "账号 A 的密码",
  "log_level": "info",
  "bind_interface": "wan",
  "bind_fwmark": "0x100",
  "portal_auto_login": false
}
```

第二份配置：

```json
{
  "user": "账号 B",
  "password": "账号 B 的密码",
  "log_level": "info",
  "bind_interface": "lan1",
  "bind_fwmark": "0x200",
  "portal_auto_login": false
}
```

`0x100`、`0x200` 是原部署中的标记，**先在下一节核对路由规则，确认对应关系后再启动程序**。不要把它们当成所有机器固定不变的值。

### 如果学校还要求网页认证

原部署开启了独立门户登录，但不应让所有读者默认开启。确认自己接入点确实需要时，分别在两份配置中补充：

```json
{
  "portal_auto_login": true,
  "portal_url": "http://portal.example.invalid/portal/",
  "portal_user": "你的门户账号",
  "portal_password": "你的门户密码",
  "portal_mac": "02:00:00:00:00:01"
}
```

这一段是要合并进原配置的可选字段，不是完整配置。门户 URL 和 MAC 都是占位值，必须换成自己的。门户账号与拨号账号可能不同。

查询各自出口的 MAC：

```sh
cat /sys/class/net/wan/address
cat /sys/class/net/lan1/address
```

每份配置填写自己出口的 MAC。不要复制别人的账号、密码或 MAC，也不要将填好的 JSON 上传到 GitHub。配置文件权限保持 `600`。

## 四、在 mwan3 中建好三种用法

在 MultiWAN 管理页面，依次填写“接口 → 成员 → 策略 → 规则”。

### 两个接口

接口名与网络页面一致，均启用 IPv4 检测。原部署参考值：

| 项目 | 两条接口的设置 |
| --- | --- |
| 名称 | 分别为 `wan1`、`wan2` |
| 检测方式 | ping |
| 检测地址 | 至少两个在当地能稳定响应的目标 |
| 至少成功的目标数 | `1` |
| 每轮发送次数 | `1` |
| 超时 | `4` 秒 |
| 检测间隔 | `10` 秒 |
| 连续失败 / 成功次数 | `5` / `5` |

检测目标不可达会导致误判，请先验证，不要只抄地址。

### 两个成员、三个策略

| 成员 | 接口 | 成员跃点 | 权重 |
| --- | --- | ---: | ---: |
| `wan1_m1` | `wan1` | `1` | `1` |
| `wan2_m1` | `wan2` | `1` | `1` |

成员跃点相同，表示一起参与均衡；它与前面网络接口的网关跃点不是同一个设置。

| 策略 | 成员 | 用法 |
| --- | --- | --- |
| `wan1_only` | `wan1_m1` | 新连接只走第一条线 |
| `wan2_only` | `wan2_m1` | 新连接只走第二条线 |
| `balanced` | 两个成员 | 新连接分给两条线 |

这三种策略的“最后手段”在本例中选 `unreachable`。

默认 IPv4 规则放在专用规则之后，覆盖 `0.0.0.0/0`、所有协议。先使用 `wan1_only`，暂时不要切到 `balanced`。原部署默认规则关闭粘性；有需要出口保持不变的应用时，应另外配置专用规则。

确认 mwan3 启用并运行，然后核对 fwmark：

```sh
mwan3 status
ip -4 rule show
ip -4 route show table all
```

只有实际规则中的 `0x100/0x3f00` 和 `0x200/0x3f00` 分别对应两个出口时，第三节的配置才可原样使用。标记不同就调整 `bind_fwmark`，不要额外创建与 mwan3 冲突的路由表。

## 五、建立两个启动服务

如果已有服务，请先备份并核对，不要覆盖现有文件。以下是新部署用的两个示例，只启动程序，不创建网口或修改防火墙。

保存为 `/etc/init.d/esurfing-wan1`：

```sh
#!/bin/sh /etc/rc.common
USE_PROCD=1
START=95
STOP=10

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/esurfingdialer -c /etc/esurfingdialer/wan1.local.json
    procd_set_param respawn 3600 5 5
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param file /etc/esurfingdialer/wan1.local.json
    procd_close_instance
}

service_triggers() {
    procd_add_interface_trigger 'interface.*' 'wan1' /etc/init.d/esurfing-wan1 restart
}
```

保存为 `/etc/init.d/esurfing-wan2`：

```sh
#!/bin/sh /etc/rc.common
USE_PROCD=1
START=95
STOP=10

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/esurfingdialer -c /etc/esurfingdialer/wan2.local.json
    procd_set_param respawn 3600 5 5
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param file /etc/esurfingdialer/wan2.local.json
    procd_close_instance
}

service_triggers() {
    procd_add_interface_trigger 'interface.*' 'wan2' /etc/init.d/esurfing-wan2 restart
}
```

设置权限：

```sh
chmod 755 /etc/init.d/esurfing-wan1 /etc/init.d/esurfing-wan2
chmod 600 /etc/esurfingdialer/wan1.local.json /etc/esurfingdialer/wan2.local.json
```

## 六、先接管第二条线，再迁移第一条线

这是原教程中值得保留的顺序：先验证备用出口，避免一下停掉两条认证。

如果第二条线也已有旧认证器，先只停它的对应任务，再启动 Go 实例。不能让两个认证器同时管理同一个出口：

```sh
/etc/init.d/esurfing-wan2 start
logread -e esurfing
```

支持 `mwan3 use` 的版本可以这样单独检测 WAN2：

```sh
mwan3 use wan2 curl --connect-timeout 3 --max-time 8 -sS -o /dev/null \
  -w '%{http_code}\n' http://connect.rom.miui.com/generate_204
```

没有这个子命令时，可以用 `curl --interface lan1 ...` 做出口检查，但还要核对实际路由并用 LAN 客户端验证，不能只看一条命令的结果。

返回 `204` 表示这次目标探测成功，不代表所有应用或长期稳定性都已验证。确认 WAN2 在 mwan3 中在线后，将默认策略暂时改为 `wan2_only`，用电脑打开新网页，确认第二条线能承担日常访问。

然后只停第一条线的旧认证任务，启动 WAN1：

```sh
/etc/init.d/esurfing-wan1 start
mwan3 use wan1 curl --connect-timeout 3 --max-time 8 -sS -o /dev/null \
  -w '%{http_code}\n' http://connect.rom.miui.com/generate_204
```

两个出口分别验证成功后，才启用开机启动并停用旧认证器的相应启动任务，保留旧配置备份：

```sh
/etc/init.d/esurfing-wan1 enable
/etc/init.d/esurfing-wan2 enable
```

第一条接管失败时，先恢复它的旧认证，不要为了继续教程而停掉已能用的第二条线。

## 七、最后切到 balanced

将默认规则从 `wan2_only` 改为 `balanced`。最终核对：

| 检查位置 | 应看到什么 |
| --- | --- |
| 网络接口 | 两条 WAN 都有 IPv4 地址 |
| mwan3 状态 | 两条线都在线 |
| 默认规则 | 使用 `balanced` |
| 成员 | 跃点都为 `1`，权重都为 `1` |
| 启动项 | 两个 Go 认证服务已启用 |
| 出口检查 | 两路分别可访问探测目标和日常网页 |

用有线电脑和支持多连接的下载或测速工具，同时观察两个 WAN 的实时流量。两条线都出现持续流量、旧线路未被新线路挤掉，再观察一段时间，才算完成这次配置。不要把单连接没有翻倍当成失败。

本地有人值守时再进行一次重启，确认 DHCP、拨号和 mwan3 都能恢复。

## 临时只用一条线与回退

在默认规则中切到 `wan1_only` 或 `wan2_only`；新连接按新策略建立，原有连接未必立即迁移。

需要重新认证时，先保证另一条可用，再只重启对应服务：

```sh
/etc/init.d/esurfing-wan1 restart
# 或只重启第二条：
/etc/init.d/esurfing-wan2 restart
```

出现反复掉线时，先退回已验证的单线策略，停止有问题的新增服务，必要时恢复备份。不要通过反复重启或继续增加会话来试探学校限制。

更一般的单线与多物理口设置见[多拨教程](multi-dial.md)；固件兼容性与 mwan3 细节见 [OpenWrt 文档](https://openwrt.org/docs/guide-user/network/wan/multiwan/mwan3)。
