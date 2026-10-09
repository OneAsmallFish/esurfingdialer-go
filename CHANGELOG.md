# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-10-10

First public release of the standalone Go implementation.

### Added

- Protocol-level CCTP authenticator: captive discovery, algorithm negotiation,
  ticket acquisition, login, KEEP keep-alive and TERM logout.
- Cipher implementations for AES-CBC/ECB, DESede-CBC/ECB, SM4-CBC/ECB, ZUC,
  ModXTEA and ModXTEA-XTEA-IV.
- Dynamic ZSM module support (`Error-Code: 200`) with native payload
  normalisation, plus a one-shot legacy user-agent fallback.
- Optional standalone H3C portal auto-login.
- Multi-uplink binding: `--bind-interface` (`SO_BINDTODEVICE`),
  `--bind-fwmark` (`SO_MARK`) and `--bind-source-ip`.
- Configurable portal host; no campus address is compiled into the binary.
- OpenWrt deployment assets: procd service, interface hotplug scripts,
  per-interface connectivity probe and a macvlan multi-dial manager.
- Documentation: `docs/multi-dial.md` (router multi-dial), `docs/protocol.md`,
  `docs/troubleshooting.md` and `docs/faq.md`.
- GitHub Actions: CI (fmt, vet, race tests, cross-compile matrix, shellcheck)
  and a tag-driven release workflow publishing ten target builds plus a bundle.
- Java cross-validation fixtures under `review/cross-validation/`, letting the
  cipher, MD5, XML-rendering and ZSM tests assert parity with the reference
  implementation.

### Fixed

- Client-ID lifecycle: every full authorization uses a fresh UUID v4 and clears
  the negotiated Algo-ID, cipher and ticket state as one unit.
- Login response validation: a decrypted response without `keep-url` is no
  longer recorded as a successful login.
- KEEP is the authoritative liveness signal; captive probes no longer trigger
  re-authentication storms on an established session.

[Unreleased]: https://github.com/OneAsmallFish/esurfingdialer-go/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/OneAsmallFish/esurfingdialer-go/releases/tag/v1.0.0
