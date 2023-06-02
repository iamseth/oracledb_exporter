FROM golang:1.20.4 as base
ARG BASE_IMAGE
# Build is starting here

WORKDIR /go/src/oracledb_exporter
FROM base as builder
COPY . .
RUN go get -d -v

ARG VERSION
ENV VERSION ${VERSION:-0.1.0}

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -ldflags "-X main.Version=${VERSION} -s -w"

FROM scratch as scratch
ARG LEGACY_TABLESPACE
ENV LEGACY_TABLESPACE=${LEGACY_TABLESPACE}
COPY --from=builder /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
ADD ./default-metrics${LEGACY_TABLESPACE}.toml /default-metrics.toml

ENV DATA_SOURCE_NAME system/oracle@oracle/xe
EXPOSE 9161
USER 1000
ENTRYPOINT ["/oracledb_exporter"]

FROM quay.io/sysdig/sysdig-mini-ubi:1.5.0 as ubi
ARG LEGACY_TABLESPACE
ENV LEGACY_TABLESPACE=${LEGACY_TABLESPACE}
COPY --from=builder /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
ADD ./default-metrics${LEGACY_TABLESPACE}.toml /default-metrics.toml
EXPOSE     9161
USER       1000
ENTRYPOINT [ "/oracledb_exporter" ]