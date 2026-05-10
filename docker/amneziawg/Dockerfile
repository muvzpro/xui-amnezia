FROM golang:1.24.4-alpine AS builder

RUN apk add --no-cache git
WORKDIR /src
RUN git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-go.git .
RUN go mod download \
  && go mod verify \
  && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/amneziawg-go

FROM alpine:3.19

ARG AWGTOOLS_RELEASE="1.0.20250901"

RUN apk add --no-cache bash curl iproute2 iptables iptables-legacy unzip \
  && cd /usr/bin \
  && curl -fL -o amneziawg-tools.zip "https://github.com/amnezia-vpn/amneziawg-tools/releases/download/v${AWGTOOLS_RELEASE}/alpine-3.19-amneziawg-tools.zip" \
  && unzip -j amneziawg-tools.zip \
  && rm -f amneziawg-tools.zip \
  && chmod +x /usr/bin/awg /usr/bin/awg-quick \
  && ln -sf /usr/bin/awg /usr/bin/wg \
  && ln -sf /usr/bin/awg-quick /usr/bin/wg-quick \
  && ln -sf /usr/sbin/iptables-legacy /usr/sbin/iptables 2>/dev/null || true

COPY --from=builder /out/amneziawg-go /usr/bin/amneziawg-go

ENV WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go
WORKDIR /etc/amnezia/amneziawg
