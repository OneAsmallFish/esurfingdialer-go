# 协议说明：一次认证里到底发生了什么

> 本文描述的是 **本项目自己的实现**，用于解释代码为何这么写。
> 不含任何抓包原文、真实地址或凭据。

## 0. 全景

```text
        ┌──────────────────┐
        │ 1. captive 探测  │  GET http://connect.rom.miui.com/generate_204
        └────────┬─────────┘
                 │ 302/200，正文里带一段校园配置注释块
        ┌────────▼─────────┐
        │ 2. 解析配置      │  auth-url / ticket-url / funcfg
        │                  │  wlanuserip / wlanacip
        └────────┬─────────┘
        ┌────────▼─────────┐
        │ 3. 会话初始化    │  POST ticket-url，body = algoId
        │                  │  ← ZSM 模块（key + algoId）
        └────────┬─────────┘
        ┌────────▼─────────┐
        │ 4. 取 ticket     │  POST ticket-url，body = 加密 XML
        │                  │  ← 加密 XML，取 ticket
        └────────┬─────────┘
        ┌────────▼─────────┐
        │ 5. 登录          │  POST auth-url，body = 加密 XML（账号 / 密码 / ticket）
        │                  │  ← keep-url / term-url / keep-retry
        └────────┬─────────┘
        ┌────────▼─────────┐
        │ 6. 保活          │  每 keep-retry 秒 POST keep-url
        │                  │  ← interval（动态更新间隔）
        └────────┬─────────┘
        ┌────────▼─────────┐
        │ 7. 下线          │  SIGTERM → POST term-url
        └──────────────────┘
```

---

## 1. Captive 探测与配置提取

请求：

```http
GET /generate_204 HTTP/1.1
User-Agent: CCTP/android64_vpn/2093
Accept: text/html,text/xml,application/xhtml+xml,application/x-javascript,*/*
Client-ID: <uuid v4>
```

重定向**手动跟随**（最多 5 跳），因为每一跳的响应头里都可能带：

| 响应头 | 用途 |
|---|---|
| `area` | 区域标识 |
| `schoolid` | 学校标识 |
| `domain` | 域标识 |

这三个值会被缓存下来，并在后续请求里作为 `CDC-Area` / `CDC-SchoolId` /
`CDC-Domain` 回传。重定向过程中 Cookie 与这些头都要保留。

正文里若出现这一对标记：

```text
<!--//config.campus.js.chinatelecom.com
    ... 校园配置 ...
//config.campus.js.chinatelecom.com-->
```

就说明当前处于**未认证**状态，从中解析出：

- `<auth-url>`：登录接口
- `<ticket-url>`：票据接口（查询串里带 `wlanuserip` 与 `wlanacip`）
- `<funcfg>`：若干带 `url` / `enable` 属性的子节点（短信验证码相关）

如果正文里没有这对标记：

- 命中独立 Portal 特征 → `ConnectivityRequirePortalLogin`
- 否则 → `ConnectivitySuccess`（已经在线，什么都不用做）

---

## 2. 请求的公共头

所有认证相关 POST 都带这些头：

| 头 | 值 |
|---|---|
| `User-Agent` | `CCTP/android64_vpn/2093`（可回退 2089） |
| `Accept` | `text/html,text/xml,application/xhtml+xml,application/x-javascript,*/*` |
| `Content-Type` | `application/x-www-form-urlencoded` |
| `Client-ID` | 本次会话的 UUID v4 |
| `Algo-ID` | 协商出来的算法 ID |
| `CDC-Checksum` | **请求体的 MD5 十六进制** |

`Algo-ID` 与 `Client-ID` 会做控制字符清洗（去掉 CR / LF 与 0x20 以下的字符），
避免头注入。

---

## 3. 会话初始化与 ZSM

请求体的内容就是当前 `Algo-ID`（首次是全零 UUID）。服务端返回一个二进制模块，
布局是：

```text
offset 0        : 4 字节头
offset 3        : 1 字节 keyLen
offset 4        : keyLen 字节 key
offset 4+keyLen : 1 字节 algoLen
offset 5+keyLen : algoLen 字节 algoId
```

解析出 `algoId` 后交给密码工厂，得到本次会话的加解密实现。
**重初始化失败时必须先把旧的 cipher 置空**，否则会带着上一次的密钥继续跑。

### 3.1 动态模块（Error-Code: 200）

部分接入点返回的不是上面的固定结构，而是一个 Android ZSM v2 容器，同时响应头
带 `Error-Code: 200`。容器布局：

```text
[0..3]        4 字节头
[3]           1 字节 keyLen
[4..]         key
[..]          1 字节 aidLen
[..]          aid（真正的算法 ID）
[..]          5 字节 props
[..]          4 字节 little-endian packed：
                bits 28..31 = variant     (必须 <= 3)
                bits  0..27 = decodedSize (必须 > 0 且 <= 0x08000000)
```

任何一个字段越界都必须拒绝，不能"猜一个算法凑合用"。

### 3.2 native 载荷规范化

用动态模块时，发送前的载荷要和 native 端字节级一致，需要三处调整：

1. XML 声明与根节点之间的换行去掉：`?> + 换行 + <request>` → `?><request>`
2. `<local-time>` 替换为 **UTC** 时间，格式 `2006-01-02 15:04:05`
3. 若还没有 `<sysinfo>`，在 `</request>` 前注入：

```text
<sysinfo><sysname>Linux</sysname><machine>armv8l</machine><ifname>wlan0</ifname><gsm.network.type>Unknown</gsm.network.type><ro.build.version.sdk>23</ro.build.version.sdk></sysinfo>
```

---

## 4. 取 ticket

加密前的 XML：

```xml
<?xml version="1.0" encoding="utf-8"?>
<request>
    <user-agent>CCTP/android64_vpn/2093</user-agent>
    <client-id>11111111-2222-3333-4444-555555555555</client-id>
    <local-time>1970-01-01 00:00:00</local-time>
    <host-name>TESTHOSTAB</host-name>
    <ipv4>192.0.2.10</ipv4>
    <ipv6></ipv6>
    <mac>02:00:00:00:00:01</mac>
    <ostag>TESTHOSTAB</ostag>
    <gwip>192.0.2.1</gwip>
</request>
```

其中：

- `ipv4` = 前面解析出的 `wlanuserip`
- `gwip` = 前面解析出的 `wlanacip`
- `host-name` / `ostag` = 每次进程启动随机生成的 10 位字符串
- `mac` = 每次进程启动随机生成，**第一字节最低位清零**（单播）

加密 → POST 到 `ticket-url` → 解密响应 → 取 `<ticket>`。

---

## 5. 登录

```xml
<?xml version="1.0" encoding="utf-8"?>
<request>
    <user-agent>CCTP/android64_vpn/2093</user-agent>
    <client-id>11111111-2222-3333-4444-555555555555</client-id>
    <ticket>FIXEDTICKET123</ticket>
    <local-time>1970-01-01 00:00:00</local-time>
    <userid>account</userid>
    <passwd>secret</passwd>
    <verify>123456</verify>
</request>
```

`<verify>` 只在需要短信验证码时出现。

响应里要取三个东西：

| 标签 | 含义 |
|---|---|
| `keep-url` | 保活接口 |
| `term-url` | 下线接口 |
| `keep-retry` | 首次保活间隔（秒） |

**判定规则**：只有拿到非空 `keep-url` 才算登录成功。
否则只输出**允许的脱敏字段**（`error-code` / `code` / `message` 等）、
响应长度和 SHA-256，然后退避重试。

> 这条规则来自一个真实缺陷：早期把"解密成功"当成"登录成功"，
> 于是一个空响应就让进程停在"本地以为在线、服务端没有会话"的半死状态。

---

## 6. 保活

```xml
<?xml version="1.0" encoding="utf-8"?>
<request>
    <user-agent>CCTP/android64_vpn/2093</user-agent>
    <client-id>11111111-2222-3333-4444-555555555555</client-id>
    <local-time>1970-01-01 00:00:00</local-time>
    <host-name>TESTHOSTAB</host-name>
    <ipv4>192.0.2.10</ipv4>
    <ticket>FIXEDTICKET123</ticket>
    <ipv6></ipv6>
    <mac>02:00:00:00:00:01</mac>
    <ostag>TESTHOSTAB</ostag>
</request>
```

响应里的 `<interval>` 会**替换**下一次的等待时间。

### 6.1 为什么 KEEP 是权威信号

已登录状态下，程序不再用 captive 探测判断存活。原因：

- KEEP 是服务端亲自确认的会话状态；
- captive 探测会经过运营商的强制门户链路，瞬时超时或 302 是常态；
- 一旦用探测结果判死，就会进入"下线 → 重认证 → 又探测失败"的风暴，
  会话数一多尤其明显。

所以状态机是：**KEEP 失败 → 才回到连通性检测 → 重新认证**。

---

## 7. 下线

请求体与保活相同，POST 到 `term-url`。整个 shutdown 流程：

1. 收到 `SIGINT` / `SIGTERM`；
2. 先取消 worker 的 context，中断在途请求；
3. TERM 用**独立的 context**（4 秒超时），不会被上面的取消影响；
4. 无论 TERM 成功与否都退出，失败只记日志。

TERM 经常超时是正常现象，服务端释放旧会话可能要几分钟。

---

## 8. Client-ID 生命周期

这是最容易写错、也最难排查的一处。

**规则**：`Client-ID` 与它协商出来的 `Algo-ID`、cipher、ticket、KEEP URL、TERM URL
属于**同一次认证尝试**，必须同生共死。

一次重认证的正确顺序：

```text
1. 尽力用旧身份 TERM（best-effort，独立 context）
2. 整组重置：cipher = nil, algoID = 全零, ticket = "",
             keepURL = "", termURL = "", keepRetry = 0
3. 生成新的 UUID v4 作为 Client-ID
4. 重新走会话初始化 → 取 ticket → 登录
```

只换 Client-ID 而不清理协商状态，服务端会用旧身份去解新请求，表现为
`invalid zsm data`——而且它是**间歇性**的，非常难定位。

---

## 9. 加密算法

密码工厂按 `algoId` 返回实现，支持：

AES-CBC / AES-ECB / DESede-CBC / DESede-ECB / SM4-CBC / SM4-ECB / ZUC /
ModXTEA / ModXTEA-XTEA-IV。

SM4 与 ZUC 来自 [gmsm](https://github.com/emmansun/gmsm)，是本项目唯一的第三方依赖。
加解密结果与 Java 参考实现做**逐字节**对拍，fixture 在
`review/cross-validation/`。

---

## 10. 日志里的脱敏约定

```text
HTTP request  url=<path-only> body-bytes=N body-sha256=<hex> ...
HTTP response url=<path-only> status=N error-code=".." bytes=N sha256=<hex> prefix-hex=<...>
```

- URL 只记录 `scheme://host/path`，**丢弃查询串**（查询串里有 `wlanuserip` 等环境信息）；
- 请求体与响应体只记录长度和摘要，不记录内容；
- `Client-ID` 只以 SHA-256 摘要形式出现；
- 登录失败时只输出白名单字段。

如果你发现任何一条日志路径会打出明文载荷或凭据，请按
[SECURITY.md](../SECURITY.md) 私下报告。
