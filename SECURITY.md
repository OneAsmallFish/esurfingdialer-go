# Security Policy

## Supported versions

The latest tagged release receives fixes. Older tags are not maintained.

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Use GitHub's private vulnerability reporting on this repository
(Security -> Report a vulnerability). If that is unavailable, contact a
maintainer directly.

Please include:

- affected version or commit,
- target platform,
- a minimal reproduction,
- impact assessment,
- any suggested mitigation.

We aim to acknowledge within a few days and to ship a fix in the next release.

## Scope

In scope:

- Memory-safety or panic-triggering input handling (especially the ZSM module
  parser and the cipher factories).
- Credential leakage through logs, error messages or crash output.
- Anything that lets a remote party cause the dialer to authenticate with
  credentials it should not use.

Out of scope:

- The behaviour of the upstream campus network itself.
- Terms-of-service questions about multi-dial. See the disclaimer in README.md.
- Physical or local-root attackers: the config files are protected with `0600`
  and the router's own hardening is out of scope.

## Handling of secrets

The dialer never logs credentials. Debug logs record request and response
**metadata** (URL path, byte counts, SHA-256 digests) rather than payloads, and
the Client-ID is logged only as a digest. If you find a path that leaks a
payload, that is a security bug — please report it.
