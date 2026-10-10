# OpenWrt 多拨教程

先按 [README](../README.md) 跑通单拨。下面分别介绍两根网线的双拨和一根网线的 macvlan 双拨，不要同时照着两套配置操作。

所有地址、MAC、设备名和账号都只是示例，需要换成自己的。我们的双物理线方案曾在现场完成认证和出网验证；单线方案在不同固件、不同接入点上需要重新验证。不要把这些例子当成适用于所有学校的安装脚本。

## 1. 动手前

- 确认学校允许使用路由器、账号允许相应的并发登录。多拨不会绕过这些限制。
- 用网线连路由器 LAN，在本地操作。修改 WAN、桥接或防火墙可能中断远程连接。
- 备份配置，并记下当前哪个口是 LAN、哪个口是 WAN。
- 一次只加一路。新线路认证成功以后，还要检查新旧线路是否都能上网。

备份命令：

```sh
backup="/root/esurfing-backup-$(date +%Y%m%d-%H%M%S)"
mkdir -m 700 "$backup"
cp -p /etc/config/network /etc/config/firewall "$backup/"
[ ! -f /etc/config/mwan3 ] || cp -p /etc/config/mwan3 "$backup/"
[ ! -d /etc/esurfingdialer ] || cp -pR /etc/esurfingdialer "$backup/"
echo "$backup"
```

这些备份可能含账号信息，只留在自己的设备上。若是远程维护，请先保留一条独立的管理通道。

查看接口名：

```sh
ubus call network.interface.wan status
ip -4 address
ip -4 route
```

`wan` 是 OpenWrt 网络接口名，`eth1`、`lan1` 是设备名，二者不要混填。拨号器的 `bind_interface` 填设备名。

## 2. 安装需要的组件

在 LuCI 的“系统 → 软件包”中安装：

- `mwan3`、`luci-app-mwan3`：多出口选路和管理界面；
- `ip-full`、`curl`：查看路由和检测出口；
- `kmod-macvlan`：只有单线方案需要。

使用 opkg 的固件也可执行：

```sh
opkg update
opkg install mwan3 luci-app-mwan3 ip-full curl
# 只有单线方案需要：
opkg install kmod-macvlan
```

使用 apk 的固件应改用对应的包管理器。mwan3 的 iptables 兼容版和原生 nftables 版配置细节可能不同，请按固件选择；不要把软件包和路由标记从其他机器整套复制过来。下面的路由标记例子以传统 mwan3 配置为例。

## 3. 两根网线：两个物理出口

示例接法：

```text
校园网线 A → 路由器 WAN（eth1）→ wan
校园网线 B → 路由器 LAN1（lan1）→ wan2
电脑        → 路由器另一个 LAN 口
```

### 配置 WAN2

在 LuCI 中操作：

1. “网络 → 设备”，把准备作为 WAN2 的端口从 `br-lan` 中移出。不要移除电脑当前连接的管理端口。
2. “网络 → 接口”，新建 `wan2`，协议选 DHCP 客户端，设备选择该端口。
3. 将 `wan2` 加入现有 WAN 防火墙区域，保留该区域的地址伪装和 LAN → WAN 转发。
4. 在高级设置中，为 `wan`、`wan2` 分别设不同的路由跃点，例如 10、20。
5. 保存并应用，确认两条线路都有 IPv4 地址和默认路由。

对应的网络配置大致如下。只用于对照字段，**不要覆盖整个 `/etc/config/network`**：

```uci
config interface 'wan'
    option device 'eth1'
    option proto 'dhcp'
    option metric '10'

config interface 'wan2'
    option device 'lan1'
    option proto 'dhcp'
    option metric '20'
```

本例让 mwan3 管理每个出口的策略路由表，不再另外填写 `ip4table '1'`、`ip4table '2'`。

### 给每条线路一份拨号配置

建立 `/etc/esurfingdialer/wan.local.json` 和 `/etc/esurfingdialer/wan2.local.json`。每个文件都填有权使用的账号密码，分别设置：

```json
{
  "user": "你的拨号账号",
  "password": "你的拨号密码",
  "log_level": "info",
  "bind_interface": "eth1"
}
```

第二个文件将 `bind_interface` 改为 `lan1`。使用不同账号时，账号密码也分别填写。

先在两个 SSH 窗口中前台运行，暂时不要设开机自启：

```sh
# 窗口一
/usr/bin/esurfingdialer -c /etc/esurfingdialer/wan.local.json
```

```sh
# 窗口二：第一条稳定后再启动
/usr/bin/esurfingdialer -c /etc/esurfingdialer/wan2.local.json
```

## 4. 配置 mwan3

在 LuCI 的“网络 → 负载均衡 / MultiWAN 管理”中设置：

1. 添加并启用 `wan`、`wan2` 两个接口。名字要与“网络 → 接口”一致。
2. 每个接口填至少两个在当地能稳定响应 ping 的检测地址，避免单一检测目标故障造成误判。
3. 建立两个成员：各自关联一条接口，成员跃点相同、权重均为 1。
4. 建立 `balanced` 策略，加入这两个成员。
5. 将默认 IPv4 规则指向 `balanced`。游戏、SSH 或其他需要出口稳定的流量，可以单独设规则固定到一条线路。

注意网络接口的路由跃点应不同，而 balanced 中两个成员的跃点应相同。下面只展示成员和策略字段，接口检测与规则仍需在 LuCI 中完成：

```uci
config member 'wan_equal'
    option interface 'wan'
    option metric '1'
    option weight '1'

config member 'wan2_equal'
    option interface 'wan2'
    option metric '1'
    option weight '1'

config policy 'balanced'
    list use_member 'wan_equal'
    list use_member 'wan2_equal'
```

### 检查拨号器是否走对出口

```sh
mwan3 status
ip -4 rule show
ip -4 route show table all
```

如果需要通过 fwmark 固定认证请求的出口，先从 `ip rule` 中找到对应的 mark 和路由表，再检查路由表里的默认路由属于哪个设备。

例如实际输出是：

```text
2001: from all fwmark 0x100/0x3f00 lookup 1
2002: from all fwmark 0x200/0x3f00 lookup 2
```

且表 1、表 2 分别指向 `eth1`、`lan1`，才在两份 JSON 中分别增加 `"bind_fwmark": "0x100"`、`"bind_fwmark": "0x200"`，然后逐个重启拨号实例。接口顺序或掩码不同时，标记也可能不同，不能直接照抄。

```sh
ip -4 route show table 1
ip -4 route show table 2
ip -4 route get 1.1.1.1 mark 0x100
ip -4 route get 1.1.1.1 mark 0x200
```

若使用 `bind_source_ip`，DHCP 续租换地址后必须同步修改；一般先用设备绑定和已核对的 fwmark，不要为了“配得全”填写一个会过期的源地址。

## 5. 一根网线：macvlan 双拨

适用前提：同一个上联端口允许多个 MAC 获得独立 DHCP 地址，账号也允许并发登录。不能满足前提时，不要反复创建虚拟接口尝试。

保留现有 `wan`，只增加一个虚拟设备。下面是需要添加的配置段；在本地备份后编辑 `/etc/config/network`，不要替换原有 LAN 配置：

```uci
config device 'esurfing_mvw1'
    option name 'mvw1'
    option type 'macvlan'
    option ifname 'eth1'
    option mode 'bridge'
    option macaddr '02:00:00:00:10:01'

config interface 'vwan1'
    option device 'mvw1'
    option proto 'dhcp'
    option metric '20'
```

`eth1` 换成实际物理 WAN，MAC 使用自己选定的、不与现有设备重复的本地管理地址。配置字段以固件的 netifd 支持情况为准。

在 LuCI 中将 `vwan1` 加入 WAN 防火墙区域，保存并应用后检查：

```sh
ubus call network.interface.vwan1 status
ip -4 address show dev mvw1
```

确认虚拟设备获得地址，再建立 `/etc/esurfingdialer/vwan1.local.json`，将 `bind_interface` 填为 `mvw1`，运行第二个拨号实例：

```sh
/usr/bin/esurfingdialer -c /etc/esurfingdialer/vwan1.local.json
```

接下来按第 4 节配置 mwan3，只是把 `wan2` 换成 `vwan1`。需要 fwmark 时，同样先核对实际路由表。

如果学校还有独立门户，每个新 MAC 可能都需要完成门户登录。在对应配置中填写自己的 `portal_url`、`portal_user`、`portal_password` 和出口 MAC，再开启 `portal_auto_login`；不要盲目复制另一条线路的门户 MAC。

这里没有提供 netns/veth 的整套部署脚本。它需要另外处理命名空间中的 DHCP、路由和 NAT，不应与本节的直接 macvlan 配置混用。

## 6. 验证两条线路

双物理口：

```sh
curl --interface eth1 -m 10 -sS -o /dev/null -w '%{http_code}\n' http://connect.rom.miui.com/generate_204
curl --interface lan1 -m 10 -sS -o /dev/null -w '%{http_code}\n' http://connect.rom.miui.com/generate_204
```

单线双拨将第二条命令的 `lan1` 改为 `mvw1`。正常应返回 204；302 通常需要检查门户，超时则先排查出口、DHCP 和路由。检测站点不可用时换一个能访问的目标，不要只凭单次探测下结论。

还要从 LAN 上实际浏览网页，观察一段时间，确认两条线路没有交替失效。我们曾遇到新增身份认证成功、但原有线路不能转发的情况，所以“日志显示登录成功”不能代替这个检查。

测速请使用多连接测试。mwan3 分配的是连接，不会把一个普通 TCP 连接自动拆到两条线路上；单连接速度没有翻倍不等于双拨失败。

## 7. 确认稳定后再设开机自启

下面是单个实例的 procd 服务示例，保存为 `/etc/init.d/esurfing-wan`。它只启动拨号器，不创建接口，也不修改路由或防火墙：

```sh
#!/bin/sh /etc/rc.common
USE_PROCD=1
START=95
STOP=10

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/esurfingdialer -c /etc/esurfingdialer/wan.local.json
    procd_set_param respawn 3600 5 5
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param file /etc/esurfingdialer/wan.local.json
    procd_close_instance
}

service_triggers() {
    procd_add_interface_trigger 'interface.*' 'wan' /etc/init.d/esurfing-wan restart
}
```

先停止之前前台运行的同一实例，再启用服务：

```sh
chmod 755 /etc/init.d/esurfing-wan
/etc/init.d/esurfing-wan enable
/etc/init.d/esurfing-wan start
logread -e esurfing
```

第二个实例另建 `/etc/init.d/esurfing-wan2`：同时替换配置路径、触发器里的网络接口名和服务路径。单线方案则用 `vwan1` 和相应配置文件。不要让前台进程和 procd 同时拨同一个出口。

DHCP 更新后，如果填写过 `bind_source_ip`，需要先更新配置再重启该实例。设置完后，在本地进行一次重启检查，确认接口、拨号服务和 mwan3 都能恢复。

## 8. 出问题时先退回单拨

- 没有 DHCP 地址：先检查物理口或 macvlan，不要先改拨号参数。
- 报 `63022` 或在线用户数量限制：停止新增实例，等旧会话释放，核对账号的并发许可。
- 第二路上线后第一路断网：停止第二路，恢复单拨；不要继续加第三、第四路。
- 双拨后游戏或管理连接不稳定：先将相关流量固定到一条可用线路，再排查选路。

回退时，在本地按相反顺序操作：先停止并禁用新增拨号服务，再将 mwan3 默认规则切回主 WAN，最后禁用新增网络接口。需要完全恢复时，用第 1 节备份逐项恢复 network、firewall、mwan3；不要在没有备用连接的情况下远程重载整套网络。

更多固件相关配置可查阅 [OpenWrt 的 mwan3 文档](https://openwrt.org/docs/guide-user/network/wan/multiwan/mwan3)。
