#!/bin/sh
# VERSION=v1.0.1 sh scripts/build-release.sh
set -eu
VERSION="${VERSION:-dev}"
DIST="${DIST:-dist}"
case "$VERSION" in
    ''|*[!A-Za-z0-9._-]*) echo "invalid VERSION" >&2; exit 2 ;;
esac
mkdir -p "$DIST"
: > "$DIST/SHA256SUMS"
TARGETS="
linux:amd64::
linux:arm64::
linux:arm:GOARM=7:v7
linux:386::
linux:mipsle:GOMIPS=softfloat:sf
linux:mips:GOMIPS=softfloat:sf
freebsd:amd64::
windows:amd64::
darwin:amd64::
darwin:arm64::
"
printf '%s\n' "$TARGETS" | while IFS=: read -r os arch envs suffix; do
    [ -n "$os" ] || continue
    name="esurfingdialer-${VERSION}-${os}-${arch}${suffix}"
    [ "$os" != windows ] || name="$name.exe"
    echo "==> $name"
    # envs contains only the fixed GOARM/GOMIPS assignments above.
    # shellcheck disable=SC2086
    env CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" $envs \
        go build -trimpath -ldflags "-s -w" -o "$DIST/$name" ./cmd/esurfingdialer
    (cd "$DIST" && sha256sum "$name") >> "$DIST/SHA256SUMS"
done
