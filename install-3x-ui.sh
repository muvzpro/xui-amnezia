#!/bin/bash

# 3X-UI with AmneziaWG Support Installation Script
# Repository: https://github.com/muvzpro/xui-amnezia

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# Repository configuration
REPO_URL="https://github.com/muvzpro/xui-amnezia"
RAW_URL="https://raw.githubusercontent.com/muvzpro/xui-amnezia/master"
RELEASE_URL="https://api.github.com/repos/muvzpro/xui-amnezia/releases/latest"

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"
amnezia_folder="/etc/amnezia/amneziawg"
amneziawg_bin="/usr/local/bin/amneziawg-go"
awg_bin="/usr/local/bin/awg"
awg_quick_bin="/usr/local/bin/awg-quick"

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "Failed to check the system OS, please contact the author!" >&2
    exit 1
fi
echo "The OS release is: $release"

arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}Unsupported CPU architecture! ${plain}" && rm -f install.sh && exit 1 ;;
    esac
}

echo "Arch: $(arch)"

# Simple helpers
is_ipv4() {
    [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] && return 0 || return 1
}
is_ipv6() {
    [[ "$1" =~ : ]] && return 0 || return 1
}
is_ip() {
    is_ipv4 "$1" || is_ipv6 "$1"
}
is_domain() {
    [[ "$1" =~ ^([A-Za-z0-9](-*[A-Za-z0-9])*\.)+(xn--[a-z0-9]{2,}|[A-Za-z]{2,})$ ]] && return 0 || return 1
}

# Port helpers
is_port_in_use() {
    local port="$1"
    if command -v ss > /dev/null 2>&1; then
        ss -ltn 2> /dev/null | awk -v p=":${port}$" '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    if command -v netstat > /dev/null 2>&1; then
        netstat -lnt 2> /dev/null | awk -v p=":${port} " '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    if command -v lsof > /dev/null 2>&1; then
        lsof -nP -iTCP:${port} -sTCP:LISTEN > /dev/null 2>&1 && return 0
    fi
    return 1
}

install_base() {
    echo -e "${green}Installing base packages...${plain}"
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl \
                git make gcc golang-go qrencode iproute2 iptables
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl \
                git make gcc golang qrencode iproute iptables
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum -y update && yum install -y cronie curl tar tzdata socat ca-certificates openssl \
                    git make gcc golang qrencode iproute iptables
            else
                dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl \
                    git make gcc golang qrencode iproute iptables
            fi
            ;;
        arch | manjaro | parch)
            pacman -Syu && pacman -Syu --noconfirm cronie curl tar tzdata socat ca-certificates openssl \
                git make gcc go qrencode iproute2 iptables
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh && zypper -q install -y cron curl tar timezone socat ca-certificates openssl \
                git make gcc go qrencode iproute2 iptables
            ;;
        alpine)
            apk update && apk add dcron curl tar tzdata socat ca-certificates openssl \
                git make gcc go qrencode iproute2 iptables
            ;;
        *)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl \
                git make gcc golang-go qrencode iproute2 iptables
            ;;
    esac
}

gen_random_string() {
    local length="$1"
    openssl rand -base64 $((length * 2)) \
        | tr -dc 'a-zA-Z0-9' \
        | head -c "$length"
}

install_acme() {
    echo -e "${green}Installing acme.sh for SSL certificate management...${plain}"
    cd ~ || return 1
    curl -s https://get.acme.sh | sh > /dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${red}Failed to install acme.sh${plain}"
        return 1
    else
        echo -e "${green}acme.sh installed successfully${plain}"
    fi
    return 0
}

# Install AmneziaWG (amneziawg-go and amneziawg-tools)
install_amneziawg() {
    echo -e "${green}Installing AmneziaWG...${plain}"
    
    # Check if already installed
    if command -v awg &> /dev/null && command -v awg-quick &> /dev/null && command -v amneziawg-go &> /dev/null; then
        echo -e "${green}AmneziaWG is already installed${plain}"
        awg --version 2>/dev/null || true
        return 0
    fi
    
    # Create build directory
    mkdir -p /opt/amneziawg-build
    cd /opt/amneziawg-build
    
    # Install amneziawg-go (userspace implementation)
    echo -e "${yellow}Building amneziawg-go...${plain}"
    if [ ! -d "amneziawg-go" ]; then
        git clone https://github.com/amnezia-vpn/amneziawg-go.git
    fi
    cd amneziawg-go
    
    # Build with Go
    export CGO_ENABLED=0
    go build -ldflags="-s -w" -o amneziawg-go
    if [ $? -ne 0 ]; then
        echo -e "${red}Failed to build amneziawg-go${plain}"
        exit 1
    fi
    install -m 755 amneziawg-go ${amneziawg_bin}
    echo -e "${green}amneziawg-go installed to ${amneziawg_bin}${plain}"
    
    # Install amneziawg-tools (awg, awg-quick)
    echo -e "${yellow}Building amneziawg-tools...${plain}"
    cd /opt/amneziawg-build
    if [ ! -d "amneziawg-tools" ]; then
        git clone https://github.com/amnezia-vpn/amneziawg-tools.git
    fi
    cd amneziawg-tools/src
    
    # Build tools
    make
    if [ $? -ne 0 ]; then
        echo -e "${red}Failed to build amneziawg-tools${plain}"
        exit 1
    fi
    make install
    echo -e "${green}amneziawg-tools installed${plain}"
    
    # Verify installation
    cd ~
    if ! command -v awg &> /dev/null; then
        echo -e "${red}awg command not found after installation${plain}"
        exit 1
    fi
    if ! command -v awg-quick &> /dev/null; then
        echo -e "${red}awg-quick command not found after installation${plain}"
        exit 1
    fi
    if [ ! -f "${amneziawg_bin}" ]; then
        echo -e "${red}amneziawg-go binary not found after installation${plain}"
        exit 1
    fi
    
    echo -e "${green}AmneziaWG installed successfully!${plain}"
    echo -e "  ${green}awg: $(command -v awg)${plain}"
    echo -e "  ${green}awg-quick: $(command -v awg-quick)${plain}"
    echo -e "  ${green}amneziawg-go: ${amneziawg_bin}${plain}"
    
    # Cleanup build directory
    rm -rf /opt/amneziawg-build
}

# Generate AmneziaWG server keys using awg
generate_awg_keys() {
    local private_key=$(awg genkey)
    local public_key=$(echo "$private_key" | awg pubkey)
    
    echo "$private_key"
    echo "$public_key"
}

# Generate AmneziaWG 2.0 obfuscation parameters
# Based on: https://docs.amnezia.org/documentation/amnezia-wg/
generate_obfuscation_params() {
    # AmneziaWG 2.0 recommended parameters
    # Jc: Junk packet count (4-12 recommended)
    # Jmin-Jmax: Junk packet size range (64-1024 bytes)
    # S1-S4: Message padding (0-64 for S1-S3, 0-32 for S4)
    # H1-H4: Dynamic message headers (uint32 ranges)
    # I1-I5: Custom signature packets (CPS format)
    
    local Jc=$((RANDOM % 9 + 4))           # 4-12
    local Jmin=$((RANDOM % 64 + 64))       # 64-128
    local Jmax=$((RANDOM % 256 + 768))     # 768-1024
    local S1=$((RANDOM % 65))              # 0-64
    local S2=$((RANDOM % 65))              # 0-64
    local S3=$((RANDOM % 65))              # 0-64
    local S4=$((RANDOM % 33))              # 0-32
    
    # H1-H4: Dynamic headers as ranges (uint32)
    # Format: "min-max" or single value
    local H1_start=$((RANDOM % 1000000))
    local H1_end=$((H1_start + RANDOM % 100000 + 100000))
    local H2_start=$((RANDOM % 1000000))
    local H2_end=$((H2_start + RANDOM % 100000 + 100000))
    local H3_start=$((RANDOM % 1000000))
    local H3_end=$((H3_start + RANDOM % 100000 + 100000))
    local H4_start=$((RANDOM % 1000000))
    local H4_end=$((H4_start + RANDOM % 100000 + 100000))
    
    # I1-I5: Custom signature packets (CPS format)
    # These are optional and used for protocol mimicry
    # Format: <b hex_data><r size><t> etc.
    # For simplicity, we generate basic signatures
    local I1="<b 0xc0000000><r 40><t>"
    local I2="<r 60>"
    local I3=""
    local I4=""
    local I5=""
    
    echo "Jc = ${Jc}"
    echo "Jmin = ${Jmin}"
    echo "Jmax = ${Jmax}"
    echo "S1 = ${S1}"
    echo "S2 = ${S2}"
    echo "S3 = ${S3}"
    echo "S4 = ${S4}"
    echo "H1 = ${H1_start}-${H1_end}"
    echo "H2 = ${H2_start}-${H2_end}"
    echo "H3 = ${H3_start}-${H3_end}"
    echo "H4 = ${H4_start}-${H4_end}"
    # I1-I5 are optional - uncomment if needed
    # echo "I1 = ${I1}"
    # echo "I2 = ${I2}"
}

# Setup AmneziaWG configuration directory
setup_amnezia_config() {
    echo -e "${green}Setting up AmneziaWG configuration...${plain}"
    
    mkdir -p ${amnezia_folder}
    mkdir -p ${amnezia_folder}/peers
    
    # Generate server keys using awg
    local keys=$(generate_awg_keys)
    local private_key=$(echo "$keys" | head -n1)
    local public_key=$(echo "$keys" | tail -n1)
    
    # Generate random server port if not specified
    local wg_port=$(shuf -i 1024-62000 -n 1)
    
    # Generate random network for AmneziaWG
    local wg_network="10.$(shuf -i 0-255 -n 1).$(shuf -i 0-255 -n 1).0"
    
    # Generate obfuscation parameters for AmneziaWG 2.0
    local obf_params=$(generate_obfuscation_params)
    
    # Create server config file
    cat > ${amnezia_folder}/awg0.conf << EOF
# AmneziaWG Server Configuration
# Generated by 3X-UI Amnezia installer
# Interface: awg0

[Interface]
PrivateKey = ${private_key}
Address = ${wg_network}.1/24
ListenPort = ${wg_port}
SaveConfig = false

# AmneziaWG 2.0 Obfuscation Parameters
# These help bypass DPI detection
EOF

    # Add obfuscation parameters
    echo "$obf_params" >> ${amnezia_folder}/awg0.conf
    
    # Save public key for reference
    echo "${public_key}" > ${amnezia_folder}/publickey
    
    # Save port for reference
    echo "${wg_port}" > ${amnezia_folder}/port
    
    # Save network for reference
    echo "${wg_network}" > ${amnezia_folder}/network
    
    # Save private key securely
    echo "${private_key}" > ${amnezia_folder}/privatekey
    chmod 600 ${amnezia_folder}/privatekey
    
    echo -e "${green}AmneziaWG configuration created:${plain}"
    echo -e "  ${green}Interface: awg0${plain}"
    echo -e "  ${green}Port: ${wg_port}${plain}"
    echo -e "  ${green}Network: ${wg_network}.0/24${plain}"
    echo -e "  ${green}Public Key: ${public_key}${plain}"
    echo -e "  ${green}Config: ${amnezia_folder}/awg0.conf${plain}"
    
    chmod 600 ${amnezia_folder}/awg0.conf
}

# Create systemd service for AmneziaWG
create_amnezia_service() {
    echo -e "${green}Creating AmneziaWG systemd service...${plain}"
    
    # Create amneziawg@.service template for multiple interfaces
    cat > /etc/systemd/system/amneziawg@.service << 'EOF'
[Unit]
Description=AmneziaWG interface %i
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Environment=WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go
ExecStart=/bin/sh -c 'exec awg-quick up /etc/amnezia/amneziawg/%i.conf'
ExecStop=/bin/sh -c 'exec awg-quick down /etc/amnezia/amneziawg/%i.conf'
ExecReload=/bin/sh -c 'awg-quick down /etc/amnezia/amneziawg/%i.conf || true; exec awg-quick up /etc/amnezia/amneziawg/%i.conf'

[Install]
WantedBy=multi-user.target
EOF

    chmod 644 /etc/systemd/system/amneziawg@.service
    systemctl daemon-reload
    
    echo -e "${green}AmneziaWG systemd service created${plain}"
    echo -e "  ${green}Service: amneziawg@awg0.service${plain}"
    echo -e "  ${green}Config: ${amnezia_folder}/awg0.conf${plain}"
}

# Verify AmneziaWG installation
verify_amneziawg_installation() {
    echo -e "${green}Verifying AmneziaWG installation...${plain}"
    
    local errors=0
    
    # Check awg binary
    if command -v awg &> /dev/null; then
        echo -e "  ${green}✓ awg: $(command -v awg)${plain}"
    else
        echo -e "  ${red}✗ awg not found${plain}"
        errors=$((errors + 1))
    fi
    
    # Check awg-quick binary
    if command -v awg-quick &> /dev/null; then
        echo -e "  ${green}✓ awg-quick: $(command -v awg-quick)${plain}"
    else
        echo -e "  ${red}✗ awg-quick not found${plain}"
        errors=$((errors + 1))
    fi
    
    # Check amneziawg-go binary
    if [ -f "${amneziawg_bin}" ]; then
        echo -e "  ${green}✓ amneziawg-go: ${amneziawg_bin}${plain}"
    else
        echo -e "  ${red}✗ amneziawg-go not found${plain}"
        errors=$((errors + 1))
    fi
    
    # Check config file
    if [ -f "${amnezia_folder}/awg0.conf" ]; then
        echo -e "  ${green}✓ Config: ${amnezia_folder}/awg0.conf${plain}"
    else
        echo -e "  ${red}✗ Config file not found${plain}"
        errors=$((errors + 1))
    fi
    
    # Check systemd service
    if [ -f "/etc/systemd/system/amneziawg@.service" ]; then
        echo -e "  ${green}✓ Service: /etc/systemd/system/amneziawg@.service${plain}"
    else
        echo -e "  ${red}✗ Service file not found${plain}"
        errors=$((errors + 1))
    fi
    
    if [ $errors -gt 0 ]; then
        echo -e "${red}AmneziaWG installation verification failed with ${errors} errors${plain}"
        return 1
    fi
    
    echo -e "${green}AmneziaWG installation verified successfully!${plain}"
    return 0
}

setup_ssl_certificate() {
    local domain="$1"
    local server_ip="$2"
    local existing_port="$3"
    local existing_webBasePath="$4"

    echo -e "${green}Setting up SSL certificate...${plain}"

    # Check if acme.sh is installed
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${yellow}Failed to install acme.sh, skipping SSL setup${plain}"
            return 1
        fi
    fi

    # Create certificate directory
    local certPath="/root/cert/${domain}"
    mkdir -p "$certPath"

    # Issue certificate
    echo -e "${green}Issuing SSL certificate for ${domain}...${plain}"
    echo -e "${yellow}Note: Port 80 must be open and accessible from the internet${plain}"

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport 80 --force

    if [ $? -ne 0 ]; then
        echo -e "${yellow}Failed to issue certificate for ${domain}${plain}"
        echo -e "${yellow}Please ensure port 80 is open and try again later with: x-ui${plain}"
        rm -rf ~/.acme.sh/${domain} 2> /dev/null
        rm -rf "$certPath" 2> /dev/null
        return 1
    fi

    # Install certificate
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem \
        --reloadcmd "systemctl restart x-ui" > /dev/null 2>&1

    if [ $? -ne 0 ]; then
        echo -e "${yellow}Failed to install certificate${plain}"
        return 1
    fi

    # Enable auto-renew
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1
    # Secure permissions: private key readable only by owner
    chmod 600 $certPath/privkey.pem 2> /dev/null
    chmod 644 $certPath/fullchain.pem 2> /dev/null

    # Set certificate for panel
    local webCertFile="/root/cert/${domain}/fullchain.pem"
    local webKeyFile="/root/cert/${domain}/privkey.pem"

    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
        ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile" > /dev/null 2>&1
        echo -e "${green}SSL certificate installed and configured successfully!${plain}"
        return 0
    else
        echo -e "${yellow}Certificate files not found${plain}"
        return 1
    fi
}

# Issue Let's Encrypt IP certificate with shortlived profile (~6 days validity)
setup_ip_certificate() {
    local ipv4="$1"
    local ipv6="$2" # optional

    echo -e "${green}Setting up Let's Encrypt IP certificate (shortlived profile)...${plain}"
    echo -e "${yellow}Note: IP certificates are valid for ~6 days and will auto-renew.${plain}"
    echo -e "${yellow}Default listener is port 80. If you choose another port, ensure external port 80 forwards to it.${plain}"

    # Check for acme.sh
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${red}Failed to install acme.sh${plain}"
            return 1
        fi
    fi

    # Validate IP address
    if [[ -z "$ipv4" ]]; then
        echo -e "${red}IPv4 address is required${plain}"
        return 1
    fi

    if ! is_ipv4 "$ipv4"; then
        echo -e "${red}Invalid IPv4 address: $ipv4${plain}"
        return 1
    fi

    # Create certificate directory
    local certDir="/root/cert/ip"
    mkdir -p "$certDir"

    # Build domain arguments
    local domain_args="-d ${ipv4}"
    if [[ -n "$ipv6" ]] && is_ipv6 "$ipv6"; then
        domain_args="${domain_args} -d ${ipv6}"
        echo -e "${green}Including IPv6 address: ${ipv6}${plain}"
    fi

    # Set reload command for auto-renewal
    local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null || true"

    # Choose port for HTTP-01 listener (default 80, prompt override)
    local WebPort="80"
    echo -e "${green}Using port ${WebPort} for standalone validation.${plain}"

    # Ensure chosen port is available
    while true; do
        if is_port_in_use "${WebPort}"; then
            echo -e "${yellow}Port ${WebPort} is in use.${plain}"
            echo -e "${yellow}Stopping x-ui temporarily...${plain}"
            systemctl stop x-ui 2>/dev/null || rc-service x-ui stop 2>/dev/null || true
            sleep 2
            if is_port_in_use "${WebPort}"; then
                echo -e "${red}Port ${WebPort} is still busy; cannot proceed.${plain}"
                return 1
            fi
        fi
        break
    done

    # Issue certificate with shortlived profile
    echo -e "${green}Issuing IP certificate for ${ipv4}...${plain}"
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1

    ~/.acme.sh/acme.sh --issue \
        ${domain_args} \
        --standalone \
        --server letsencrypt \
        --certificate-profile shortlived \
        --days 6 \
        --httpport ${WebPort} \
        --force

    if [ $? -ne 0 ]; then
        echo -e "${red}Failed to issue IP certificate${plain}"
        echo -e "${yellow}Please ensure port ${WebPort} is reachable (or forwarded from external port 80)${plain}"
        # Cleanup
        rm -rf ~/.acme.sh/${ipv4} 2> /dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} 2> /dev/null
        rm -rf ${certDir} 2> /dev/null
        return 1
    fi

    echo -e "${green}Certificate issued successfully, installing...${plain}"

    # Install certificate
    ~/.acme.sh/acme.sh --installcert -d ${ipv4} \
        --key-file "${certDir}/privkey.pem" \
        --fullchain-file "${certDir}/fullchain.pem" \
        --reloadcmd "${reloadCmd}" 2>&1 || true

    # Verify certificate files exist
    if [[ ! -f "${certDir}/fullchain.pem" || ! -f "${certDir}/privkey.pem" ]]; then
        echo -e "${red}Certificate files not found after installation${plain}"
        rm -rf ~/.acme.sh/${ipv4} 2> /dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} 2> /dev/null
        rm -rf ${certDir} 2> /dev/null
        return 1
    fi

    echo -e "${green}Certificate files installed successfully${plain}"

    # Enable auto-upgrade for acme.sh
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1

    # Secure permissions
    chmod 600 ${certDir}/privkey.pem 2> /dev/null
    chmod 644 ${certDir}/fullchain.pem 2> /dev/null

    # Configure panel to use the certificate
    echo -e "${green}Setting certificate paths for the panel...${plain}"
    ${xui_folder}/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem"

    echo -e "${green}IP certificate installed and configured successfully!${plain}"
    echo -e "${green}Certificate valid for ~6 days, auto-renews via acme.sh cron job.${plain}"
    return 0
}

# Reusable interactive SSL setup
prompt_and_setup_ssl() {
    local panel_port="$1"
    local web_base_path="$2"
    local server_ip="$3"

    local ssl_choice=""

    echo -e "${yellow}Choose SSL certificate setup method:${plain}"
    echo -e "${green}1.${plain} Let's Encrypt for Domain (90-day validity, auto-renews)"
    echo -e "${green}2.${plain} Let's Encrypt for IP Address (6-day validity, auto-renews)"
    echo -e "${green}3.${plain} Custom SSL Certificate (Path to existing files)"
    echo -e "${blue}Note:${plain} Options 1 & 2 require port 80 open. Option 3 requires manual paths."
    read -rp "Choose an option (default 2 for IP): " ssl_choice
    ssl_choice="${ssl_choice// /}"

    # Default to 2 (IP cert) if input is empty or invalid
    if [[ "$ssl_choice" != "1" && "$ssl_choice" != "3" ]]; then
        ssl_choice="2"
    fi

    case "$ssl_choice" in
        1)
            # Domain certificate
            echo -e "${green}Using Let's Encrypt for domain certificate...${plain}"
            local domain=""
            while true; do
                read -rp "Please enter your domain name: " domain
                domain="${domain// /}"
                if [[ -z "$domain" ]]; then
                    echo -e "${red}Domain name cannot be empty. Please try again.${plain}"
                    continue
                fi
                if ! is_domain "$domain"; then
                    echo -e "${red}Invalid domain format: ${domain}. Please enter a valid domain name.${plain}"
                    continue
                fi
                break
            done
            
            if setup_ssl_certificate "$domain" "$server_ip" "$panel_port" "$web_base_path"; then
                SSL_HOST="${domain}"
                echo -e "${green}✓ SSL certificate configured successfully with domain: ${domain}${plain}"
            else
                echo -e "${red}SSL certificate setup failed for domain mode.${plain}"
                SSL_HOST="${server_ip}"
            fi
            ;;
        2)
            # IP certificate
            echo -e "${green}Using Let's Encrypt for IP certificate (shortlived profile)...${plain}"

            local ipv6_addr=""
            read -rp "Do you have an IPv6 address to include? (leave empty to skip): " ipv6_addr
            ipv6_addr="${ipv6_addr// /}"

            # Stop panel if running (port 80 needed)
            if [[ $release == "alpine" ]]; then
                rc-service x-ui stop > /dev/null 2>&1
            else
                systemctl stop x-ui > /dev/null 2>&1
            fi

            setup_ip_certificate "${server_ip}" "${ipv6_addr}"
            if [ $? -eq 0 ]; then
                SSL_HOST="${server_ip}"
                echo -e "${green}✓ Let's Encrypt IP certificate configured successfully${plain}"
            else
                echo -e "${red}✗ IP certificate setup failed. Please check port 80 is open.${plain}"
                SSL_HOST="${server_ip}"
            fi
            ;;
        3)
            # Custom certificate
            echo -e "${green}Using custom existing certificate...${plain}"
            local custom_cert=""
            local custom_key=""
            local custom_domain=""

            read -rp "Please enter domain name certificate issued for: " custom_domain
            custom_domain="${custom_domain// /}"

            while true; do
                read -rp "Input certificate path (keywords: .crt / fullchain): " custom_cert
                custom_cert=$(echo "$custom_cert" | tr -d '"' | tr -d "'")
                if [[ -f "$custom_cert" && -r "$custom_cert" && -s "$custom_cert" ]]; then
                    break
                elif [[ ! -f "$custom_cert" ]]; then
                    echo -e "${red}Error: File does not exist! Try again.${plain}"
                elif [[ ! -r "$custom_cert" ]]; then
                    echo -e "${red}Error: File exists but is not readable (check permissions)!${plain}"
                else
                    echo -e "${red}Error: File is empty!${plain}"
                fi
            done

            while true; do
                read -rp "Input private key path (keywords: .key / privatekey): " custom_key
                custom_key=$(echo "$custom_key" | tr -d '"' | tr -d "'")
                if [[ -f "$custom_key" && -r "$custom_key" && -s "$custom_key" ]]; then
                    break
                elif [[ ! -f "$custom_key" ]]; then
                    echo -e "${red}Error: File does not exist! Try again.${plain}"
                elif [[ ! -r "$custom_key" ]]; then
                    echo -e "${red}Error: File exists but is not readable (check permissions)!${plain}"
                else
                    echo -e "${red}Error: File is empty!${plain}"
                fi
            done

            ${xui_folder}/x-ui cert -webCert "$custom_cert" -webCertKey "$custom_key" > /dev/null 2>&1

            if [[ -n "$custom_domain" ]]; then
                SSL_HOST="$custom_domain"
            else
                SSL_HOST="${server_ip}"
            fi

            echo -e "${green}✓ Custom certificate paths applied.${plain}"
            echo -e "${yellow}Note: You are responsible for renewing these files externally.${plain}"

            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            ;;
        *)
            echo -e "${red}Invalid option. Skipping SSL setup.${plain}"
            SSL_HOST="${server_ip}"
            ;;
    esac
}

config_after_install() {
    local existing_hasDefaultCredential=$(${xui_folder}/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    
    local URL_lists=(
        "https://api4.ipify.org"
        "https://ipv4.icanhazip.com"
        "https://v4.api.ipinfo.io/ip"
        "https://ipv4.myexternalip.com/raw"
        "https://4.ident.me"
        "https://check-host.net/ip"
    )
    local server_ip=""
    for ip_address in "${URL_lists[@]}"; do
        local response=$(curl -s -w "\n%{http_code}" --max-time 3 "${ip_address}" 2> /dev/null)
        local http_code=$(echo "$response" | tail -n1)
        local ip_result=$(echo "$response" | head -n-1 | tr -d '[:space:]"')
        if [[ "${http_code}" == "200" && "${ip_result}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            server_ip="${ip_result}"
            break
        fi
    done

    if [[ -z "$server_ip" ]]; then
        echo -e "${yellow}Could not auto-detect server IP from any provider.${plain}"
        while [[ -z "$server_ip" ]]; do
            read -rp "Please enter your server's public IPv4 address: " server_ip
            server_ip="${server_ip// /}"
            if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                echo -e "${red}Invalid IPv4 address. Please try again.${plain}"
                server_ip=""
            fi
        done
    fi

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath=$(gen_random_string 18)
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            read -rp "Would you like to customize the Panel Port settings? (If not, a random port will be applied) [y/n]: " config_confirm
            if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
                read -rp "Please set up the panel port: " config_port
                echo -e "${yellow}Your Panel Port is: ${config_port}${plain}"
            else
                local config_port=$(shuf -i 1024-62000 -n 1)
                echo -e "${yellow}Generated random port: ${config_port}${plain}"
            fi

            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"

            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL Certificate Setup (MANDATORY)     ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}For security, SSL certificate is required for all panels.${plain}"
            echo -e "${yellow}Let's Encrypt now supports both domains and IP addresses!${plain}"
            echo ""

            prompt_and_setup_ssl "${config_port}" "${config_webBasePath}" "${server_ip}"

            # Display final credentials and access information
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     Panel Installation Complete!         ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}Username:    ${config_username}${plain}"
            echo -e "${green}Password:    ${config_password}${plain}"
            echo -e "${green}Port:        ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "${green}Access URL:  https://${SSL_HOST}:${config_port}/${config_webBasePath}${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}⚠ IMPORTANT: Save these credentials securely!${plain}"
            echo -e "${yellow}⚠ SSL Certificate: Enabled and configured${plain}"
        else
            local config_webBasePath=$(gen_random_string 18)
            echo -e "${yellow}WebBasePath is missing or too short. Generating a new one...${plain}"
            ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}"
            echo -e "${green}New WebBasePath: ${config_webBasePath}${plain}"

            if [[ -z "${existing_cert}" ]]; then
                echo ""
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${green}     SSL Certificate Setup (RECOMMENDED)   ${plain}"
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${yellow}Let's Encrypt now supports both domains and IP addresses!${plain}"
                echo ""
                prompt_and_setup_ssl "${existing_port}" "${config_webBasePath}" "${server_ip}"
                echo -e "${green}Access URL:  https://${SSL_HOST}:${existing_port}/${config_webBasePath}${plain}"
            else
                echo -e "${green}Access URL: https://${server_ip}:${existing_port}/${config_webBasePath}${plain}"
            fi
        fi
    else
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            echo -e "${yellow}Default credentials detected. Security update required...${plain}"
            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}"
            echo -e "Generated new random login credentials:"
            echo -e "###############################################"
            echo -e "${green}Username: ${config_username}${plain}"
            echo -e "${green}Password: ${config_password}${plain}"
            echo -e "###############################################"
        else
            echo -e "${green}Username, Password, and WebBasePath are properly set.${plain}"
        fi

        existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
        if [[ -z "$existing_cert" ]]; then
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL Certificate Setup (RECOMMENDED)   ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}Let's Encrypt now supports both domains and IP addresses!${plain}"
            echo ""
            prompt_and_setup_ssl "${existing_port}" "${existing_webBasePath}" "${server_ip}"
            echo -e "${green}Access URL:  https://${SSL_HOST}:${existing_port}/${existing_webBasePath}${plain}"
        else
            echo -e "${green}SSL certificate already configured. No action needed.${plain}"
        fi
    fi

    ${xui_folder}/x-ui migrate
}

install_xui() {
    cd ${xui_folder%/x-ui}/

    # Download resources from muvzpro/xui-amnezia repository only
    if [ $# == 0 ]; then
        tag_version=$(curl -Ls "https://api.github.com/repos/muvzpro/xui-amnezia/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${yellow}Trying to fetch version with IPv4...${plain}"
            tag_version=$(curl -4 -Ls "https://api.github.com/repos/muvzpro/xui-amnezia/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        fi
        
        # Use default version if API is unavailable
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${yellow}GitHub API unavailable, using default version v1.1.3${plain}"
            tag_version="v1.1.3"
        fi
        
        echo -e "Got x-ui latest version: ${tag_version}, beginning the installation..."
        
        # Download from muvzpro/xui-amnezia only
        curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz https://github.com/muvzpro/xui-amnezia/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Downloading x-ui failed from muvzpro/xui-amnezia${plain}"
            echo -e "${red}Please ensure your server can access GitHub${plain}"
            echo -e "${yellow}You may need to manually download the release from:${plain}"
            echo -e "${green}https://github.com/muvzpro/xui-amnezia/releases${plain}"
            exit 1
        fi
    else
        tag_version=$1
        url="https://github.com/muvzpro/xui-amnezia/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz"
        echo -e "Beginning to install x-ui $1"
        curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Download x-ui $1 failed from muvzpro/xui-amnezia${plain}"
            exit 1
        fi
    fi
    
    curl -4fLRo /usr/bin/x-ui-temp https://raw.githubusercontent.com/muvzpro/xui-amnezia/master/x-ui.sh
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Failed to download x-ui.sh${plain}"
        exit 1
    fi

    # Stop x-ui service and remove old resources
    if [[ -e ${xui_folder}/ ]]; then
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop
        else
            systemctl stop x-ui
        fi
        rm ${xui_folder}/ -rf
    fi

    # Extract resources and set permissions
    tar zxvf x-ui-linux-$(arch).tar.gz
    rm x-ui-linux-$(arch).tar.gz -f

    cd x-ui
    chmod +x x-ui
    chmod +x x-ui.sh

    # Check the system's architecture and rename the file accordingly
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm
        chmod +x bin/xray-linux-arm
    fi
    chmod +x x-ui bin/xray-linux-$(arch)

    # Update x-ui cli and set permission
    mv -f /usr/bin/x-ui-temp /usr/bin/x-ui
    chmod +x /usr/bin/x-ui
    mkdir -p /var/log/x-ui
    
    # Install AmneziaWG (amneziawg-go and amneziawg-tools)
    install_amneziawg
    
    # Setup AmneziaWG configuration
    setup_amnezia_config
    
    # Create AmneziaWG systemd service
    create_amnezia_service
    
    # Verify AmneziaWG installation
    verify_amneziawg_installation
    
    # Enable and start AmneziaWG service
    echo -e "${green}Enabling AmneziaWG service...${plain}"
    systemctl daemon-reload
    systemctl enable amneziawg@awg0.service
    
    # Start AmneziaWG if config exists
    if [ -f "${amnezia_folder}/awg0.conf" ]; then
        echo -e "${green}Starting AmneziaWG service...${plain}"
        systemctl start amneziawg@awg0.service || echo -e "${yellow}Note: AmneziaWG service may need manual configuration${plain}"
        
        # Verify service is running
        if systemctl is-active --quiet amneziawg@awg0.service; then
            echo -e "${green}✓ AmneziaWG service is running${plain}"
        else
            echo -e "${yellow}⚠ AmneziaWG service not running - check configuration${plain}"
        fi
    fi

    config_after_install

    # Etckeeper compatibility
    if [ -d "/etc/.git" ]; then
        if [ -f "/etc/.gitignore" ]; then
            if ! grep -q "x-ui/x-ui.db" "/etc/.gitignore"; then
                echo "" >> "/etc/.gitignore"
                echo "x-ui/x-ui.db" >> "/etc/.gitignore"
                echo -e "${green}Added x-ui.db to /etc/.gitignore for etckeeper${plain}"
            fi
        else
            echo "x-ui/x-ui.db" > "/etc/.gitignore"
            echo -e "${green}Created /etc/.gitignore and added x-ui.db for etckeeper${plain}"
        fi
    fi

    if [[ $release == "alpine" ]]; then
        curl -4fLRo /etc/init.d/x-ui https://raw.githubusercontent.com/muvzpro/xui-amnezia/master/x-ui.rc
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Failed to download x-ui.rc${plain}"
            exit 1
        fi
        chmod +x /etc/init.d/x-ui
        rc-update add x-ui
        rc-service x-ui start
    else
        # Install systemd service file
        service_installed=false

        if [ -f "x-ui.service" ]; then
            echo -e "${green}Found x-ui.service in extracted files, installing...${plain}"
            cp -f x-ui.service ${xui_service}/ > /dev/null 2>&1
            if [[ $? -eq 0 ]]; then
                service_installed=true
            fi
        fi

        if [ "$service_installed" = false ]; then
            case "${release}" in
                ubuntu | debian | armbian)
                    if [ -f "x-ui.service.debian" ]; then
                        echo -e "${green}Found x-ui.service.debian in extracted files, installing...${plain}"
                        cp -f x-ui.service.debian ${xui_service}/x-ui.service > /dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                    ;;
                arch | manjaro | parch)
                    if [ -f "x-ui.service.arch" ]; then
                        echo -e "${green}Found x-ui.service.arch in extracted files, installing...${plain}"
                        cp -f x-ui.service.arch ${xui_service}/x-ui.service > /dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                    ;;
                *)
                    if [ -f "x-ui.service.rhel" ]; then
                        echo -e "${green}Found x-ui.service.rhel in extracted files, installing...${plain}"
                        cp -f x-ui.service.rhel ${xui_service}/x-ui.service > /dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                    ;;
            esac
        fi

        # If service file not found in tar.gz, download from GitHub
        if [ "$service_installed" = false ]; then
            echo -e "${yellow}Service files not found in tar.gz, downloading from GitHub...${plain}"
            case "${release}" in
                ubuntu | debian | armbian)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/muvzpro/xui-amnezia/master/x-ui.service.debian > /dev/null 2>&1
                    ;;
                arch | manjaro | parch)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/muvzpro/xui-amnezia/master/x-ui.service.arch > /dev/null 2>&1
                    ;;
                *)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/muvzpro/xui-amnezia/master/x-ui.service.rhel > /dev/null 2>&1
                    ;;
            esac

            if [[ $? -ne 0 ]]; then
                echo -e "${red}Failed to install x-ui.service from GitHub${plain}"
                exit 1
            fi
            service_installed=true
        fi

        if [ "$service_installed" = true ]; then
            echo -e "${green}Setting up systemd unit...${plain}"
            chown root:root ${xui_service}/x-ui.service > /dev/null 2>&1
            chmod 644 ${xui_service}/x-ui.service > /dev/null 2>&1
            systemctl daemon-reload
            systemctl enable x-ui
            systemctl start x-ui
        else
            echo -e "${red}Failed to install x-ui.service file${plain}"
            exit 1
        fi
    fi

    echo -e "${green}x-ui ${tag_version}${plain} installation finished, it is running now..."
    echo -e ""
    echo -e "┌───────────────────────────────────────────────────────┐"
    echo -e "│  ${blue}x-ui control menu usages (subcommands):${plain}              │"
    echo -e "│                                                       │"
    echo -e "│  ${blue}x-ui${plain}              - Admin Management Script          │"
    echo -e "│  ${blue}x-ui start${plain}        - Start                            │"
    echo -e "│  ${blue}x-ui stop${plain}         - Stop                             │"
    echo -e "│  ${blue}x-ui restart${plain}      - Restart                          │"
    echo -e "│  ${blue}x-ui status${plain}       - Current Status                   │"
    echo -e "│  ${blue}x-ui settings${plain}     - Current Settings                 │"
    echo -e "│  ${blue}x-ui enable${plain}       - Enable Autostart on OS Startup   │"
    echo -e "│  ${blue}x-ui disable${plain}      - Disable Autostart on OS Startup  │"
    echo -e "│  ${blue}x-ui log${plain}          - Check logs                       │"
    echo -e "│  ${blue}x-ui banlog${plain}       - Check Fail2ban ban logs          │"
    echo -e "│  ${blue}x-ui update${plain}       - Update                           │"
    echo -e "│  ${blue}x-ui legacy${plain}       - Legacy version                   │"
    echo -e "│  ${blue}x-ui install${plain}      - Install                          │"
    echo -e "│  ${blue}x-ui uninstall${plain}    - Uninstall                        │"
    echo -e "│                                                       │"
    echo -e "│  ${green}AmneziaWG Commands:${plain}                                 │"
    echo -e "│  ${blue}systemctl start amnezia${plain}   - Start AmneziaWG          │"
    echo -e "│  ${blue}systemctl stop amnezia${plain}    - Stop AmneziaWG           │"
    echo -e "│  ${blue}systemctl status amnezia${plain}  - AmneziaWG Status         │"
    echo -e "└───────────────────────────────────────────────────────┘"
}

echo -e "${green}Running...${plain}"
install_base
install_xui $1
