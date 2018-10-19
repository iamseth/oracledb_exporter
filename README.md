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
- oracledb_sessions_active (deprecated. Use ``sum(oracledb_sessions_activity{status='ACTIVE'})`` instead.)
- oracledb_sessions_inactive (deprecated. Use ``sum(oracledb_sessions_activity{status='INACTIVE'})`` instead.)
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
- oracledb_process_count

# Installation

## Docker

You can run via Docker using an existing image. If you don't already have an Oracle server, you can run one locally in a container and then link the exporter to it.

```bash
docker run -d --name oracle -p 1521:1521 wnameless/oracle-xe-11g:16.04
docker run -d --name oracledb_exporter --link=oracle -p 9161:9161 -e DATA_SOURCE_NAME=system/oracle@oracle/xe iamseth/oracledb_exporter

```

## Binary Release

Pre-compiled versions for Linux 64 bit and Mac OSX 64 bit can be found under [releases](https://github.com/iamseth/oracledb_exporter/releases).

In order to run, you'll need the [Oracle Instant Client Basic](http://www.oracle.com/technetwork/database/features/instant-client/index-097480.html) for your operating system. Only the basic version is required for execution.

# Running

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

# Integration with Grafana

An example Grafana dashboard is available [here](https://grafana.com/dashboards/3333).


# Troubleshooting

## Error scraping for wait_time

If you experience an error `Error scraping for wait_time: sql: Scan error on column index 1: converting driver.Value type string (",01") to a float64: invalid syntax source="main.go:144"` you may need to set the NLS_LANG variable.

```bash

export NLS_LANG=AMERICAN_AMERICA.WE8ISO8859P1
export DATA_SOURCE_NAME=system/oracle@myhost
/path/to/binary -l log.level error -l web.listen-address 9161
```

If using Docker, set the same variable using the -e flag.
