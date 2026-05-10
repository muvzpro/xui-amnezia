#!/bin/sh
set -eu

CONFIG_DIR="${AMNEZIA_CONFIG_DIR:-/etc/amnezia/amneziawg}"
mkdir -p "$CONFIG_DIR"

# Ensure ip_forward is enabled before bringing up interfaces
# In Docker containers /proc/sys may be read-only, so we try gracefully
if [ -w /proc/sys/net/ipv4/ip_forward ]; then
    echo 1 > /proc/sys/net/ipv4/ip_forward 2>/dev/null || true
fi
if [ -w /proc/sys/net/ipv6/conf/all/forwarding ]; then
    echo 1 > /proc/sys/net/ipv6/conf/all/forwarding 2>/dev/null || true
fi

# Ensure iptables-legacy is available as fallback (Alpine-specific)
if command -v iptables-legacy >/dev/null 2>&1; then
    update-alternatives --set iptables /usr/sbin/iptables-legacy 2>/dev/null || true
fi

up_all() {
  for conf in "$CONFIG_DIR"/*.conf; do
    [ -f "$conf" ] || continue
    iface="$(basename "$conf" .conf)"
    if awg show interfaces 2>/dev/null | tr ' ' '\n' | grep -qx "$iface"; then
      echo "Interface $iface already up, skipping"
      continue
    fi
    echo "Bringing up $iface..."
    awg-quick up "$conf" || {
      echo "awg-quick up failed for $iface, trying userspace fallback..."
      WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go awg-quick up "$conf" || true
    }
  done
}

down_all() {
  for iface in $(awg show interfaces 2>/dev/null || true); do
    conf="$CONFIG_DIR/$iface.conf"
    echo "Bringing down $iface..."
    awg-quick down "$conf" 2>/dev/null || awg-quick down "$iface" 2>/dev/null || true
  done
}

trap 'down_all; exit 0' INT TERM

up_all

# Keep container alive and watch for config changes
while :; do
  sleep 3600 &
  wait $!
done
