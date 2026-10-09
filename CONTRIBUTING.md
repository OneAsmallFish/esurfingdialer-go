# Contributing

Thanks for taking the time to help. This is a small, focused project; the rules
below keep it reviewable.

## Ground rules

1. **Never commit real credentials or real network detail.** No account names,
   passwords, SMS codes, campus IP addresses, MAC addresses, hostnames or raw
   packet captures. Use documentation ranges (`192.0.2.0/24`, `2001:db8::/32`,
   `02:00:00:00:xx:xx`) and obviously fake accounts.
2. **No third-party binaries.** Do not add decompiled or repackaged output from
   any vendor client.
3. **Keep the dependency count at one.** The project deliberately depends only
   on `github.com/emmansun/gmsm` for SM4 and ZUC. A new dependency needs a very
   good reason in the pull request description.
4. **Do not build features that bypass accounting or impersonate someone else.**
   The tool authenticates *your* account for *your* device.

## Development

```sh
git clone https://github.com/OneAsmallFish/esurfingdialer-go.git
cd esurfingdialer-go

make fmt-check     # gofmt -s
make vet
make test
make test-race
make release       # cross-compile every published target
```

Go 1.24 or newer.

### Optional Java parity fixtures

`review/cross-validation/*.out` are captured from a JVM harness running the
reference implementation, and let the cipher, MD5, XML-rendering and ZSM tests
assert byte-for-byte parity.

The upstream project keeps these fixtures one directory above the Go module, so
the five test files that read them resolve `../../review/cross-validation/`
instead. **That relative path is the only difference between this repository's
Go source and the upstream tree** - no behaviour is changed.

```sh
# point the cipher cross-check at your own captures
ESURFING_JAVA_OUT=/path/to/java-crypto.out go test ./internal/cipher -v
```

## Shell scripts

Deployment scripts target OpenWrt's BusyBox `sh`. CI runs `shellcheck` at error
severity; run it locally before pushing:

```sh
shellcheck --severity=error --shell=sh deploy/openwrt/usr/libexec/*
bash -n deploy/openwrt/etc/init.d/esurfingdialer
```

## Pull requests

- One logical change per pull request.
- Explain *why*, not just *what*. If it fixes a field problem, describe the
  symptom and how you confirmed the fix.
- Add or update a test when behaviour changes.
- Update `CHANGELOG.md` under `## [Unreleased]`.

## Reporting bugs

Use the issue template. Include the Go version, target `GOOS/GOARCH`, router
model and firmware, and the relevant log lines **with URLs redacted**. Never
paste credentials.
