ARG SSTP_IMAGE_TAG=latest
ARG SSTP_IMAGE_DIGEST=sha256:4afc497feee97bbcc9f2fe80f86bc086e8d9d9ad4757071928fe6ec8cdbd7fd8

FROM tiomny/docker-sstp-client:${SSTP_IMAGE_TAG}@${SSTP_IMAGE_DIGEST}

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

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY ppp-ip-up.sh /etc/ppp/ip-up.d/20-sstp-policy-routing
COPY ppp-ip-down.sh /etc/ppp/ip-down.d/20-sstp-policy-routing
COPY healthcheck.sh /usr/local/bin/sstp-healthcheck

RUN chmod +x /usr/local/bin/entrypoint.sh \
    && chmod +x /usr/local/bin/sstp-healthcheck \
    && chmod +x /etc/ppp/ip-up.d/20-sstp-policy-routing \
    && chmod +x /etc/ppp/ip-down.d/20-sstp-policy-routing

ENTRYPOINT []
CMD ["/usr/local/bin/entrypoint.sh"]
