# Oracle DB Exporter

[![Build Status](https://travis-ci.org/iamseth/oracledb_exporter.svg)](https://travis-ci.org/iamseth/oracledb_exporter)
[![GoDoc](https://godoc.org/github.com/iamseth/oracledb_exporter?status.svg)](http://godoc.org/github.com/iamseth/oracledb_exporter)
[![Report card](https://goreportcard.com/badge/github.com/iamseth/oracledb_exporter)](https://goreportcard.com/badge/github.com/iamseth/oracledb_exporter)

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
- oracledb_sessions_active
- oracledb_sessions_inactive
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
- oracledb_tablespace_bytes_free

# Installation

## Docker

You can run via Docker using an existing image. If you don't already have an Oracle server, you can run one locally in a container and then link the exporter to it.

```bash
docker run --name oracle -d -p 8080:8080 -p 1521:1521 sath89/oracle-12c
docker run -d --link=oracle -p 9161:9161 -e DATA_SOURCE_NAME=system/oracle@oracle/xe.oracle.docker iamseth/oracledb_exporter
```

## Binary Release

Pre-compiled versions for Linux 64 bit and Mac OSX 64 bit can be found under [releases](https://github.com/iamseth/oracledb_exporter/releases).

In order to run, you'll need the [Oracle Instant Client Basic](http://www.oracle.com/technetwork/database/features/instant-client/index-097480.html) for your operating system. Only the basic version is required for execution.

# Running the Binary with DATA_SOURCE_NAME variable

Ensure that the environment variable DATA_SOURCE_NAME is set correctly before starting. For Example

```bash
export DATA_SOURCE_NAME=system/oracle@myhost
/path/to/binary -l log.level error -l web.listen-address 9161
```

# Running the Binary with config file

Another way to launch the binary is to use a directory where you store your configuration in **poml** file. If you provide more than one file, this exporter load them all. It's a convenient way to multiplex your Oracle monitoring when you've got more than one Oracle instance on a server.

There's two examples in ``config-example``. In this file, you can provide each values on separate fields like this:

```
user="system"
pass="temppasswd"
host="localhost"
port="1521"
service="XE"
```

**host** and **port** are not required and use default value (localhost and 1521). You can change this default value using --listener-address and --listener-port options.

You can also provide only string field with all informations:

```
string="system/temppasswd@localhost:1521/XE"
```

By default, this exporter add a name label using the filename (without the extension). If you want to change this label, you can use **name** field with the value you wan't:

```
name="DEMO"
```

Here is an example using one config file for **oracledb_sessions_active**:

```
# HELP oracledb_sessions_active Gauge metric with count of sessions marked ACTIVE
# TYPE oracledb_sessions_active gauge
oracledb_sessions_active{name="XE"} 41
```

Same metrics with two config file (XE.poml and DEMO.poml):
```
# HELP oracledb_sessions_active Gauge metric with count of sessions marked ACTIVE
# TYPE oracledb_sessions_active gauge
oracledb_sessions_active{name="DEMO"} 41
oracledb_sessions_active{name="XE"} 31
```

## Usage

```bash
Usage of oracledb_exporter:
  -config-dir string
        Directory where we store config. Each file result in map
  -listener-address string
        Default Oracle listener address (default "localhost")
  -listener-port string
        Default Oracle listener port (default "1521")
  -web.listen-address string
        Address to listen on for web interface and telemetry. (default ":9161")
  -web.telemetry-path string
        Path under which to expose metrics. (default "/metrics")
```

# Integration with Grafana

An example Grafana dashboard is available [here](https://grafana.com/dashboards/3333).
