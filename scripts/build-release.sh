#!/bin/sh
# Build every supported target into dist/ and write SHA256SUMS.
#
#   VERSION=v1.2.3 scripts/build-release.sh
#
# Kept as a shell script (not inlined in the workflow) so a release can be
# reproduced byte-for-byte on a laptop with the same Go toolchain.

set -eu

VERSION="${VERSION:-dev}"
DIST="${DIST:-dist}"
PKG="esurfingdialer"

rm -rf "$DIST"
mkdir -p "$DIST"

# os:arch:golang-env:suffix
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

build_one() {
	os="$1"; arch="$2"; envs="$3"; suffix="$4"
	name="$PKG-${VERSION}-${os}-${arch}${suffix}"
	[ "$os" = "windows" ] && name="$name.exe"

	echo "==> $name"
	# shellcheck disable=SC2086
	env CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" $envs \
		go build -trimpath \
		-ldflags "-s -w" \
		-o "$DIST/$name" ./cmd/esurfingdialer
}

echo "$TARGETS" | while IFS=: read -r os arch envs suffix; do
	[ -n "$os" ] || continue
	build_one "$os" "$arch" "$envs" "$suffix"
done

( cd "$DIST" && sha256sum ./* > SHA256SUMS )
echo
echo "artifacts:"
ls -l "$DIST"
