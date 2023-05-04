# Can't use a variable to refer to external image directly with COPY.
# So using image in a step but doing nothing
ARG ORACLE_IMAGE
FROM ${ORACLE_IMAGE} as oracle-image

# Build is starting here
FROM docker.io/library/golang:1.19 AS build

ARG ORACLE_VERSION
ENV ORACLE_VERSION=${ORACLE_VERSION}
ARG MAJOR_VERSION
ENV MAJOR_VERSION=${MAJOR_VERSION}
ENV LD_LIBRARY_PATH "/usr/lib/oracle/${MAJOR_VERSION}/client64/lib"

# Retrieving binaries from oracle image
COPY --from=oracle-image /usr/lib/oracle /usr/lib/oracle
COPY --from=oracle-image /usr/share/oracle /usr/share/oracle
COPY --from=oracle-image /usr/include/oracle /usr/include/oracle

RUN echo $LD_LIBRARY_PATH >> /etc/ld.so.conf.d/oracle.conf && ldconfig

WORKDIR /go/src/oracledb_exporter
COPY . .
RUN go get -d -v

ARG VERSION
ENV VERSION ${VERSION:-0.1.0}

ENV PKG_CONFIG_PATH /go/src/oracledb_exporter

RUN GOOS=linux GOARCH=amd64 go build -v -ldflags "-X main.Version=${VERSION} -s -w"

FROM docker.io/library/ubuntu:22.10
LABEL org.opencontainers.image.authors="Seth Miller,Yannig Perré <yannig.perre@gmail.com>"
LABEL org.opencontainers.image.description="Oracle DB Exporter"

ENV VERSION ${VERSION:-0.1.0}
ENV DEBIAN_FRONTEND=noninteractive

# We only need lib directory
COPY --from=build /usr/lib/oracle /usr/lib/oracle
RUN apt-get -qq update && \
  apt-get -qq install -y --no-install-recommends tzdata libaio1 && \
  rm -rf /var/lib/apt/lists/*

RUN adduser --system --uid 1000 --group appuser \
  && usermod -a -G 0,appuser appuser

ARG ORACLE_VERSION
ENV ORACLE_VERSION=${ORACLE_VERSION}
ARG MAJOR_VERSION
ENV MAJOR_VERSION=${MAJOR_VERSION}
ENV LD_LIBRARY_PATH "/usr/lib/oracle/${MAJOR_VERSION}/client64/lib"
RUN echo $LD_LIBRARY_PATH >> /etc/ld.so.conf.d/oracle.conf && ldconfig

ARG LEGACY_TABLESPACE
ENV LEGACY_TABLESPACE=${LEGACY_TABLESPACE}
COPY --chown=appuser:appuser --from=build /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
ADD ./default-metrics${LEGACY_TABLESPACE}.toml /default-metrics.toml

ENV DATA_SOURCE_NAME system/oracle@oracle/xe

EXPOSE 9161

USER appuser

ENTRYPOINT ["/oracledb_exporter"]