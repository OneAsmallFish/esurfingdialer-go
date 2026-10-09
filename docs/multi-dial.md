# 路由器多拨完全指南

> 本文以我们自己在用的一台 OpenWrt 路由器为例，所有地址、MAC、账号均已替换为示例值。
> 把它当成一份“照着做就能通”的工程记录，而不是营销文案。
>
> 阅读前提：你已经能用 `esurfingdialer` 在路由器上完成 **单拨**。如果还没有，
> 先跳到 [第 4 章](#4-第一步让单拨先稳定跑起来)。

---

## 0. 先说结论

| 方案 | 隔离性 | 复杂度 | 稳定性 | 我们的选择 |
|---|---|---|---|---|
| **A. 多物理口多拨** | 天然（每个口一条线） | ★☆☆☆☆ | ★★★★★ | ✅ 生产在用 |
| **B. macvlan 单线多拨** | 中（共享物理口） | ★★★☆☆ | ★★★★☆ | ✅ 单线场景首选 |
| **C. netns + veth 隔离** | 强 | ★★★★★ | ★★★☆☆ | ❌ 已放弃 |

以及一条比架构更重要的结论：

> **能认证成功 ≠ 能稳定转发。**
> 我们在实测中见过：第三个身份顺利拿到 ZSM、ticket、KEEP，然后 **两条物理线路先后
> 被上游黑洞**，全程没有任何错误码。多拨的瓶颈通常不在你的路由器，而在上游的并发策略。
> 所以本文后半部分的重点是 **怎么观测、怎么回滚**，不是怎么把会话数堆到最大。

---

## 1. 多拨到底是什么

### 1.1 校园网认证模型

先理解一件事：CCTP 认证里，**身份不只是账号**。一次登录请求里带了这些东西：

```text
user-agent, client-id, ticket, local-time, userid, passwd, verify
```

而在此之前，取 ticket 的请求里带了：

```text
client-id, local-time, host-name, ipv4, mac, ostag, gwip
```

其中 `ipv4`、`mac`、`gwip` 来自你当前的 DHCP 租约和接口。也就是说：

**上游看到的“一个在线用户” = 账号 + Client-ID + MAC + IP 的组合。**

这直接决定了两件事：

1. 想让上游把你当成两个客户端，你必须 **有两个不同的 MAC / IP**；
2. 仅仅复制一份配置文件、绑同一个接口跑两个进程，是 **没有意义的**——它们会互相
   顶掉会话，而且因为 Client-ID 不同，还会触发身份错乱。

### 1.2 和 PPPoE 多拨的区别

| | PPPoE 多拨 | 校园网 CCTP 多拨 |
|---|---|---|
| 身份载体 | PPPoE 会话 ID + 账号 | MAC + IP + Client-ID |
| 每个会话的接口 | 天然独立（`pppoe-wan`） | **需要你自己造**（macvlan / 物理口） |
| 认证时机 | 拨号时 | 拿到 DHCP 租约 **之后** |
| 失败表现 | 拨号失败 | 认证成功但流量黑洞（更阴险） |

结论：校园网多拨比 PPPoE 多拨多了一整个“造接口 + 分路由”的前置工作，
而且失败模式更隐蔽。

### 1.3 上游必须满足的两个前提

在动手之前，请先用一台普通电脑 / 手机验证：

- **前提 1：同一账号允许多会话。** 用手机登录、再用电脑登录，看手机是否被踢。
  如果第二个会话直接把第一个顶下线，多拨在你的接入点上不成立。
- **前提 2：每个 MAC 能拿到独立 IP。** 换个 MAC 插同一根线，DHCP 要能给出 **不同** 的地址。
  如果所有 MAC 拿到同一个 IP，说明上游按端口 / 账号做绑定，macvlan 方案无效。

两条里任何一条不满足，后面的工作都不用做。

---

## 2. 架构选型

### 2.1 方案 A：多物理口多拨

```text
        ┌──────────────┐
  eth1 ─┤ wan   table 1├─ 0x100 ─┐
        │              │         │
  lan1 ─┤ wan2  table 2├─ 0x200 ─┤
        │              │         ├─ mwan3 balanced
        │   OpenWrt    │         │
  lan2 ─┤   br-lan     │         │
        └──────┬───────┘         │
               │                 │
           你的电脑 ─────────────┘
```

**适用**：路由器有多个 WAN 口，且每个口都能独立上联（不同交换机端口 / 不同线路）。
**优点**：物理隔离，一条线抖动不影响另一条；上游看到的是两个真实端口，最不容易被策略针对。
**缺点**：需要多个可用上联口。

### 2.2 方案 B：macvlan 单线多拨（推荐给单线场景）

```text
                  ┌──────────────────────────────┐
                  │           OpenWrt            │
   一根网线 ──────┤ eth1 (父接口)                │
                  │   ├── mvw1   macvlan  table 3│──0x300──┐
                  │   └── mvlw1  macvlan  table 4│──0x400──┤
                  │                              │         ├─ mwan3
                  │ br-lan ── 你的电脑           │         │
                  └──────────────────────────────┘         │
                                                           │
       上游看到 3 个不同 MAC → 3 个 DHCP 租约 → 3 个身份 ───┘
```

macvlan 让一个物理网卡持有多个 MAC 地址，每个 MAC 独立发 DHCP、独立认证。
**不需要 netns，不需要 veth，不需要 NAT**，因为所有会话都在根命名空间里，
netifd 直接给每个会话配一张路由表。

**适用**：只有一根网线可用。
**关键约束**：macvlan 的每个子接口都要能拿到独立 DHCP 租约（前提 2）。

### 2.3 方案 C：netns + veth（我们为什么放弃）

我们早期真的这么做：给每个虚拟会话开一个 network namespace，dialer 跑在 namespace 里，
用 veth 对连回根命名空间，再配 NAT 和转发规则。

它能做到：会话之间彻底隔离，连路由表都不会串。

它的代价：

- 每个会话要维护 **veth 对 + namespace 内地址 + NAT + 转发 + LAN 回程路由**；
- DHCP 客户端跑在 namespace 里，而 netifd 在根命名空间，两边状态要手动同步；
- 任何一个环节漏了，症状都是“认证成功但上不了网”，排查要跨两个命名空间；
- 清理时要按 owner 标记逐个杀进程、删 namespace，容易留下僵尸状态。

换成 macvlan 直拨后，同样的功能只需要：**一条 `ip link` + 一个 UCI 接口 + 一条 `ip rule`**。
所以方案 C 在本项目里已经废弃。

> 什么情况下还值得用 netns？当你要给不同会话 **完全不同的 DNS、防火墙策略或路由策略**，
> 并且愿意为此付出上面的维护成本时。

---

## 3. 前置检查

在路由器上执行，确认环境齐备：

```sh
# 架构（决定你编译哪个 GOARCH）
uname -m
opkg print-architecture

# OpenWrt 版本
head -3 /etc/openwrt_release

# 必需组件
opkg list-installed | grep -E 'mwan3|kmod-macvlan|ip-full|curl'
```

缺什么装什么：

```sh
opkg update
opkg install mwan3 ip-full curl kmod-macvlan
```

> `ip-full` 很重要：busybox 的 `ip` 不支持完整的 `ip rule` / 策略路由语法。

确认你的 WAN 口物理名：

```sh
ubus call network.interface.wan status | jsonfilter -e '@.l3_device'
# 例如 eth1
```

后面所有例子里的 `eth1` / `lan1` 都要换成你自己的接口名。

---

## 4. 第一步：让单拨先稳定跑起来

**不要跳过这一步。** 多拨的所有问题都会被单拨阶段的错误掩盖。

### 4.1 编译

在你的电脑上（不是路由器上）：

```sh
cd esurfingdialer-go

# ARM64 软路由
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o esurfingdialer ./cmd/esurfingdialer

# MT7621 / MT7628
GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o esurfingdialer ./cmd/esurfingdialer
```

也可以用 `make release` 一次编出全部架构，产物在 `dist/`。

### 4.2 部署

```sh
scp esurfingdialer root@192.168.1.1:/usr/bin/
ssh root@192.168.1.1
chmod +x /usr/bin/esurfingdialer
mkdir -p /etc/esurfingdialer
```

### 4.3 配置

```sh
cp /path/to/deploy/openwrt/etc/esurfingdialer/wan.json.example /etc/esurfingdialer/wan.json
chmod 600 /etc/esurfingdialer/wan.json
vi /etc/esurfingdialer/wan.json
```

填 `user` / `password`，先 **不要** 设 `bind_interface` 和 `bind_fwmark`。

### 4.4 前台跑通

```sh
/usr/bin/esurfingdialer -c /etc/esurfingdialer/wan.json --log-level debug
```

盯住这行：

```text
[INFO] The login has been authorized.
```

然后验证数据面：

```sh
curl -s -o /dev/null -w '%{http_code}\n' http://connect.rom.miui.com/generate_204
# 期望 204
```

### 4.5 判定标准

单拨阶段必须全部满足才继续：

- [ ] 日志出现 `The login has been authorized.`
- [ ] captive 探测返回 `204`（不是 302、不是超时）
- [ ] 日志里出现 `Keep Url` / `Term Url` / `Keep Retry`
- [ ] 挂机 10 分钟，日志里周期性出现 `Send Keep Packet`，没有反复重认证
- [ ] `Ctrl-C` 退出后，日志里没有 `term error`（或只有超时告警）

---

## 5. 第二步：多物理口多拨

假设：`eth1` 是 WAN1，`lan1` 是 WAN2。

### 5.1 给每个口一张路由表

```uci
# /etc/config/network
config interface 'wan'
	option device 'eth1'
	option proto 'dhcp'
	option ip4table '1'
	option defaultroute '1'
	option peerdns '0'

config interface 'wan2'
	option device 'lan1'
	option proto 'dhcp'
	option ip4table '2'
	option defaultroute '1'
	option peerdns '0'
```

执行 `uci commit network && /etc/init.d/network reload`。

```sh
# 验证：两张表各有一条 default
ip route show table 1 | head -2
ip route show table 2 | head -2
```

> `ip4table` 是 OpenWrt 的关键字：让 netifd 把这个接口的默认路由装进指定表，
> 而不是全部塞进 main 表。这是策略路由的地基。

### 5.2 给每个实例一个服务

```uci
# /etc/config/esurfingdialer
config instance 'wan'
	option enabled '1'
	option config_file '/etc/esurfingdialer/wan.json'
	option interface 'wan'
	option bind_interface 'eth1'
	option bind_fwmark '0x100'
	option log_level 'info'

config instance 'wan2'
	option enabled '1'
	option config_file '/etc/esurfingdialer/wan2.json'
	option interface 'wan2'
	option bind_interface 'lan1'
	option bind_fwmark '0x200'
	option log_level 'info'
```

两份 json 的 `user` / `password` 各自填好。

```sh
cp deploy/openwrt/etc/init.d/esurfingdialer /etc/init.d/
chmod +x /etc/init.d/esurfingdialer
/etc/init.d/esurfingdialer enable
/etc/init.d/esurfingdialer start

# 看两个实例是否都起来了
ubus call service list '{"name":"esurfingdialer"}' | jsonfilter -e '@.esurfingdialer.instances.*.running'
```

### 5.3 源地址规则

```uci
# /etc/config/esurfingdialer（续）
config source_rule 'wan'
	option device 'eth1'
	option table '1'
	option priority '801'

config source_rule 'wan2'
	option device 'lan1'
	option table '2'
	option priority '802'
```

配合 `deploy/openwrt/etc/hotplug.d/iface/95-esurfing-source-rules`，
每次 DHCP 续约都会把规则重新钉到当前地址上。

```sh
ip rule show
# 801:	from 10.1.1.5 lookup 1
# 802:	from 10.2.2.9 lookup 2
```

### 5.4 mwan3 负载均衡

```sh
cp deploy/openwrt/etc/config/mwan3.example /etc/config/mwan3
vi /etc/config/mwan3        # 把 track_ip 换成你能 ping 通的地址
/etc/init.d/mwan3 restart
mwan3 status
```

关键点：`mwan3` **只负责选路，不负责拨号**。任何一个 member 在加入之前，
都必须先用 `esurfingdialer-nettest.sh` 证明它能通：

```sh
esurfingdialer-nettest.sh eth1
esurfingdialer-nettest.sh lan1
```

### 5.5 验证

```sh
# 每条路径单独出网
curl --interface eth1 -s -o /dev/null -w 'eth1 %{http_code}\n' http://connect.rom.miui.com/generate_204
curl --interface lan1 -s -o /dev/null -w 'lan1 %{http_code}\n' http://connect.rom.miui.com/generate_204

# 按 fwmark 选路（模拟 dialer 的走法）
ip route get 223.5.5.5 mark 0x100
ip route get 223.5.5.5 mark 0x200
```

两条都要出 `204`，且 `ip route get` 显示的是各自的表。

---

## 6. 第三步：macvlan 单线多拨

### 6.1 手工验证可行性（必做）

在写任何脚本之前，先手工造一个虚拟接口，确认上游会给它独立 IP：

```sh
# 造一个 macvlan，MAC 用本地管理地址（第一字节第二位为 1，例如 02/06/0A/0E）
ip link add link eth1 name mvw-test type macvlan mode bridge
ip link set mvw-test address 02:00:00:00:01:01
ip link set mvw-test up

# 手工要一个 DHCP 地址
udhcpc -i mvw-test -q -n -t 10
ip -4 addr show dev mvw-test
```

**判定**：拿到的 IP 必须与 `eth1` 上的 **不同**。
如果是同一个 IP，或者根本拿不到，说明上游按端口 / 账号绑定，macvlan 方案在你的接入点
不成立，请回到方案 A。

验证完清理掉：

```sh
ip link del mvw-test
```

### 6.2 让 netifd 管理虚拟接口

我们不手工跑 `udhcpc`——交给 netifd，它会在租约变化时自动更新路由表：

```uci
# /etc/config/network
config interface 'vwan1'
	option device 'mvw1'
	option proto 'dhcp'
	option ip4table '3'
	option defaultroute '1'
	option peerdns '0'
	option ipaddr ''        # 租约 hint，由脚本写回
```

> 接口名（`vwan1`）和 macvlan 设备名（`mvw1`）是两个东西。
> 前者是 netifd 的逻辑接口，后者是内核里的网卡。脚本会按配置把两者关联起来。

### 6.3 多拨会话配置

```uci
# /etc/config/esurfing-multidial
config session 'vwan1'
	option enabled '1'
	option parent 'eth1'
	option device 'mvw1'
	option mac '02:00:00:00:01:01'
	option table '3'
	option fwmark '0x300'
	option priority '803'
	option base_config '/etc/esurfingdialer/wan.json'
```

字段含义：

| 选项 | 作用 |
|---|---|
| `parent` | macvlan 挂在哪张物理网卡上 |
| `device` | macvlan 网卡名 |
| `mac` | 这个会话的身份 MAC，**必须全局唯一** |
| `table` | netifd 装默认路由的路由表号，要和 `network` 里 `ip4table` 一致 |
| `fwmark` | dialer 给 socket 打的 `SO_MARK`，和 mwan3 的 mmx_mask 对齐 |
| `priority` | `ip rule` 优先级；数值越小越先匹配，要避开 mwan3 已占用的区间 |
| `base_config` | 复用哪份账号配置；脚本只覆盖 `bind_fwmark` / `bind_source_ip` |

### 6.4 启动

```sh
cp deploy/openwrt/usr/libexec/esurfing-macvlan-multidial /usr/libexec/
chmod +x /usr/libexec/esurfing-macvlan-multidial
cp deploy/openwrt/etc/config/esurfing-multidial /etc/config/

/usr/libexec/esurfing-macvlan-multidial start-one vwan1
/usr/libexec/esurfing-macvlan-multidial status vwan1
```

期望输出：

```text
vwan1: created macvlan mvw1 on eth1 mac=02:00:00:00:01:01
vwan1: lease 10.3.3.7 on mvw1
vwan1: source rule ready 10.3.3.7 -> table 3 (priority 803)
vwan1: dialer started pid=12345
vwan1: ready ip=10.3.3.7 device=mvw1 captive=204

vwan1 device=mvw1 link=up ip=10.3.3.7 dialer_alive=yes table_default=yes
```

五个字段全绿才算成功。`link=up` 但 `dialer_alive=no` 说明认证挂了，
去看 `/tmp/vwan1.dialer.log`。

### 6.5 为什么要有租约 hint

macvlan 会话重启时，如果 DHCP 给了新 IP，Portal 会把它算成一个 **全新的在线设备**，
而不是“老设备重连”。会话数一多，很容易撞上在线数上限。

脚本的做法是把上次成功租约写进 UCI：

```sh
uci get network.vwan1.ipaddr
# 10.3.3.7
```

这只是 **请求**，DHCP 服务器仍然是权威。但把 MAC / IP 这一对稳定下来，能明显减少
`online user limit` 类型的失败。这是我们踩了坑之后才加上的。

---

## 7. 第四步：策略路由（最容易翻车的地方）

### 7.1 问题

多拨之后你有 2~4 张默认路由表，但 **内核只看 main 表**。所以：

- `curl` 默认走 main 表 → 永远只从一条线出去；
- dialer 的认证流量如果也走 main 表 → 多个实例的流量从同一条线出去 → 上游看到的
  源 IP 和会话对不上 → 认证成功但流量黑洞。

### 7.2 两种绑定方式，选 fwmark

| | 源地址绑定 `--bind-source-ip` | fwmark 绑定 `--bind-fwmark` |
|---|---|---|
| 原理 | socket 绑本地源地址 | socket 打 `SO_MARK`，由 `ip rule` 选表 |
| DHCP 换 IP 后 | **静默失效** | 不受影响 |
| 需要额外规则 | `from <ip> lookup <table>` | `fwmark <mark> lookup <table>` |
| 结论 | 作为补充 | ✅ **主力** |

**根因**：源地址绑定把 IP 写死在 socket 上。续约换 IP 的那一刻，原来那条
`from <旧IP> lookup 3` 规则再也匹配不到任何包，流量悄悄回落到 main 表——
不报错、不掉线，只是慢 / 串线。这就是为什么我们最终以 fwmark 为主。

### 7.3 完整的规则链

```sh
ip rule show
```

```text
0:	from all lookup local
801:	from 10.1.1.5 lookup 1        # wan   源地址规则（hotplug 维护）
802:	from 10.2.2.9 lookup 2        # wan2  源地址规则
803:	from 10.3.3.7 lookup 3        # vwan1 源地址规则
1001:	from all fwmark 0x100/0x3f00 lookup 1    # mwan3: wan
1002:	from all fwmark 0x200/0x3f00 lookup 2    # mwan3: wan2
1008:	from all fwmark 0x300/0x3f00 lookup 3    # mwan3: vwan1
32766:	from all lookup main
32767:	from all lookup default
```

读法（**从上往下，第一条匹配即生效**）：

1. `local` 表最高优先，本地地址永远走本地；
2. 801–804 把“源地址 = 某个 WAN 的地址”的包钉到对应表——这是给 **没打 mark 的流量**
   兜底的（例如路由器自己发的、走了 main 表的程序）；
3. 1001+ 是 mwan3 给打过 mark 的流量选表；
4. 剩下的才走 main / default。

> **顺序很重要。** 如果源地址规则排到 mwan3 的 fwmark 规则后面，就会出现
> “mwan3 选好了表，又被源地址规则改回去”的诡异现象。我们把源地址规则放在 8xx，
> 把 mwan3 留在 1xxx。

### 7.4 手工验证

```sh
# 按 mark 查表
ip route get 223.5.5.5 mark 0x300
# 期望：dev mvw1 ... table 3

# 按源地址查表
ip route get 223.5.5.5 from 10.3.3.7
# 期望：dev mvw1 ... table 3
```

两个结果必须一致。不一致说明规则顺序或者表号配错了。

### 7.5 反 martian / rp_filter

多个接口持有同一网段的地址时，内核会因为“包从错误的接口进来”丢掉它。
如果出现“能 ping 通网关但上不了公网”，检查：

```sh
sysctl net.ipv4.conf.all.rp_filter
sysctl net.ipv4.conf.mvw1.rp_filter
```

按需放宽（放在 `/etc/sysctl.d/` 或 `/etc/firewall.user` 里）：

```sh
sysctl -w net.ipv4.conf.all.rp_filter=2
sysctl -w net.ipv4.conf.mvw1.rp_filter=2
```

> 放宽 rp_filter 有安全含义，只在你能控制上联环境时这么做。

---

## 8. 第五步：服务化与自愈

### 8.1 procd 守护

```sh
cp deploy/openwrt/etc/init.d/esurfingdialer /etc/init.d/
chmod +x /etc/init.d/esurfingdialer
/etc/init.d/esurfingdialer enable
/etc/init.d/esurfingdialer start
```

脚本为每个 `config instance` 起一个 procd 实例，并带上：

- `GOMAXPROCS=1`：这个进程是 I/O 密集、CPU 空闲的，限制 P 的数量能显著减少
  调度器线程和内存占用；
- `respawn`：挂掉自动拉起；
- `term_timeout 30`：给主动下线留足时间（TERM 请求本身可能超时，见战场笔记）；
- `procd_set_param file`：配置文件变更时触发 reload。

### 8.2 接口事件

```sh
cp deploy/openwrt/etc/hotplug.d/iface/99-esurfingdialer /etc/hotplug.d/iface/
cp deploy/openwrt/etc/hotplug.d/iface/95-esurfing-source-rules /etc/hotplug.d/iface/
chmod +x /etc/hotplug.d/iface/99-esurfingdialer /etc/hotplug.d/iface/95-esurfing-source-rules
```

- `99-esurfingdialer`：接口 up / update 时重启对应实例（DHCP 换了 wlanuserip 必须重新认证）；
- `95-esurfing-source-rules`：把源地址规则重新钉到新租约上。

### 8.3 开机顺序

macvlan 会话 **必须等父线路稳定之后再开**。原因：父线路自己还在认证时，
Portal 会把你的虚拟会话算进一个“半成品”身份里，容易触发上限。

我们脚本里的做法是等 90 秒：

```sh
SETTLE_SECONDS=90        # /usr/libexec/esurfing-macvlan-multidial
```

这个值按你的接入点认证速度调整。太快会撞上限，太慢影响开机体验。

### 8.4 健康检查

```sh
# 前台跑，每 30 秒检查一次每个会话的进程与路由表
/usr/libexec/esurfing-macvlan-multidial watch
```

它会做三件事：dialer 进程死了就停掉该会话、路由表丢了默认路由就告警、
源地址规则丢了就补回来。

### 8.5 时间同步

OpenWrt 时间不对会让日志时间错乱，也会影响 `local-time` 字段（部分接入点会校验）：

```sh
uci set system.@system[0].zonename='Asia/Shanghai'
uci set system.@system[0].timezone='CST-8'
uci set system.ntp.enabled='1'
uci commit system
/etc/init.d/system reload
/etc/init.d/sysntpd restart
```

---

## 9. 排障手册

### 9.1 症状 → 原因 → 处理

| 症状 | 大概率原因 | 处理 |
|---|---|---|
| `no route to host` | 认证流量走了 main 表，从错误的线路出去 | 加 `--bind-fwmark`，并确认 `ip rule` 里有对应规则 |
| 日志停在 `session initialization is still failing` | ZSM 模块解析失败 / 服务端给了不同的模块 | 试 `legacy_user_agent`；开 `--log-level debug` 看字节数与 SHA-256 |
| `invalid zsm data` 周期性出现 | Client-ID 与协商状态没一起清理 | 升级到包含 Client-ID 生命周期修复的版本 |
| `login rejected: ...` | 账号 / 密码 / 验证码问题，响应缺 `keep-url` | 看日志里的脱敏字段；确认是否需要短信验证码 |
| 认证成功但 `curl` 超时 | 数据面没通：策略路由 / rp_filter / 上游互斥 | 依次查 `ip rule`、`ip route get`、rp_filter；再看是不是第三路把前两路顶掉了 |
| `online user limit` | 上游在线数上限 | 减少会话数；稳定 MAC / IP 对（租约 hint） |
| 单个会话反复重认证 | 用 captive 探测判死导致的认证风暴 | 确认版本中 KEEP 是唯一存活信号；不要自己写探测脚本杀进程 |
| `term error` 超时 | TERM 接口响应慢 | 正常现象，best-effort；不影响退出 |
| macvlan 拿不到 DHCP | 上游按端口 / 账号绑定 | 回到方案 A（多物理口） |
| `ip rule` 里规则时有时无 | DHCP 续约换了地址 | 部署 `95-esurfing-source-rules` hotplug |
| 重启后会话顺序错乱 | 父线路还没认证完就开了子会话 | 增大 `SETTLE_SECONDS` |

### 9.2 必会的观测命令

```sh
# 每个会话的进程与路由表
/usr/libexec/esurfing-macvlan-multidial status

# 路由决策
ip route get 223.5.5.5 mark 0x300
ip route get 223.5.5.5 from 10.3.3.7

# 规则与表
ip rule show
ip route show table 3

# 网卡与地址
ip -details link show dev mvw1
ip -4 addr show

# 单个接口的通断
esurfingdialer-nettest.sh mvw1

# 日志
logread -f -e esurfingdialer
tail -f /tmp/vwan1.dialer.log
```

### 9.3 真实内存占用怎么看

`ps` 或 `top` 里的 `VSZ` 是虚拟地址空间，**不是** 实际占用。Go 进程请查 `VmRSS`：

```sh
for pid in $(pidof esurfingdialer); do
	echo "PID=$pid"
	grep -E '^(VmSize|VmRSS|VmHWM|Threads):' /proc/$pid/status
done
free -m
```

有 `smaps_rollup` 的话可以看更细的私有 / 共享常驻页。

---

## 10. 实测数据与容量规划

以下是我们那台路由器上的观测量级（具体数值随固件和会话数变化，仅供规划参考）：

| 会话数 | 每实例常驻内存 | 稳定性 |
|---|---|---|
| 1 | 十几 MB | 稳 |
| 2 | 十几 MB × 2 | 稳，长期运行 |
| 3 | 十几 MB × 3 | **认证能过，但数据面出现互斥** |
| 4 | 十几 MB × 4 | 四者身份可达性轮换，不可用于生产 |

第三条会话的实测时间线（单变量实验：不动已稳定的两条物理线，只加第三条）：

```text
20:55:24  第三条认证成功
20:55:44  第三个身份首个 KEEP 成功
20:56:15  物理线路 1 被黑洞
20:57:32  两条物理线路同时不可达，第三条仍可达
          全程零错误码、零 online user limit
```

停掉第三条后，物理线路自行恢复。

**结论**：在这种接入点上，**两路是安全上限**。第三路不是“配置问题”，是上游策略。
想验证更多路，必须本地有人值守，并准备好回滚。

---

## 11. 回滚

多拨改动集中在四层，回滚也按层来：

```sh
# 1. 停掉虚拟会话
/usr/libexec/esurfing-macvlan-multidial stop

# 2. 把虚拟接口从 mwan3 摘掉
uci set mwan3.vwan1.enabled='0'
uci set mwan3.vwan2.enabled='0'
uci commit mwan3
/etc/init.d/mwan3 restart

# 3. 恢复 network（删掉 vwan 接口段）
vi /etc/config/network
/etc/init.d/network reload

# 4. 确认只剩物理两路
ip rule show
ip route show table 1 | head -2
ip route show table 2 | head -2
mwan3 status
```

回滚的验收标准：**两条物理线路各自 `204`，main 表默认路由正常，零残留 macvlan 接口**。

```sh
ip -br link | grep -E 'mvw|mvlw'   # 应该没有输出
```

---

## 12. 速查表

```sh
# ---- 编译 ----
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o esurfingdialer ./cmd/esurfingdialer

# ---- 单拨调试 ----
/usr/bin/esurfingdialer -c /etc/esurfingdialer/wan.json --log-level debug

# ---- 服务 ----
/etc/init.d/esurfingdialer enable|start|stop|reload

# ---- macvlan 多拨 ----
/usr/libexec/esurfing-macvlan-multidial start|stop|status|watch
/usr/libexec/esurfing-macvlan-multidial start-one vwan1
/usr/libexec/esurfing-macvlan-multidial stop-one  vwan1

# ---- 验证 ----
esurfingdialer-nettest.sh mvw1
curl --interface mvw1 -s -o /dev/null -w '%{http_code}\n' http://connect.rom.miui.com/generate_204

# ---- 路由 ----
ip rule show
ip route show table 3
ip route get 223.5.5.5 mark 0x300

# ---- 日志 ----
logread -f -e esurfingdialer
tail -f /tmp/vwan1.dialer.log
```

---

## 附：术语

| 词 | 含义 |
|---|---|
| **多拨** | 同一账号 / 同一条线路上建立多个独立认证会话 |
| **macvlan** | 一张物理网卡派生出多个 MAC 的虚拟网卡技术 |
| **fwmark / SO_MARK** | 内核给 socket 打的标记，供 `ip rule` 选路由表 |
| **策略路由** | 按源地址 / fwmark 等条件选择路由表，而不是只看目的地址 |
| **mwan3** | OpenWrt 的多 WAN 负载均衡与故障切换组件 |
| **ZSM** | 服务端下发、决定本次会话使用哪种加密算法的模块 |
| **KEEP / TERM** | 会话保活接口 / 主动下线接口，地址由登录响应给出 |
