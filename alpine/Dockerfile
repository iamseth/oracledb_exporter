FROM golang:1.11 AS build

ENV LD_LIBRARY_PATH /usr/lib/oracle/18.3/client64/lib

RUN apt-get -qq update && apt-get install --no-install-recommends -qq libaio1 rpm libgcc1
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

COPY sgerrand.rsa.pub /etc/apk/keys/sgerrand.rsa.pub
COPY glibc-2.29-r0.apk /tmp/glibc-2.29-r0.apk
RUN apk add /tmp/glibc-2.29-r0.apk && rm -f /etc/apk/keys/sgerrand.rsa.pub /tmp/glibc-2.29-r0.apk

ENV LD_LIBRARY_PATH /usr/lib/oracle/18.3/client64/lib

COPY --from=build $LD_LIBRARY_PATH $LD_LIBRARY_PATH
COPY --from=build /go/src/oracledb_exporter/oracledb_exporter /oracledb_exporter
COPY --from=build /lib/x86_64-linux-gnu/libgcc_s.so.1 /usr/glibc-compat/lib
COPY --from=build /lib/x86_64-linux-gnu/libaio.so.1 /usr/glibc-compat/lib
ADD ./default-metrics.toml /default-metrics.toml

ENV DATA_SOURCE_NAME system/oracle@oracle/xe

RUN chmod 755 /oracledb_exporter

EXPOSE 9161

ENTRYPOINT ["/oracledb_exporter"]
