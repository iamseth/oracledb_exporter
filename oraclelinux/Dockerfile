FROM golang:1.19 AS build

ARG ORACLE_VERSION
ENV ORACLE_VERSION=${ORACLE_VERSION}
ARG MAJOR_VERSION
ENV MAJOR_VERSION=${MAJOR_VERSION}
ENV LD_LIBRARY_PATH "/usr/lib/oracle/${MAJOR_VERSION}/client64/lib"

RUN apt-get -qq update && apt-get install -y --no-install-recommends -qq libaio1 rpm && rm -rf /var/lib/apt/lists/*
COPY oracle*${ORACLE_VERSION}*.rpm /
RUN rpm -Uh --nodeps /oracle-instantclient*.x86_64.rpm && rm /*.rpm

COPY oci8.pc.template /usr/share/pkgconfig/oci8.pc
RUN sed -i "s/@ORACLE_VERSION@/$ORACLE_VERSION/g" /usr/share/pkgconfig/oci8.pc && \
  sed -i "s/@MAJOR_VERSION@/$MAJOR_VERSION/g" /usr/share/pkgconfig/oci8.pc && \
  find /usr -name oci.pc
RUN echo $LD_LIBRARY_PATH >> /etc/ld.so.conf.d/oracle.conf && ldconfig

WORKDIR /go/src/oracledb_exporter
COPY . .
RUN go get -d -v

ARG VERSION
ENV VERSION ${VERSION:-0.1.0}

ENV PKG_CONFIG_PATH /go/src/oracledb_exporter

RUN GOOS=linux GOARCH=amd64 go build -v -ldflags "-X main.Version=${VERSION} -s -w"

FROM oraclelinux:8-slim

ARG ORACLE_VERSION
ENV ORACLE_VERSION=${ORACLE_VERSION}
ARG MAJOR_VERSION
ENV MAJOR_VERSION=${MAJOR_VERSION}

RUN microdnf -y install oracle-release-el8 && \
  microdnf -y install oracle-instantclient-release-el8 && \
  microdnf -y install oracle-instantclient-basic && \
  microdnf clean all

ARG LEGACY_TABLESPACE
ENV LEGACY_TABLESPACE=${LEGACY_TABLESPACE}
COPY --from=build /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
ADD ./default-metrics${LEGACY_TABLESPACE}.toml /default-metrics.toml

RUN chmod 755 /oracledb_exporter && \
  chmod 644 /default-metrics.toml && \
  groupadd www-data && useradd -g www-data www-data
USER www-data
ENV DATA_SOURCE_NAME system/oracle@oracle/xe
ENV LD_LIBRARY_PATH "/usr/lib/oracle/${MAJOR_VERSION}/client64/lib"

EXPOSE 9161

ENTRYPOINT ["/oracledb_exporter"]
