FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/control-plane ./cmd/control-plane

FROM debian:bookworm-slim

ARG XRAY_DOWNLOAD_URL=https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    wget \
    unzip \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "${XRAY_DOWNLOAD_URL}" -o /tmp/xray.zip \
    && unzip /tmp/xray.zip xray -d /usr/local/bin \
    && chmod +x /usr/local/bin/xray \
    && rm -f /tmp/xray.zip

RUN groupadd --system --gid 10001 controlplane \
    && useradd --system --uid 10001 --gid 10001 --create-home --home-dir /home/controlplane controlplane

WORKDIR /srv/control-plane

COPY --from=build /out/control-plane /usr/local/bin/control-plane
COPY migrations /srv/control-plane/migrations

USER 10001:10001

EXPOSE 8080

CMD ["control-plane"]
