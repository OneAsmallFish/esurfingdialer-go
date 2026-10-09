#!/bin/sh
# Per-interface connectivity check. Use it to prove that a specific upstream
# (physical WAN or macvlan virtual session) really carries traffic before you
# add it to mwan3.
#
#   esurfingdialer-nettest.sh <iface> [target] [count] [interval]

IFACE="$1"
TARGET="${2:-223.5.5.5}"
COUNT="${3:-50}"
INTERVAL="${4:-0.2}"
CAPTIVE="${CAPTIVE_URL:-http://connect.rom.miui.com/generate_204}"

if [ -z "$IFACE" ]; then
	echo "Usage: $0 <iface> [target] [count] [interval]" >&2
	exit 1
fi

if [ ! -e "/sys/class/net/$IFACE" ]; then
	echo "interface $IFACE does not exist" >&2
	exit 1
fi

echo "== ping $IFACE -> $TARGET ($COUNT packets, ${INTERVAL}s apart) =="
ping -I "$IFACE" -c "$COUNT" -i "$INTERVAL" "$TARGET"
echo

echo "== captive probe $IFACE -> $CAPTIVE =="
echo "expect 204 once authorization is complete; 302 means the portal is still intercepting"
i=0
while [ "$i" -lt 30 ]; do
	curl --interface "$IFACE" -m 3 -s -o /dev/null \
		-w 'http=%{http_code} time=%{time_total}s\n' "$CAPTIVE"
	i=$((i + 1))
	sleep 1
done
