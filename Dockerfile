# Build is starting here
FROM docker.io/library/golang:1.19 AS build

WORKDIR /go/src/oracledb_exporter
COPY . .
RUN go get -d -v

ARG VERSION
ENV VERSION ${VERSION:-0.1.0}

RUN GOOS=linux GOARCH=amd64 go build -v -ldflags "-X main.Version=${VERSION} -s -w"

FROM docker.io/library/ubuntu:23.04 as ubuntu
LABEL org.opencontainers.image.authors="Seth Miller,Yannig Perré <yannig.perre@gmail.com>"
LABEL org.opencontainers.image.description="Oracle DB Exporter"

ENV VERSION ${VERSION:-0.1.0}
ENV DEBIAN_FRONTEND=noninteractive

ARG LEGACY_TABLESPACE
ENV LEGACY_TABLESPACE=${LEGACY_TABLESPACE}
COPY --chown=appuser:appuser --from=build /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
ADD ./default-metrics${LEGACY_TABLESPACE}.toml /default-metrics.toml

ENV DATA_SOURCE_NAME system/oracle@oracle/xe

EXPOSE 9161

USER 1000

ENTRYPOINT ["/oracledb_exporter"]

FROM docker.io/library/oraclelinux:8-slim as oracle-linux
LABEL org.opencontainers.image.authors="Seth Miller,Yannig Perré <yannig.perre@gmail.com>"
LABEL org.opencontainers.image.description="Oracle DB Exporter"

ARG LEGACY_TABLESPACE
ENV LEGACY_TABLESPACE=${LEGACY_TABLESPACE}
COPY --from=build /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
ADD ./default-metrics${LEGACY_TABLESPACE}.toml /default-metrics.toml

USER 1000

EXPOSE 9161

ENTRYPOINT ["/oracledb_exporter"]

FROM docker.io/library/alpine:3.17 as alpine
LABEL org.opencontainers.image.authors="Seth Miller,Yannig Perré <yannig.perre@gmail.com>"
LABEL org.opencontainers.image.description="Oracle DB Exporter"

COPY --from=build /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
USER 1000

EXPOSE 9161

ENTRYPOINT ["/oracledb_exporter"]
