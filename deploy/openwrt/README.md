# OpenWrt deployment assets

Everything needed to run the dialer as a supervised service on OpenWrt, plus the
macvlan multi-dial manager.

Read [`../docs/multi-dial.md`](../../docs/multi-dial.md) first — it explains the
architecture, the policy routing and the failure modes.

## Layout

```text
deploy/openwrt/
├── etc/
│   ├── config/
│   │   ├── esurfingdialer        # one procd instance per uplink
│   │   ├── esurfing-multidial    # one macvlan session per virtual MAC
│   │   ├── mwan3.example         # load balancing / failover
│   │   └── network.example       # per-session routing tables
│   ├── esurfingdialer/
│   │   ├── wan.json.example
│   │   └── wan2.json.example
│   ├── init.d/
│   │   └── esurfingdialer        # procd service
│   └── hotplug.d/iface/
│       ├── 95-esurfing-source-rules
│       └── 99-esurfingdialer
└── usr/
    ├── bin/esurfingdialer-nettest.sh
    └── libexec/esurfing-macvlan-multidial
```

## Install

```sh
# from the bundle root
scp -r deploy/openwrt/* root@192.168.1.1:/
ssh root@192.168.1.1

chmod +x /etc/init.d/esurfingdialer \
         /etc/hotplug.d/iface/95-esurfing-source-rules \
         /etc/hotplug.d/iface/99-esurfingdialer \
         /usr/bin/esurfingdialer-nettest.sh \
         /usr/libexec/esurfing-macvlan-multidial

# credentials live here, never in the repo
cp /etc/esurfingdialer/wan.json.example /etc/esurfingdialer/wan.json
chmod 600 /etc/esurfingdialer/wan.json
vi /etc/esurfingdialer/wan.json
```

## The service

`/etc/init.d/esurfingdialer` starts one procd instance per `config instance`
section in `/etc/config/esurfingdialer`:

```sh
/etc/init.d/esurfingdialer enable
/etc/init.d/esurfingdialer start
/etc/init.d/esurfingdialer reload     # picks up config changes

logread -f -e esurfingdialer
```

Useful options per instance:

| Option | Meaning |
|---|---|
| `enabled` | set to `0` to keep the section but skip it |
| `config_file` | path to the JSON config holding credentials |
| `interface` | logical netifd interface; also wires up the reload trigger |
| `bind_interface` | passed as `--bind-interface` |
| `bind_fwmark` | passed as `--bind-fwmark` |
| `bind_source_ip` | passed as `--bind-source-ip`, optional |
| `log_level` | `debug` / `info` / `warn` / `error` |

Each instance runs with `GOMAXPROCS=1`, `respawn` and `term_timeout 30`.

## The macvlan multi-dial manager

```sh
/usr/libexec/esurfing-macvlan-multidial start      # all enabled sessions
/usr/libexec/esurfing-macvlan-multidial status
/usr/libexec/esurfing-macvlan-multidial stop

/usr/libexec/esurfing-macvlan-multidial start-one vwan1
/usr/libexec/esurfing-macvlan-multidial stop-one  vwan1

/usr/libexec/esurfing-macvlan-multidial watch      # 30 s health loop
```

It is idempotent: creating an existing macvlan link, re-adding an existing `ip
rule` or re-persisting an unchanged lease hint are all no-ops.

Every action is driven by `/etc/config/esurfing-multidial`. The session name must
match the netifd interface that owns the DHCP client and the routing table, so
`/etc/config/network` needs a matching block — see `network.example`.

Health output is a single line per session:

```text
vwan1 device=mvw1 link=up ip=10.3.3.7 dialer_alive=yes table_default=yes
```

`link=up` with `dialer_alive=no` means authorization failed; check
`/tmp/vwan1.dialer.log`. The manager also writes the last good lease to
`network.<name>.ipaddr` so the MAC/IP pair stays stable across restarts.

## Connectivity probe

```sh
esurfingdialer-nettest.sh eth1
esurfingdialer-nettest.sh mvw1 223.5.5.5 100 0.2
```

Pings the target through a specific interface, then polls the captive endpoint 30
times. **A member must return 204 here before you add it to mwan3.**

## Package requirements

```sh
opkg update
opkg install mwan3 ip-full curl kmod-macvlan
```

`ip-full` matters: BusyBox `ip` does not implement the full `ip rule` and policy
routing syntax the manager relies on.

## Safety notes

- Config files holding credentials are mode `0600`; the manager copies them to
  `/tmp` with the same mode and never echoes them.
- Dialer logs are redacted before being echoed on failure: URLs are replaced with
  `<url>`.
- Removing a session deletes only the macvlan link it created. It never touches
  `eth0`, a DSA LAN port, or the parent interface.
- The manager verifies `/proc/<pid>/cmdline` before signalling a PID, so a
  recycled PID is never killed by mistake.
