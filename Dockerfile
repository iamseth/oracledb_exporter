FROM golang:1.11 AS build

RUN apt-get -qq update && apt-get install --no-install-recommends -qq libaio1 rpm
COPY *.rpm /
RUN rpm -Uvh --nodeps /oracle-instantclient12.2-devel-12.2.0.1.0-1.x86_64.rpm /oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm && rm /*.rpm
RUN echo $LD_LIBRARY_PATH >> /etc/ld.so.conf.d/oracle.conf && ldconfig

WORKDIR /go/src/oracledb_exporter
COPY . .
RUN go get -d -v

ARG VERSION
ENV VERSION ${VERSION:-0.1.0}

ENV PKG_CONFIG_PATH /go/src/oracledb_exporter
ENV GOOS            linux
ENV LD_LIBRARY_PATH /usr/lib/oracle/12.2/client64/lib

RUN go build -v -ldflags "-X main.Version=${VERSION} -s -w"

FROM ubuntu:18.04
LABEL authors="Seth Miller,Yannig Perré"
LABEL maintainer="Yannig Perré <yannig.perre@gmail.com>"

ENV VERSION ${VERSION:-0.1.0}

COPY oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm /

RUN apt-get -qq update && \
    apt-get install --no-install-recommends -qq libaio1 rpm -y && rpm -Uvh --nodeps /oracle*rpm && \
    rm -f /oracle*rpm

ENV LD_LIBRARY_PATH /usr/lib/oracle/12.2/client64/lib
RUN echo $LD_LIBRARY_PATH >> /etc/ld.so.conf.d/oracle.conf && ldconfig

COPY --from=build /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
ADD ./default-metrics.toml /default-metrics.toml

ENV DATA_SOURCE_NAME system/oracle@oracle/xe

RUN chmod 755 /oracledb_exporter

EXPOSE 9161

ENTRYPOINT ["/oracledb_exporter"]
