# SSTP Egress

Минимальная обёртка над `tiomny/docker-sstp-client`.

Назначение:

- поднять `ppp0`;
- применить policy routing через PPP hooks;
- сохранить published ports на контейнере `sstp-egress`;
- предоставить общий network namespace для `xray-edge` и `mtg-edge`.
