# Cross-validation fixtures

Captured from a JVM harness running the Java reference implementation, so the Go
code can be checked for byte-for-byte parity:

| File | Consumed by | Assertion |
|---|---|---|
| `java-crypto.out` | `internal/cipher/crosscheck_test.go` | ciphertext matches for every algorithm / plaintext pair, both directions |
| `java-md5.out` | `internal/netclient/md5_test.go` | `CDC-Checksum` MD5 hex |
| `java-parse.out` | `internal/app/extracttag_test.go` | XML tag extraction equivalence classes |
| `java-xml.out` | `internal/app/payload_render_test.go` | rendered request payload bytes |
| `java-zsm.out` | `internal/session/zsm_cross_test.go` | ZSM algo-ID parsing results |

All inputs are synthetic (fake accounts, documentation addresses, UUIDs that
exist only in the fixture). No real network data is present.

The upstream project keeps this directory one level above the Go module, which is
why the five consuming test files use `../../review/cross-validation/`. That
relative path is the only source difference in this repository.
