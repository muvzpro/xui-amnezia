# 3X-UI with AmneziaWG Support

[English](/README.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui with AmneziaWG" src="./media/3x-ui-light.png">
  </picture>
</p>

[![Release](https://img.shields.io/github/v/release/muvzpro/xui-amnezia.svg)](https://github.com/muvzpro/xui-amnezia/releases)
[![Downloads](https://img.shields.io/github/downloads/muvzpro/xui-amnezia/total.svg)](https://github.com/muvzpro/xui-amnezia/releases/latest)
[![License](https://img.shields.io/badge/license-MPL%202.0-blue.svg)](https://www.mozilla.org/en-US/MPL/2.0/)

**3X-UI with AmneziaWG** — advanced, open-source web-based control panel designed for managing Xray-core server with AmneziaWG support. It offers a user-friendly interface for configuring and monitoring various VPN and proxy protocols.

> [!IMPORTANT]
> This project is only for personal usage, please do not use it for illegal purposes, and please do not use it in a production environment.

## Features

- **Xray Core** - Full Xray functionality preserved from original 3x-ui
- **AmneziaWG** - WireGuard with obfuscation support
- **AmneziaWG 2.0** - Enhanced obfuscation parameters
- **Client Expiry Management** - Automatic peer pausing and extension
- **Web Panel** - Modern Vue.js interface for management
- **Traffic Statistics** - Monitor peer and server traffic

## Quick Start

```bash
bash <(curl -Ls https://raw.githubusercontent.com/muvzpro/xui-amnezia/master/install-3x-ui.sh)
```

## After Installation

### Panel Access

After installation, you'll see:
- Username
- Password
- Port
- WebBasePath
- Access URL (https://...)

### AmneziaWG Commands

```bash
# Start AmneziaWG
systemctl start amnezia

# Stop AmneziaWG
systemctl stop amnezia

# Check status
systemctl status amnezia

# Enable on boot
systemctl enable amnezia

# View logs
journalctl -u amnezia -f
```

### Panel Commands

```bash
x-ui              - Admin Management Script
x-ui start        - Start panel
x-ui stop         - Stop panel
x-ui restart      - Restart panel
x-ui status       - Current Status
x-ui settings     - Current Settings
x-ui enable       - Enable Autostart on OS Startup
x-ui disable      - Disable Autostart on OS Startup
x-ui log          - Check logs
x-ui update       - Update
x-ui uninstall    - Uninstall
```

## Configuration

### AmneziaWG Configuration

Configuration files are located at:
- `/etc/amnezia/amnezia.conf` - Server configuration
- `/etc/amnezia/publickey` - Server public key
- `/etc/amnezia/port` - Server port
- `/etc/amnezia/network` - WireGuard network

## Requirements

- Linux (Ubuntu, Debian, CentOS, Arch, Alpine supported)
- Root access
- Port 80 open for SSL certificate issuance
- WireGuard kernel module support

## Credits

- Original 3x-ui by [MHSanaei](https://github.com/MHSanaei/3x-ui)
- AmneziaWG by [AmneziaVPN](https://github.com/amnezia-vpn/amneziawg-go)
- WireGuard by [WireGuard](https://www.wireguard.com/)

## Support

- GitHub Issues: [muvzpro/xui-amnezia/issues](https://github.com/muvzpro/xui-amnezia/issues)