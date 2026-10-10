# Windows 使用

Windows 版适合让一台 Windows x64 电脑直接连接校园网并完成认证。它是命令行程序，没有图形界面，也不自带托盘或 Windows 服务。

**Windows 版不是路由器多拨方案。** 当前 `bind_interface`、`bind_fwmark`、`bind_source_ip` 在非 Linux 平台会被忽略；不要照抄 OpenWrt 的出口绑定配置，也不要认为多开几个窗口就能聚合带宽。

## 1. 下载并校验

到 [Releases](https://github.com/OneAsmallFish/esurfingdialer-go/releases/latest) 下载文件名带 `windows-amd64.exe` 的程序和 `SHA256SUMS`，放在自己的工作目录里，例如 `D:\esurfingdialer`。

在该目录打开 PowerShell。以 v1.0.1 的文件名为例：

```powershell
Get-FileHash -LiteralPath .\esurfingdialer-v1.0.1-windows-amd64.exe -Algorithm SHA256
Select-String -LiteralPath .\SHA256SUMS -Pattern 'windows-amd64.exe'
```

比较两个 SHA-256 值是否一致，确认下载来源后，把程序重命名为 `esurfingdialer.exe`。其他版本按实际文件名替换，不需要安装 Go、Java 或额外 DLL。

如果文件因下载来源被阻止，在核验文件后可查看“属性 → 解除锁定”，或执行：

```powershell
Unblock-File -LiteralPath .\esurfingdialer.exe
```

这不是关闭安全软件或绕过恶意软件检查的办法；仍有安全告警时应先核对来源，不要盲目放行。

## 2. 建立配置文件

在同一目录生成一个不带 BOM 的 UTF-8 配置模板：

```powershell
$config = @'
{
  "user": "你的拨号账号",
  "password": "你的拨号密码",
  "log_level": "info",
  "portal_auto_login": false
}
'@
$path = Join-Path $PWD 'windows.local.json'
[System.IO.File]::WriteAllText($path, $config, [System.Text.UTF8Encoding]::new($false))
notepad $path
```

在记事本中将账号、密码换成自己的并保存，保持 UTF-8 编码、不带 BOM。Windows PowerShell 5.1 的 `Set-Content -Encoding UTF8` 会写入 BOM，不适合直接用来生成本程序的 JSON。

配置文件包含密码，只保存在自己可访问的目录，不要放到公开网盘或提交到仓库。使用配置文件比把密码写进命令行更方便，也能避免密码出现在命令历史或进程参数中。

## 3. 接入校园网并运行

电脑先连接需要认证的校园网，让网卡获得 DHCP 地址。检查时可运行：

```powershell
ipconfig
```

如果电脑接在一个已经认证的路由器后面，上网的是路由器的会话，不代表电脑已验证了直接接入认证。

在 PowerShell 运行：

```powershell
.\esurfingdialer.exe -c .\windows.local.json
```

先用普通终端即可，不需要为出口绑定而打开管理员终端，因为 Windows 版没有实现这些绑定。

看到 `The login has been authorized.` 后，实际打开网页确认能上网。程序需要保持运行；结束时在同一窗口按 `Ctrl-C`，让它尝试正常下线。关闭窗口、强制结束或断电不保证能完成下线，学校的旧会话可能还需要等待释放。

## 4. 有独立网页门户时

默认不要开启 `portal_auto_login`。确认接入点还需要网页认证，并且是程序支持的门户页面时，再在 JSON 中合并这些字段：

```json
{
  "portal_auto_login": true,
  "portal_url": "http://portal.example.invalid/portal/",
  "portal_user": "你的门户账号",
  "portal_password": "你的门户密码",
  "portal_mac": "02:00:00:00:00:01"
}
```

这只是可选字段片段。URL 和 MAC 都是占位值，需要换成学校门户地址和实际接入网卡的 MAC。门户账号可能与拨号账号不同。网卡 MAC 可以在 `ipconfig /all` 中查看。

## 常见问题

**双击后窗口一闪就没了？** 程序需要账号配置，请从 PowerShell 按上面的命令启动，这样可以看到错误信息。

**提示缺少账号、密码？** 检查是否运行了正确目录里的配置，字段名应是 `user`、`password`，两者都不能为空。

**提示 JSON 解析失败？** 检查是否用了英文双引号、是否有多余逗号；也要确认文件不是 UTF-16 或带 BOM 的 UTF-8。

**登录成功但仍不能上网？** 检查电脑是否真的在校园网接入线路上；有 VPN、多张网卡或独立门户时，需要确认系统实际选路与认证接入一致。Windows 版不能用 `bind_interface` 固定出口，提升终端权限也不会补上这项功能。

**想让路由器给多台设备上网？** 用 Linux / OpenWrt 版，见[多拨教程](multi-dial.md)，不要用 Windows 版冒充路由器部署。

**查看参数和进程占用：**

```powershell
.\esurfingdialer.exe -h
Get-Process -Name esurfingdialer | Select-Object Id, CPU, WorkingSet64, PrivateMemorySize64
```

`CPU` 是累计 CPU 秒数，不是当前百分比；当前 CPU 百分比可以在任务管理器中查看。内存列单位为字节，工作集与进程私有内存也不是同一个指标。这里没有承诺固定的 CPU 或内存数值，实际请以自己的电脑和网络状态为准。

上述步骤已核对当前参数与 Windows 构建；并不代表所有学校的真实认证都已验证。网络兼容性仍取决于接入系统。
