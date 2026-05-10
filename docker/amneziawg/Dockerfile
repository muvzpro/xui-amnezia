# Alternative Dockerfile based on official amneziavpn/amneziawg-go image
# This adds iptables-legacy fallback and custom entrypoint support

FROM amneziavpn/amneziawg-go:latest

USER root

# Install iptables-legacy fallback for Alpine compatibility
RUN apk add --no-cache iptables-legacy \
  && ln -sf /usr/sbin/iptables-legacy /usr/sbin/iptables 2>/dev/null || true

# Ensure config directory exists
RUN mkdir -p /etc/amnezia/amneziawg

WORKDIR /etc/amnezia/amneziawg

# Default: keep the original entrypoint from base image
# Override via docker-compose or command line if needed
