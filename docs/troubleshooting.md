# 排障手册（客户端侧）

> 路由器 / 多拨相关的问题见 [multi-dial.md 第 9 章](multi-dial.md#9-排障手册)。
> 这里只讲客户端本身。

## 快速定位表

| 现象 | 先看哪里 |
|---|---|
| 启动就报 `config error` | 命令行 / 环境变量 / 配置文件是否都提供了账号密码 |
| 完全没有日志 | 可能已经在线，程序发现 204 就什么都不做 |
| 卡在 `session initialization` | ZSM 模块解析，开 `--log-level debug` |
| 一直 `login error` | 账号密码、验证码、或上游拒绝 |
| 登录成功但没网 | 绑定选项 / 路由 / 上游策略 |
| 反复重认证 | 版本是否包含 KEEP 权威修复 |

---

## 1. 完全没有输出

这是**正常**的：探测到已经联网（204）时，程序不需要做任何事，只会在 debug
级别留下痕迹。想要确认它在工作：

```sh
./esurfingdialer -u 账号 -p 密码 --log-level debug
```

如果确实需要重认证，先确认当前出口是否真的已经在线：

```sh
curl -s -o /dev/null -w '%{http_code}\n' http://connect.rom.miui.com/generate_204
```

---

## 2. session initialization is still failing

ZSM 模块没有通过校验。开 debug 看这几个值：

```text
error-code="" bytes=<N> sha256=<hex> prefix-hex=<...>
```

排查顺序：

1. **是不是模块变了？** 字节数与 sha256 是模块的唯一标识，先记录下来。
2. **试旧 UA。** 部分接入点对 `2093` 和 `2089` 返回**不同的**模块。
   配置里设置 `legacy_user_agent`，程序会在初始化失败时自动回退一次。
3. **不要**把未知模块硬映射到某个固定算法。宁可失败并保留证据。

相关代码：`internal/session/session.go` 的 `parseAlgoID` / `parseDynamicModule`。

---

## 3. invalid zsm data 反复出现

几乎总是 **Client-ID 生命周期**问题：换了 Client-ID 但没清协商状态，或者反过来。
确认你用的版本包含该修复（见 [CHANGELOG](../CHANGELOG.md) 1.0.0 的 Fixed 段）。

如果升级后仍然复现，请抓一次 debug 日志（含前后两次认证的完整往返），
把 sha256 和字节数贴到 issue 里——不要贴 URL。

---

## 4. login rejected

响应解密成功，但缺少 `keep-url`。日志会给出白名单字段：

```text
login rejected: error-code=..., code=..., message=...; bytes=N sha256=<hex>
```

常见原因：

| 字段 | 含义 | 处理 |
|---|---|---|
| 验证码相关 | 需要短信验证码 | 用 `-s` 预置，或在前台手动输入 |
| 账号 / 密码错误 | 凭据不对 | 核对；注意有的学校账号带后缀 |
| 在线数限制 | 该账号已有会话 | 减少实例数，或先等下线的旧会话释放 |
| 空响应 | 服务端没给任何字段 | 记录 sha256，可能是上游变更，开 issue |

---

## 5. 登录成功但没有网络

按顺序查：

```sh
# a. 当前接口对不对
ip route get 223.5.5.5

# b. 绑定选项是否生效（多 WAN 场景）
ip rule show

# c. 直接的 captive 探测
curl --interface <你的接口> -s -o /dev/null -w '%{http_code}\n' http://connect.rom.miui.com/generate_204

# d. DNS
nslookup example.com
```

如果 b 里没有你期望的规则，就是绑定 / 策略路由问题 → 看
[multi-dial.md](multi-dial.md#7-第四步策略路由最容易翻车的地方)。

---

## 6. 短信验证码

程序先查询"是否需要验证码"，需要才会提示。前台运行时：

```text
This login requires a SMS verification code.
Input Code:
```

无人值守场景预置：

```sh
./esurfingdialer -u 账号 -p 密码 -s 123456
# 或
ESURFING_SMS=123456 ./esurfingdialer
```

注意验证码有时效，预置值过期会导致登录失败。

---

## 7. 退出时的 term error

```text
[ERROR] term error: context deadline exceeded
```

**通常可以忽略。** TERM 接口响应慢是常态（4 秒超时），而且主动下线是 best-effort：
超时不影响进程退出，服务端会在之后自行回收会话。

真正需要关注的是：日志里连续出现 TERM 失败 **并且** 紧接着重新认证失败——
那说明旧会话还没释放，新会话就被拒绝了。这时候的做法是等几分钟再启动。

---

## 8. 日志时间不对

程序输出固定使用 UTC+8 的时间戳。如果路由器时区不对，启用 NTP：

```sh
uci set system.@system[0].zonename='Asia/Shanghai'
uci set system.@system[0].timezone='CST-8'
uci set system.ntp.enabled='1'
uci commit system
/etc/init.d/system reload
/etc/init.d/sysntpd restart
```

---

## 9. 内存看起来很大

`ps` / `top` 的 `VSZ` 是虚拟地址空间，Go 运行时会预留很多。
看 `VmRSS`：

```sh
grep -E '^(VmSize|VmRSS|VmHWM|Threads):' /proc/$(pidof esurfingdialer)/status
```

多实例场景给每个进程设 `GOMAXPROCS=1`（我们的 procd 脚本已经这么做），
能显著减少调度器线程数。

---

## 10. 还是不行

收集这些信息再开 issue：

```sh
./esurfingdialer --version
uname -m
head -3 /etc/openwrt_release 2>/dev/null
# debug 日志（务必把 URL 换成 <url>）
./esurfingdialer -c 你的配置 --log-level debug 2>&1 | sed -E 's#https?://[^ ]+#<url>#g'
```

**不要**贴：账号、密码、验证码、内网 IP、MAC、完整 URL、抓包原文。
