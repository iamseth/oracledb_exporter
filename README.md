# Oracle DB Exporter

[![Build Status](https://travis-ci.org/iamseth/oracledb_exporter.svg)](https://travis-ci.org/iamseth/oracledb_exporter)
[![GoDoc](https://godoc.org/github.com/iamseth/oracledb_exporter?status.svg)](http://godoc.org/github.com/iamseth/oracledb_exporter)
[![Report card](https://goreportcard.com/badge/github.com/iamseth/oracledb_exporter)](https://goreportcard.com/badge/github.com/iamseth/oracledb_exporter)

##### Table of Contents

[Description](#description)  
[Installation](#installation)  
[Running](#running)  
[Grafana](#grafana)  
[Troubleshooting](#troubleshooting)  
[Operating principles](operating-principles.md)

# Description

A [Prometheus](https://prometheus.io/) exporter for Oracle modeled after the MySQL exporter. I'm not a DBA or seasoned Go developer so PRs definitely welcomed.

The following metrics are exposed currently.

- oracledb_exporter_last_scrape_duration_seconds
- oracledb_exporter_last_scrape_error
- oracledb_exporter_scrapes_total
- oracledb_up
- oracledb_activity_execute_count
- oracledb_activity_parse_count_total
- oracledb_activity_user_commits
- oracledb_activity_user_rollbacks
- oracledb_sessions_activity
- oracledb_wait_time_application
- oracledb_wait_time_commit
- oracledb_wait_time_concurrency
- oracledb_wait_time_configuration
- oracledb_wait_time_network
- oracledb_wait_time_other
- oracledb_wait_time_scheduler
- oracledb_wait_time_system_io
- oracledb_wait_time_user_io
- oracledb_tablespace_bytes
- oracledb_tablespace_max_bytes
- oracledb_tablespace_free
- oracledb_tablespace_used_percent
- oracledb_process_count
- oracledb_resource_current_utilization
- oracledb_resource_limit_value

# Installation

## Docker

You can run via Docker using an existing image. Since version 0.4, the images are available on the github registry.

Here an example to retrieve the version 0.5.0:

```bash
docker pull ghcr.io/iamseth/oracledb_exporter:0.5.0
```

And here a command to run it and forward the port:

```bash
docker run -it --rm -p 9161:9161 ghcr.io/iamseth/oracledb_exporter:0.5.0
```

If you don't already have an Oracle server, you can run one locally in a container and then link the exporter to it.

```bash
docker run -d --name oracle -p 1521:1521 wnameless/oracle-xe-11g-r2:18.04-apex
docker run -d --name oracledb_exporter --link=oracle -p 9161:9161 -e DATA_SOURCE_NAME=oracle://system:oracle@oracle:1521/xe ghcr.io/iamseth/oracledb_exporter:0.5.0
```

Since 0.2.1, the exporter image exist with Alpine flavor. Watch out for their use. It is for the moment a test.

```bash
docker run -d --name oracledb_exporter --link=oracle -p 9161:9161 -e DATA_SOURCE_NAME=oracle://system:oracle@oracle/xe iamseth/oracledb_exporter:alpine
```

### Different Docker Images

Different Linux Distros:

- `x.y.z` - Ubuntu Linux image
- `x.y.z-oraclelinux` - Oracle Enterprise Linux image
- `x.y.z-Alpine` - Alpine Linux image

Forked Version:
All the above docker images have a duplicate image tag ending in
`_legacy-tablespace`. These versions use the older/deprecated tablespace
utilization calculation based on the aggregate sum of file sizes in a given
tablespace. The newer mechanism takes into account block sizes, extents, and
fragmentation aligning with the same metrics reported from the Oracle Enterprise
Manager. See https://github.com/iamseth/oracledb_exporter/issues/153 for
details. The versions above should have a more useful tablespace utilization
calculation going forward.

## Binary Release

Pre-compiled versions for Linux 64 bit and Mac OSX 64 bit can be found under [releases](https://github.com/iamseth/oracledb_exporter/releases).

In order to run, you'll need the [Oracle Instant Client Basic](http://www.oracle.com/technetwork/database/features/instant-client/index-097480.html)
for your operating system. Only the basic version is required for execution.

# Running
Ensure that the environment variable DATA_SOURCE_NAME is set correctly before starting.
DATA_SOURCE_NAME should be in Oracle Database connection string format:  

```conn
    oracle://user:pass@server/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
```

For Example:

```bash
# export Oracle location:
export DATA_SOURCE_NAME=oracle://system:password@oracle-sid
# or using a complete url:
export DATA_SOURCE_NAME=oracle://user:password@myhost:1521/service
# 19c client for primary/standby configuration
export DATA_SOURCE_NAME=oracle://user:password@primaryhost:1521,standbyhost:1521/service
# 19c client for primary/standby configuration with options
export DATA_SOURCE_NAME=oracle://user:password@primaryhost:1521,standbyhost:1521/service?connect_timeout=5&transport_connect_timeout=3&retry_count=3
# 19c client for ASM instance connection (requires SYSDBA)
export DATA_SOURCE_NAME=oracle://user:password@primaryhost:1521,standbyhost:1521/+ASM?as=sysdba
# Then run the exporter
/path/to/binary/oracledb_exporter --log.level error --web.listen-address 0.0.0.0:9161
```
## Default-metrics requirement
Make sure to grant `SYS` privilege on `SELECT` statement for the monitoring user, on the following tables.
```
dba_tablespace_usage_metrics
dba_tablespaces
v$system_wait_class
v$asm_diskgroup_stat
v$datafile
v$sysstat
v$process
v$waitclassmetric
v$session
v$resource_limit
```

# Integration with System D

Create **oracledb_exporter** user with disabled login and **oracledb_exporter** group\
mkdir /etc/oracledb_exporter\
chown root:oracledb_exporter /etc/oracledb_exporter  
chmod 775 /etc/oracledb_exporter  
Put config files to **/etc/oracledb_exporter**  
Put binary to **/usr/local/bin**

Create file **/etc/systemd/system/oracledb_exporter.service** with the following content:

```bash
[Unit]
Description=Service for oracle telemetry client
After=network.target
[Service]
Type=oneshot
#!!! Set your values and uncomment
#User=oracledb_exporter
#Group=oracledb_exporter
#Environment="DATA_SOURCE_NAME=dbsnmp/Bercut01@//primaryhost:1521,standbyhost:1521/myservice?transport_connect_timeout=5&retry_count=3"
#Environment="LD_LIBRARY_PATH=/u01/app/oracle/product/19.0.0/dbhome_1/lib"
#Environment="ORACLE_HOME=/u01/app/oracle/product/19.0.0/dbhome_1"
#Environment="CUSTOM_METRICS=/etc/oracledb_exporter/custom-metrics.toml"
ExecStart=/usr/local/bin/oracledb_exporter  \
  --default.metrics "/etc/oracledb_exporter/default-metrics.toml"  \
  --log.level error --web.listen-address 0.0.0.0:9161
[Install]
WantedBy=multi-user.target
```

Then tell System D to read files:

    systemctl daemon-reload

Start this new service:

    systemctl start oracledb_exporter

Check service status:

    systemctl status oracledb_exporter

## Usage

```bash
Usage of oracledb_exporter:
  --log.format value
       	If set use a syslog logger or JSON logging. Example: logger:syslog?appname=bob&local=7 or logger:stdout?json=true. Defaults to stderr.
  --log.level value
       	Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal].
  --custom.metrics string
        File that may contain various custom metrics in a TOML file.
  --default.metrics string
        Default TOML file metrics.
  --web.systemd-socket
        Use systemd socket activation listeners instead of port listeners (Linux only).
  --web.listen-address string
       	Address to listen on for web interface and telemetry. (default ":9161")
  --web.telemetry-path string
       	Path under which to expose metrics. (default "/metrics")
  --database.maxIdleConns string
        Number of maximum idle connections in the connection pool. (default "0")
  --database.maxOpenConns string
        Number of maximum open connections in the connection pool. (default "10")
  --web.config.file
        Path to configuration file that can enable TLS or authentication.
```

# Default metrics

This exporter comes with a set of default metrics defined in **default-metrics.toml**. You can modify this file or
provide a different one using `default.metrics` option.

# Custom metrics

> NOTE: Do not put a `;` at the end of your SQL queries as this will **NOT** work.

This exporter does not have the metrics you want? You can provide new one using TOML file. To specify this file to the
exporter, you can:

- Use `--custom.metrics` flag followed by the TOML file
- Export CUSTOM_METRICS variable environment (`export CUSTOM_METRICS=my-custom-metrics.toml`)

This file must contain the following elements:

- One or several metric section (`[[metric]]`)
- For each section a context, a request and a map between a field of your request and a comment.

Here's a simple example:

```
[[metric]]
context = "test"
request = "SELECT 1 as value_1, 2 as value_2 FROM DUAL"
metricsdesc = { value_1 = "Simple example returning always 1.", value_2 = "Same but returning always 2." }
```

This file produce the following entries in the exporter:

```
# HELP oracledb_test_value_1 Simple example returning always 1.
# TYPE oracledb_test_value_1 gauge
oracledb_test_value_1 1
# HELP oracledb_test_value_2 Same but returning always 2.
# TYPE oracledb_test_value_2 gauge
oracledb_test_value_2 2
```

You can also provide labels using labels field. Here's an example providing two metrics, with and without labels:

```
[[metric]]
context = "context_no_label"
request = "SELECT 1 as value_1, 2 as value_2 FROM DUAL"
metricsdesc = { value_1 = "Simple example returning always 1.", value_2 = "Same but returning always 2." }

[[metric]]
context = "context_with_labels"
labels = [ "label_1", "label_2" ]
request = "SELECT 1 as value_1, 2 as value_2, 'First label' as label_1, 'Second label' as label_2 FROM DUAL"
metricsdesc = { value_1 = "Simple example returning always 1.", value_2 = "Same but returning always 2." }
```

This TOML file produce the following result:

```
# HELP oracledb_context_no_label_value_1 Simple example returning always 1.
# TYPE oracledb_context_no_label_value_1 gauge
oracledb_context_no_label_value_1 1
# HELP oracledb_context_no_label_value_2 Same but returning always 2.
# TYPE oracledb_context_no_label_value_2 gauge
oracledb_context_no_label_value_2 2
# HELP oracledb_context_with_labels_value_1 Simple example returning always 1.
# TYPE oracledb_context_with_labels_value_1 gauge
oracledb_context_with_labels_value_1{label_1="First label",label_2="Second label"} 1
# HELP oracledb_context_with_labels_value_2 Same but returning always 2.
# TYPE oracledb_context_with_labels_value_2 gauge
oracledb_context_with_labels_value_2{label_1="First label",label_2="Second label"} 2
```

Last, you can set metric type using **metricstype** field.

```
[[metric]]
context = "context_with_labels"
labels = [ "label_1", "label_2" ]
request = "SELECT 1 as value_1, 2 as value_2, 'First label' as label_1, 'Second label' as label_2 FROM DUAL"
metricsdesc = { value_1 = "Simple example returning always 1 as counter.", value_2 = "Same but returning always 2 as gauge." }
# Can be counter or gauge (default)
metricstype = { value_1 = "counter" }
```

This TOML file will produce the following result:

```
# HELP oracledb_test_value_1 Simple test example returning always 1 as counter.
# TYPE oracledb_test_value_1 counter
oracledb_test_value_1 1
# HELP oracledb_test_value_2 Same test but returning always 2 as gauge.
# TYPE oracledb_test_value_2 gauge
oracledb_test_value_2 2
```

You can find [here](./custom-metrics-example/custom-metrics.toml) a working example of custom metrics for slow queries, big queries and top 100 tables.

# Customize metrics in a docker image

If you run the exporter as a docker image and want to customize the metrics, you can use the following example:

```Dockerfile
FROM iamseth/oracledb_exporter:latest

COPY custom-metrics.toml /

ENTRYPOINT ["/oracledb_exporter", "--custom.metrics", "/custom-metrics.toml"]
```

# Using a multiple host data source name

> NOTE: This has been tested with v0.2.6a and will most probably work on versions above.

> NOTE: While `user/password@//database1.example.com:1521,database3.example.com:1521/DBPRIM` works with SQLPlus, it doesn't seem to work with oracledb-exporter v0.2.6a.

In some cases, one might want to scrape metrics from the currently available database when having a active-passive replication setup.

This will try to connect to any available database to scrape for the metrics. With some replication options, the secondary database is not available when replicating. This allows the scraper to automatically fall back in case of the primary one failing.

This example allows to achieve this:

### Files & Folder:

- tns_admin folder: `/path/to/tns_admin`
- tnsnames.ora file: `/path/to/tns_admin/tnsnames.ora`

Example of a tnsnames.ora file:

```
database =
(DESCRIPTION =
  (ADDRESS_LIST =
    (ADDRESS = (PROTOCOL = TCP)(HOST = database1.example.com)(PORT = 1521))
    (ADDRESS = (PROTOCOL = TCP)(HOST = database2.example.com)(PORT = 1521))
  )
  (CONNECT_DATA =
    (SERVICE_NAME = DBPRIM)
  )
)
```

### Environment Variables

- `TNS_ENTRY`: Name of the entry to use (`database` in the example file above)
- `TNS_ADMIN`: Path you choose for the tns admin folder (`/path/to/tns_admin` in the example file above)
- `DATA_SOURCE_NAME`: Datasource pointing to the `TNS_ENTRY` (`user:password@database` in the example file above)

# TLS connection to database

First, set the following variables:

    export WALLET_PATH=/wallet/path/to/use
    export TNS_ENTRY=tns_entry
    export DB_USERNAME=db_username
    export TNS_ADMIN=/tns/admin/path/to/use

Create the wallet and set the credential:

    mkstore -wrl $WALLET_PATH -create
    mkstore -wrl $WALLET_PATH -createCredential $TNS_ENTRY $DB_USERNAME

Then, update sqlnet.ora:

    echo "
    WALLET_LOCATION = (SOURCE = (METHOD = FILE) (METHOD_DATA = (DIRECTORY = $WALLET_PATH )))
    SQLNET.WALLET_OVERRIDE = TRUE
    SSL_CLIENT_AUTHENTICATION = FALSE
    " >> $TNS_ADMIN/sqlnet.ora

To use the wallet, use the wallet_location parameter. You may need to disable ssl verification with the
ssl_server_dn_match parameter.

Here a complete example of string connection:

    DATA_SOURCE_NAME=oracle://username:password@server:port/service?ssl_server_dn_match=false&wallet_location=wallet_path

For more details, have a look at the following location: https://github.com/iamseth/oracledb_exporter/issues/84

# Integration with Grafana

An example Grafana dashboard is available [here](https://grafana.com/grafana/dashboards/3333-oracledb/).

# Build

## Docker build

To build Ubuntu and Alpine image, run the following command:

    make docker

You can also build only Ubuntu image:

    make ubuntu-image

Or Alpine:

    make alpine-image

## Building Binaries

Run build:

```sh
    make go-build
```

will output binaries and archive inside the `dist` folder for the building operating system.

## Import into your Golang Application

The `oracledb_exporter` can also be imported into your Go based applications. The [Grafana Agent](https://github.com/grafana/agent/) uses this pattern to implement the [OracleDB integration](https://grafana.com/docs/grafana-cloud/data-configuration/integrations/integration-reference/integration-oracledb/). Feel free to modify the code to fit your application's use case.

Here is a small snippet of an example usage of the exporter in code:

```go
 promLogConfig := &promlog.Config{}
 // create your own config
 logger := promlog.New(promLogConfig)

 // replace with your connection string
 connectionString := "oracle://username:password@localhost:1521/orcl.localnet"
 oeExporter, err := oe.NewExporter(logger, &oe.Config{
  DSN:          connectionString,
  MaxIdleConns: 0,
  MaxOpenConns: 10,
  QueryTimeout: 5,
 })

 if err != nil {
  panic(err)
 }

 metricChan := make(chan prometheus.Metric, len(oeExporter.DefaultMetrics().Metric))
 oeExporter.Collect(metricChan)

 // alternatively its possible to run scrapes on an interval
 // and Collect() calls will only return updated data once
 // that intervaled scrape is run
 // please note this is a blocking call so feel free to run
 // in a separate goroutine
 // oeExporter.RunScheduledScrapes(context.Background(), time.Minute)

 for r := range metricChan {
  // Write to the client of your choice
  // or spin up a promhttp.Server to serve these metrics
  r.Write(&dto.Metric{})
 }

```

# FAQ/Troubleshooting

## Unable to convert current value to float (metric=par,metri...in.go:285

Oracle is trying to send a value that we cannot convert to float. This could be anything like 'UNLIMITED' or 'UNDEFINED' or 'WHATEVER'.

In this case, you must handle this problem by testing it in the SQL request. Here an example available in default metrics:

```toml
[[metric]]
context = "resource"
labels = [ "resource_name" ]
metricsdesc = { current_utilization= "Generic counter metric from v$resource_limit view in Oracle (current value).", limit_value="Generic counter metric from v$resource_limit view in Oracle (UNLIMITED: -1)." }
request="SELECT resource_name,current_utilization,CASE WHEN TRIM(limit_value) LIKE 'UNLIMITED' THEN '-1' ELSE TRIM(limit_value) END as limit_value FROM v$resource_limit"
```

If the value of limite_value is 'UNLIMITED', the request send back the value -1.

You can increase the log level (`--log.level debug`) in order to get the statement generating this error.

## error while loading shared libraries: libclntsh.so.xx.x: cannot open shared object file: No such file or directory

This exporter use libs from Oracle in order to connect to Oracle Database. If you are running the binary version, you
must install the Oracle binaries somewhere on your machine and **you must install the good version number**. If the
error talk about the version 18.3, you **must** install 18.3 binary version. If it's 12.2, you **must** install 12.2.

An alternative is to run this exporter using a Docker container. This way, you don't have to worry about Oracle binaries
version as they are embedded in the container.

Here an example to run this exporter (to scrap metrics from system/oracle@//host:1521/service-or-sid) and bind the exporter port (9161) to the global machine:

`docker run -it --rm -p 9161:9161 -e DATA_SOURCE_NAME=oracle://system/oracle@//host:1521/service-or-sid iamseth/oracledb_exporter:0.2.6a`

## Error scraping for wait_time

If you experience an error `Error scraping for wait_time: sql: Scan error on column index 1: converting driver.Value type string (",01") to a float64: invalid syntax source="main.go:144"` you may need to set the NLS_LANG variable.

```bash

export NLS_LANG=AMERICAN_AMERICA.WE8ISO8859P1
export DATA_SOURCE_NAME=system/oracle@myhost
/path/to/binary --log.level error --web.listen-address :9161
```

If using Docker, set the same variable using the -e flag.

## An Oracle instance generates a lot of trace files being monitored by exporter

As being said, Oracle instance may (and probably does) generate a lot of trace files alongside its alert log file, one trace file per scraping event. The trace file contains the following lines

```
...
*** MODULE NAME:(prometheus_oracle_exporter-amd64@hostname)
...
kgxgncin: clsssinit: CLSS init failed with status 3
kgxgncin: clsssinit: return status 3 (0 SKGXN not av) from CLSS
```

The root cause is Oracle's reaction of quering ASM-related views without ASM used. The current workaround proposed is to setup a regular task to cleanup these trace files from the filesystem, as example

```
$ find $ORACLE_BASE/diag/rdbms -name '*.tr[cm]' -mtime +14 -delete
```

## TLS and basic authentication

Apache Exporter supports TLS and basic authentication. This enables better
control of the various HTTP endpoints.

To use TLS and/or basic authentication, you need to pass a configuration file
using the `--web.config` parameter. The format of the file is described
[in the exporter-toolkit repository](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md).

Note that the TLS and basic authentication settings affect all HTTP endpoints:
/metrics for scraping, /probe for probing, and the web UI.


## Multi-target support

This exporter supports the multi-target pattern. This allows running a single instance of this exporter for multiple Oracle targets.

To use the multi-target functionality, send a http request to the endpoint `/scrape?target=foo:1521` where target is set to the DSN of the Oracle instance to scrape metrics from.
