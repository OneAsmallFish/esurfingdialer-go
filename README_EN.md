# esurfingdialer

> **Turn one dorm ethernet drop into two.**
>
> A Go reimplementation of a China Telecom campus network (CCTP) authenticator:
> one static binary, no JVM, no GUI, automatic reconnect, multi-WAN and
> single-line multi-dial support.

[![ci](https://github.com/OneAsmallFish/esurfingdialer-go/actions/workflows/ci.yml/badge.svg)](https://github.com/OneAsmallFish/esurfingdialer-go/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/OneAsmallFish/esurfingdialer-go?sort=semver)](https://github.com/OneAsmallFish/esurfingdialer-go/releases)
[![license](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)

[中文](README.md) · [Multi-dial guide](docs/multi-dial.md) · [Protocol](docs/protocol.md) · [Troubleshooting](docs/troubleshooting.md) · [FAQ](docs/faq.md)

---

## What it is

It speaks the protocol itself: captive discovery, algorithm negotiation, ticket
acquisition, login, KEEP keep-alive and TERM logout.

| Problem | What this does |
|---|---|
| The official client is an app and will not run on a router | Pure Go, cross-compiles to one file for MIPS / ARM / x86 |
| Connections drop and need manual re-login | Keep-alive, backoff re-auth, procd supervision |
| You want to bond several uplinks | Interface / source-IP / fwmark binding, made for mwan3 |
| Instances interfere with each other | One config per process, precise procd restarts |

AES-CBC/ECB, DESede-CBC/ECB, SM4-CBC/ECB, ZUC and ModXTEA are supported. The
only third-party dependency is [gmsm](https://github.com/emmansun/gmsm) (SM4, ZUC).

It does **not** crack, bypass accounting or impersonate anyone — it authenticates
your own account for your own device.

## Quick start

```sh
git clone https://github.com/OneAsmallFish/esurfingdialer-go.git
cd esurfingdialer-go

go build -trimpath -ldflags "-s -w" -o esurfingdialer ./cmd/esurfingdialer
make release        # or cross-compile every published target into dist/

./esurfingdialer -u ACCOUNT -p PASSWORD
```

Success looks like:

```text
[INFO] The login has been authorized.
```

The process keeps the session alive by itself and logs out on `SIGTERM`.
See `./esurfingdialer --help` for every option.

## Configuration

Precedence: config file, then environment variables, then command-line flags.

```json
{
  "user": "ACCOUNT",
  "password": "PASSWORD",
  "log_level": "info",

  "bind_interface": "eth1",
  "bind_fwmark": "0x100",

  "portal_url": "",
  "portal_auto_login": false
}
```

- `bind_interface` / `bind_fwmark` / `bind_source_ip`: uplink binding, see
  chapter 7 of the multi-dial guide.
- `portal_host` falls back to the built-in default when empty, so the default
  path is unchanged; other campuses can override it without rebuilding.
- No account or credential is hard-coded anywhere in the source.

## Router multi-dial

### 👉 [`docs/multi-dial.md`](docs/multi-dial.md)

Three architectures compared, a step-by-step walkthrough, copy-pasteable UCI
config, how to read the policy routing chain, a troubleshooting matrix and a
rollback procedure. [`deploy/openwrt/`](deploy/openwrt/) ships the matching procd
service, hotplug scripts and the macvlan multi-dial manager.

```sh
scp esurfingdialer root@192.168.1.1:/usr/bin/
scp -r deploy/openwrt/* root@192.168.1.1:/
ssh root@192.168.1.1
vi /etc/esurfingdialer/wan.json      # fill in your account
/etc/init.d/esurfingdialer enable && /etc/init.d/esurfingdialer start
logread -f -e esurfingdialer
```

## Compatibility

| Target | GOOS / GOARCH | Notes |
|---|---|---|
| ARM64 router | `linux/arm64` | recommended |
| MT7621 / MT7628 | `linux/mipsle` + `GOMIPS=softfloat` | hardfloat traps with SIGILL |
| Older MIPS | `linux/mips` + `GOMIPS=softfloat` | |
| ARMv7 | `linux/arm` + `GOARM=7` | |
| x86 / NAS | `linux/amd64` | |
| Windows / macOS | `windows/amd64`, `darwin/*` | local debugging only |

Check `uname -m` and `opkg print-architecture` on the router first.

## Documentation

| Document | Contents |
|---|---|
| [docs/multi-dial.md](docs/multi-dial.md) | Router multi-dial guide |
| [docs/protocol.md](docs/protocol.md) | Auth chain, ZSM, Client-ID lifetime, log redaction |
| [docs/troubleshooting.md](docs/troubleshooting.md) | Client-side troubleshooting |
| [docs/faq.md](docs/faq.md) | Frequently asked questions |
| [deploy/openwrt/README.md](deploy/openwrt/README.md) | OpenWrt deployment assets |
| [CHANGELOG.md](CHANGELOG.md) | Release history |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |

## Disclaimer

- For **learning about network protocols and for personal, legitimate access**.
- Use only an account you are entitled to use, and follow your school's or
  operator's network policy and terms of service. Multi-dial may violate them.
- The authors are not responsible for account suspension, service interruption
  or hardware damage.
- Not affiliated with, endorsed by, or authorised by any telecom operator,
  equipment vendor or official client.
- No third-party client distribution is included.

## License

[GNU General Public License v3.0](LICENSE).

Copyright (C) 2026 esurfingdialer contributors

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version. It is distributed in the hope that it will be useful, but **WITHOUT ANY
WARRANTY**; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
A PARTICULAR PURPOSE. See LICENSE for details.
