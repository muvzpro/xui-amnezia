#!/bin/bash
# AWG Helper Script for 3x-ui
# Provides safe Docker management for AmneziaWG container
# Based on Amnezia-Web-Panel approach

set -euo pipefail

# Configuration
CONTAINER_NAME="${XUI_AMNEZIA_DOCKER_CONTAINER:-3xui_amneziawg}"
CONFIG_DIR="${AMNEZIA_CONFIG_DIR:-/etc/amnezia/amneziawg}"
DOCKER_CMD="${DOCKER_CMD:-docker}"

# Logging
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [AWG-HELPER] $*" >&2
}

error() {
    log "ERROR: $*" >&2
    exit 1
}

# Check if container is running
check_container() {
    if ! $DOCKER_CMD ps --format '{{.Names}}' | grep -qx "$CONTAINER_NAME"; then
        error "Container $CONTAINER_NAME is not running"
    fi
}

# Execute command in container
docker_exec() {
    $DOCKER_CMD exec -i "$CONTAINER_NAME" "$@"
}

# Get server config from container
get_config() {
    local interface="${1:-awg0}"
    local config_path="$CONFIG_DIR/${interface}.conf"
    docker_exec cat "$config_path"
}

# Save config to container
save_config() {
    local interface="${1:-awg0}"
    local config_path="$CONFIG_DIR/${interface}.conf"
    local temp_file

    # Read config from stdin
    temp_file=$(mktemp)
    cat > "$temp_file"

    # Copy to container
    $DOCKER_CMD cp "$temp_file" "$CONTAINER_NAME:$config_path"
    rm -f "$temp_file"

    log "Config saved for interface $interface"
}

# Sync config without restart
sync_config() {
    local interface="${1:-awg0}"
    local config_path="$CONFIG_DIR/${interface}.conf"

    log "Syncing config for interface $interface"

    # Use awg-quick strip + awg syncconf for hot reload
    if docker_exec sh -c "awg-quick strip '$config_path' | awg syncconf '$interface' /dev/stdin"; then
        log "Config synced successfully"
    else
        error "Failed to sync config"
    fi
}

# Show interface status
show_status() {
    local interface="${1:-awg0}"
    docker_exec awg show "$interface"
}

# Show all dump
show_dump() {
    docker_exec awg show all dump
}

# Generate keys
gen_key() {
    docker_exec awg genkey
}

pub_key() {
    docker_exec sh -c "cat | awg pubkey" < /dev/stdin
}

gen_psk() {
    docker_exec awg genpsk
}

# Add peer to config
add_peer() {
    local interface="${1:-awg0}"
    local pubkey="$2"
    local psk="$3"
    local allowed_ips="$4"
    local config_path="$CONFIG_DIR/${interface}.conf"

    log "Adding peer $pubkey to interface $interface"

    # Get current config
    local config
    config=$(get_config "$interface")

    # Check if peer already exists
    if echo "$config" | grep -q "PublicKey = $pubkey"; then
        error "Peer $pubkey already exists"
    fi

    # Add peer section
    local peer_section
    peer_section=$(
        echo "[Peer]"
        echo "PublicKey = $pubkey"
        [ -n "$psk" ] && echo "PresharedKey = $psk"
        echo "AllowedIPs = $allowed_ips"
    )

    # Append to config
    {
        echo "$config"
        echo
        echo "$peer_section"
    } | save_config "$interface"

    # Sync config
    sync_config "$interface"
}

# Remove peer from config
remove_peer() {
    local interface="${1:-awg0}"
    local pubkey="$2"
    local config_path="$CONFIG_DIR/${interface}.conf"

    log "Removing peer $pubkey from interface $interface"

    # Get current config
    local config
    config=$(get_config "$interface")

    # Remove peer section
    local new_config
    new_config=$(echo "$config" | awk -v pubkey="$pubkey" '
        BEGIN { in_peer = 0; skip = 0 }
        /^\[Peer\]/ {
            in_peer = 1
            skip = 0
            peer_block = $0 "\n"
            next
        }
        in_peer && /^PublicKey = / {
            peer_block = peer_block $0 "\n"
            if ($3 == pubkey) {
                skip = 1
            }
            next
        }
        in_peer && /^$/ {
            if (!skip) {
                print peer_block
            }
            in_peer = 0
            peer_block = ""
            next
        }
        in_peer {
            peer_block = peer_block $0 "\n"
            next
        }
        {
            print
        }
        END {
            if (in_peer && !skip) {
                print peer_block
            }
        }
    ')

    # Save new config
    echo "$new_config" | save_config "$interface"

    # Sync config
    sync_config "$interface"
}

# Main command dispatcher
case "${1:-}" in
    check-container)
        check_container
        echo "Container $CONTAINER_NAME is running"
        ;;
    get-config)
        check_container
        get_config "${2:-awg0}"
        ;;
    save-config)
        check_container
        save_config "${2:-awg0}"
        ;;
    sync-config)
        check_container
        sync_config "${2:-awg0}"
        ;;
    show-status)
        check_container
        show_status "${2:-awg0}"
        ;;
    show-dump)
        check_container
        show_dump
        ;;
    gen-key)
        check_container
        gen_key
        ;;
    pub-key)
        check_container
        pub_key
        ;;
    gen-psk)
        check_container
        gen_psk
        ;;
    add-peer)
        check_container
        if [ $# -lt 5 ]; then
            error "Usage: $0 add-peer [interface] <pubkey> <psk> <allowed_ips>"
        fi
        add_peer "${2:-awg0}" "$3" "$4" "$5"
        ;;
    remove-peer)
        check_container
        if [ $# -lt 3 ]; then
            error "Usage: $0 remove-peer [interface] <pubkey>"
        fi
        remove_peer "${2:-awg0}" "$3"
        ;;
    *)
        cat >&2 <<EOF
AWG Helper Script for 3x-ui

Usage: $0 <command> [args...]

Commands:
    check-container              Check if container is running
    get-config [interface]       Get server config
    save-config [interface]      Save config from stdin
    sync-config [interface]      Sync config without restart
    show-status [interface]      Show interface status
    show-dump                    Show all dump
    gen-key                      Generate private key
    pub-key                      Generate public key from stdin
    gen-psk                      Generate preshared key
    add-peer [iface] <pub> <psk> <ips>  Add peer to config
    remove-peer [iface] <pub>    Remove peer from config

Environment variables:
    CONTAINER_NAME               Container name (default: 3xui_amneziawg)
    CONFIG_DIR                   Config directory (default: /etc/amnezia/amneziawg)
    DOCKER_CMD                   Docker command (default: docker)
EOF
        exit 1
        ;;
esac