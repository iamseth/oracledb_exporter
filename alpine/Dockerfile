FROM golang:1.11 AS build

ENV LD_LIBRARY_PATH /usr/lib/oracle/18.3/client64/lib

RUN apt-get -qq update && apt-get install --no-install-recommends -qq libaio1 rpm
COPY oci8.pc /usr/share/pkgconfig
COPY *.rpm /
RUN rpm -Uh --nodeps /oracle-instantclient*.x86_64.rpm && rm /*.rpm
RUN echo $LD_LIBRARY_PATH >> /etc/ld.so.conf.d/oracle.conf && ldconfig

WORKDIR /go/src/oracledb_exporter
COPY . .
RUN go get -d -v

ARG VERSION
ENV VERSION ${VERSION:-0.1.0}

ENV PKG_CONFIG_PATH /go/src/oracledb_exporter
ENV GOOS            linux

RUN go build -v -ldflags "-X main.Version=${VERSION} -s -w"

FROM alpine:3.9
LABEL authors="Seth Miller,Yannig Perré"
LABEL maintainer="Yannig Perré <yannig.perre@gmail.com>"

ENV VERSION ${VERSION:-0.1.0}

COPY oracle-instantclient*basic*.rpm /

RUN apk add libaio rpm && rpm -Uh --nodeps /oracle*rpm && rm -f /oracle*rpm

ENV LD_LIBRARY_PATH /usr/lib/oracle/18.3/client64/lib
RUN ldconfig $LD_LIBRARY_PATH

COPY --from=build /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
ADD ./default-metrics.toml /default-metrics.toml

ENV DATA_SOURCE_NAME system/oracle@oracle/xe

RUN cp -rp /usr/lib/oracle/18.3/client64/lib/* /usr/lib && chmod 755 /oracledb_exporter

EXPOSE 9161

ENTRYPOINT ["/oracledb_exporter"]
