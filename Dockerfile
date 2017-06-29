FROM centos:7.3.1611
MAINTAINER Seth Miller <seth@sethmiller.me>

ARG VERSION

RUN yum install -y libaio && \
    yum clean all && \
    rpm -Uvh https://www.dropbox.com/s/f2ul3y0854y8oqw/oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm

ADD oci8.pc /usr/share/pkgconfig/oci8.pc
ADD https://github.com/iamseth/oracledb_exporter/releases/download/${VERSION}/oracledb_exporter-${VERSION}.linux.x86_64 /oracledb_exporter
RUN chmod 755 /oracledb_exporter

EXPOSE 9161

ENV LD_LIBRARY_PATH /usr/lib/oracle/12.2/client64/lib
ENTRYPOINT ["/oracledb_exporter"]
