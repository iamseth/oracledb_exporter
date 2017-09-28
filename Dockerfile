FROM ubuntu:17.04
MAINTAINER Seth Miller <seth@sethmiller.me>

RUN apt-get -qq update && \
    apt-get install --no-install-recommends -qq libaio1 rpm wget && \
    wget --no-check-certificate https://www.dropbox.com/s/f2ul3y0854y8oqw/oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm && \
    rpm -Uvh --nodeps oracle*rpm && \
    rm -f oracle*rpm && \
    dpkg --purge rpm wget libicu57 libxml2 libarchive13 rpm2cpio rpm-common debugedit libcap2 libdbus-1-3 \
                 libelf1 libgdbm3 libidn11 liblua5.2-0 liblzo2-2 libmagic-mgc libmagic1 libnettle6 libnspr4 \
                 libnss3 libpopt0 librpm3 librpmbuild3 librpmio3 librpmsign3 libperl5.24 perl && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /usr/share/perl && \
    rm -rf /usr/share/doc && \
    rm -rf /usr/share/locale && \
    rm -rf /usr/share/man && \
    rm -rf /usr/lib/oracle/12.2/client64/lib/libociei.so && \
    rm -rf /usr/lib/oracle/12.2/client64/lib/*jar

ENV LD_LIBRARY_PATH /usr/lib/oracle/12.2/client64/lib

ADD ./dist/oracledb_exporter.linux-* /oracledb_exporter

RUN chmod 755 /oracledb_exporter

EXPOSE 9161

ENTRYPOINT ["/oracledb_exporter"]
