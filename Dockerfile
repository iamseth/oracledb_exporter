FROM ubuntu:18.04
MAINTAINER Seth Miller <seth@sethmiller.me>

RUN apt-get -qq update && \
    apt-get install --no-install-recommends -qq libaio1 rpm wget -y && \
    wget --no-check-certificate https://www.dropbox.com/s/f2ul3y0854y8oqw/oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm && \
    rpm -Uvh --nodeps oracle*rpm && \
    rm -f oracle*rpm

ENV LD_LIBRARY_PATH /usr/lib/oracle/12.2/client64/lib

ADD ./dist/oracledb_exporter.linux-* /oracledb_exporter

RUN chmod 755 /oracledb_exporter

EXPOSE 9161

ENTRYPOINT ["/oracledb_exporter"]
