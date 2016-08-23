# Oracle DB Exporter

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


## Requirements

All requirements may be downloaded from [Oracle](http://www.oracle.com/technetwork/database/features/instant-client/index-097480.html)

### build

To build, you'll need the following packages installed.

- Oracle Instant Client Package - Basic
- Instant Client Package - SQL*Plus
- Instant Client Package - SDK

### installation/running

To run, you'll need the following packages installed.

- Oracle Instant Client Package - Basic
- Instant Client Package - SQL*Plus

## Install

Ensure requirements are met and configure oci8.pc file. See [Oracle driver](https://github.com/mattn/go-oci8) documentation for details. After then, it's just a go get to install.

```bash
go get -u github.com/iamseth/oracledb_exporter
```

## Running

Ensure that the environment variable DATA_SOURCE_NAME is set correctly before starting. For Example

```bash
export DATA_SOURCE_NAME=system/oracle@myhost
/path/to/binary -l log.level error -l web.listen-address 9161
```
## Usage

```bash
Usage of oracledb_exporter:
  -log.format value
       	If set use a syslog logger or JSON logging. Example: logger:syslog?appname=bob&local=7 or logger:stdout?json=true. Defaults to stderr.
  -log.level value
       	Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal].
  -web.listen-address string
       	Address to listen on for web interface and telemetry. (default ":9161")
  -web.telemetry-path string
       	Path under which to expose metrics. (default "/metrics")
```

## Binary releases

Pre-compiled versions may be found in the [release section](https://github.com/iamseth/oracledb_exporter/releases).
