FROM tiomny/docker-sstp-client:latest

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    iproute2 \
    iptables \
    iputils-ping \
    curl \
    ca-certificates \
    procps \
    net-tools \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY infra/sstp-egress/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY infra/sstp-egress/ppp-ip-up.sh /etc/ppp/ip-up.d/20-sstp-policy-routing
COPY infra/sstp-egress/ppp-ip-down.sh /etc/ppp/ip-down.d/20-sstp-policy-routing
COPY infra/sstp-egress/healthcheck.sh /usr/local/bin/sstp-healthcheck

RUN chmod +x /usr/local/bin/entrypoint.sh \
    && chmod +x /usr/local/bin/sstp-healthcheck \
    && chmod +x /etc/ppp/ip-up.d/20-sstp-policy-routing \
    && chmod +x /etc/ppp/ip-down.d/20-sstp-policy-routing

ENTRYPOINT []
CMD ["/usr/local/bin/entrypoint.sh"]
