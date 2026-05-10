#!/bin/sh
set -eu

CONFIG_DIR="${AMNEZIA_CONFIG_DIR:-/etc/amnezia/amneziawg}"
mkdir -p "$CONFIG_DIR"

up_all() {
  for conf in "$CONFIG_DIR"/*.conf; do
    [ -f "$conf" ] || continue
    iface="$(basename "$conf" .conf)"
    if awg show interfaces 2>/dev/null | tr ' ' '\n' | grep -qx "$iface"; then
      continue
    fi
    awg-quick up "$conf" || true
  done
}

down_all() {
  for iface in $(awg show interfaces 2>/dev/null || true); do
    conf="$CONFIG_DIR/$iface.conf"
    awg-quick down "$conf" || awg-quick down "$iface" || true
  done
}

trap 'down_all; exit 0' INT TERM

up_all
while :; do
  sleep 3600 &
  wait $!
done
