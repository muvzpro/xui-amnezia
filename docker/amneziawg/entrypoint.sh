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

# Check if /dev/net/tun is available
if [ ! -c /dev/net/tun ]; then
    echo "ERROR: /dev/net/tun device not available. WireGuard cannot create tunnel interfaces."
    echo "Make sure the container has --device /dev/net/tun:/dev/net/tun and NET_ADMIN capability."
    exit 1
fi

echo "Starting AmneziaWG entrypoint..."
echo "CONFIG_DIR: $CONFIG_DIR"
echo "Available configs: $(ls -la $CONFIG_DIR/*.conf 2>/dev/null || echo 'none')"

up_all() {
  for conf in "$CONFIG_DIR"/*.conf; do
    [ -f "$conf" ] || continue
    iface="$(basename "$conf" .conf)"
    if awg show interfaces 2>/dev/null | tr ' ' '\n' | grep -qx "$iface"; then
      echo "Interface $iface already up, skipping"
      continue
    fi
    echo "Bringing up $iface with config $conf..."
    if awg-quick up "$conf"; then
        echo "Successfully brought up $iface"
    else
        echo "Failed to bring up $iface, trying userspace fallback..."
        if WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go awg-quick up "$conf"; then
            echo "Successfully brought up $iface with userspace fallback"
        else
            echo "Failed to bring up $iface even with userspace fallback"
        fi
    fi
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
echo "AmneziaWG entrypoint ready. Active interfaces: $(awg show interfaces 2>/dev/null || echo 'none')"
while :; do
  sleep 3600 &
  wait $!
done
