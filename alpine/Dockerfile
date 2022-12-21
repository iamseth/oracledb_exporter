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

FROM frolvlad/alpine-glibc:glibc-2.35
LABEL authors="Seth Miller,Yannig Perré"
LABEL maintainer="Yannig Perré <yannig.perre@gmail.com>"

ENV VERSION ${VERSION:-0.1.0}

RUN apk add libaio --no-cache

ARG LEGACY_TABLESPACE
ENV LEGACY_TABLESPACE=${LEGACY_TABLESPACE}
COPY --from=build /usr/lib/oracle /usr/lib/oracle
COPY --from=build /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
ADD ./default-metrics${LEGACY_TABLESPACE}.toml /default-metrics.toml

ARG ORACLE_VERSION
ENV ORACLE_VERSION=${ORACLE_VERSION}
ARG MAJOR_VERSION
ENV MAJOR_VERSION=${MAJOR_VERSION}
ENV LD_LIBRARY_PATH "/usr/lib/oracle/${MAJOR_VERSION}/client64/lib"
RUN ldconfig "$LD_LIBRARY_PATH"

ENV DATA_SOURCE_NAME system/oracle@oracle/xe

RUN chmod 755 /oracledb_exporter

EXPOSE 9161

ENTRYPOINT ["/oracledb_exporter"]