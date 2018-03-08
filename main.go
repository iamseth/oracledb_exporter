package main

import (
	"database/sql"
	"flag"
	"net/http"
	"os"
	"strings"
	"strconv"
	"time"
	"errors"
	"context"

	"github.com/BurntSushi/toml"

	_ "github.com/mattn/go-oci8"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

var (
	// Version will be set at build time.
	Version       = "0.0.0.dev"
	listenAddress = flag.String("web.listen-address", ":9161", "Address to listen on for web interface and telemetry.")
	metricPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	landingPage   = []byte("<html><head><title>Oracle DB Exporter " + Version + "</title></head><body><h1>Oracle DB Exporter " + Version + "</h1><p><a href='" + *metricPath + "'>Metrics</a></p></body></html>")
	customMetrics = flag.String("custom.metrics", os.Getenv("CUSTOM_METRICS"), "File that may contain various custom metrics in a TOML file.")
	queryTimeout  = flag.String("query.timeout", "5", "Query timeout (in seconds).")
)

// Metric name parts.
const (
	namespace = "oracledb"
	exporter  = "exporter"
)

// Metrics object description
type Metric struct {
	Context       string
	Labels        []string
	MetricsDesc   map[string]string
	FieldToAppend string
	Request       string
	IgnoreZeroResult bool
}

// Used to load multiple metrics from file
type Metrics struct {
	Metric []Metric
}

var (
	additionalMetrics Metrics
	defaultMetrics = []Metric{
		Metric {
			Context: "resource",
			Labels: []string{ "resource_name" },
			MetricsDesc: map[string]string {
				"current_utilization": "Generic counter metric from v$resource_limit view in Oracle (current value).",
				"limit_value": "Generic counter metric from v$resource_limit view in Oracle (limit value).",
			},
			Request: "SELECT resource_name,current_utilization,limit_value FROM v$resource_limit",
		},
		Metric {
			Context: "asm_diskgroup",
			Labels: []string{ "name" },
			MetricsDesc: map[string]string {
				"total": "Total size of ASM disk group.",
				"free": "Free space available on ASM disk group.",
			},
			Request: "SELECT name,total_mb*1024*1024 as total,free_mb*1024*1024 as free FROM v$asm_diskgroup",
			IgnoreZeroResult: true,
		},
		Metric {
			Context: "activity",
			MetricsDesc: map[string]string {
				"value": "Generic counter metric from v$sysstat view in Oracle.",
			},
			FieldToAppend: "name",
			Request: "SELECT name, value FROM v$sysstat WHERE name IN ('parse count (total)', 'execute count', 'user commits', 'user rollbacks')",
		},
		Metric {
			Context: "wait_time",
			MetricsDesc: map[string]string {
				"value": "Generic counter metric from v$waitclassmetric view in Oracle.",
			},
			FieldToAppend: "wait_class",
			Request: `
  SELECT
    n.wait_class as WAIT_CLASS,
    round(m.time_waited/m.INTSIZE_CSEC,3) as VALUE
  FROM
    v$waitclassmetric  m, v$system_wait_class n
  WHERE
    m.wait_class_id=n.wait_class_id AND n.wait_class != 'Idle'`,
		},
		Metric {
			Context: "tablespace",
			Labels: []string{"tablespace", "type"},
			MetricsDesc: map[string]string {
				"bytes":     "Generic counter metric of tablespaces bytes in Oracle.",
				"max_bytes": "Generic counter metric of tablespaces max bytes in Oracle.",
				"free":      "Generic counter metric of tablespaces free bytes in Oracle.",
			},
			Request: `
  SELECT
    Z.name       as tablespace,
    dt.contents  as type,
    Z.bytes      as bytes,
    Z.max_bytes  as max_bytes,
    Z.free_bytes as free
  FROM
  (
    SELECT
      X.name                   as name,
      SUM(nvl(X.free_bytes,0)) as free_bytes,
      SUM(X.bytes)             as bytes,
      SUM(X.max_bytes)         as max_bytes
    FROM
      (
        SELECT
          ddf.tablespace_name as name,
          ddf.status as status,
          ddf.bytes as bytes,
          sum(dfs.bytes) as free_bytes,
          CASE
            WHEN ddf.maxbytes = 0 THEN ddf.bytes
            ELSE ddf.maxbytes
          END as max_bytes
        FROM
          sys.dba_data_files ddf,
          sys.dba_tablespaces dt,
          sys.dba_free_space dfs
        WHERE ddf.tablespace_name = dt.tablespace_name
        AND ddf.file_id = dfs.file_id(+)
        GROUP BY
          ddf.tablespace_name,
          ddf.file_name,
          ddf.status,
          ddf.bytes,
          ddf.maxbytes
      ) X
    GROUP BY X.name
    UNION ALL
    SELECT
      Y.name                   as name,
      MAX(nvl(Y.free_bytes,0)) as free_bytes,
      SUM(Y.bytes)             as bytes,
      SUM(Y.max_bytes)         as max_bytes
    FROM
      (
        SELECT
          dtf.tablespace_name as name,
          dtf.status as status,
          dtf.bytes as bytes,
          (
            SELECT
              ((f.total_blocks - s.tot_used_blocks)*vp.value)
            FROM
              (SELECT tablespace_name, sum(used_blocks) tot_used_blocks FROM gv$sort_segment WHERE  tablespace_name!='DUMMY' GROUP BY tablespace_name) s,
              (SELECT tablespace_name, sum(blocks) total_blocks FROM dba_temp_files where tablespace_name !='DUMMY' GROUP BY tablespace_name) f,
              (SELECT value FROM v$parameter WHERE name = 'db_block_size') vp
            WHERE f.tablespace_name=s.tablespace_name AND f.tablespace_name = dtf.tablespace_name
          ) as free_bytes,
          CASE
            WHEN dtf.maxbytes = 0 THEN dtf.bytes
            ELSE dtf.maxbytes
          END as max_bytes
        FROM
          sys.dba_temp_files dtf
      ) Y
    GROUP BY Y.name
  ) Z, sys.dba_tablespaces dt
  WHERE
    Z.name = dt.tablespace_name`,
		},
	}

)

// Exporter collects Oracle DB metrics. It implements prometheus.Collector.
type Exporter struct {
	dsn             string
	duration, error prometheus.Gauge
	totalScrapes    prometheus.Counter
	scrapeErrors    *prometheus.CounterVec
	up              prometheus.Gauge
	db              *sql.DB

}

// NewExporter returns a new Oracle DB exporter for the provided DSN.
func NewExporter(dsn string) *Exporter {
	db, err := sql.Open("oci8", dsn)
	if err != nil {
		log.Errorln("Error while connecting to", dsn)
		panic(err)
	}
	return &Exporter{
		dsn: dsn,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from Oracle DB.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrapes_total",
			Help:      "Total number of times Oracle DB was scraped for metrics.",
		}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occured scraping a Oracle database.",
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from Oracle DB resulted in an error (1 for error, 0 for success).",
		}),
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the Oracle database server is up.",
		}),
		db: db,
	}
}

// Describe describes all the metrics exported by the MS SQL exporter.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	// We cannot know in advance what metrics the exporter will generate
	// So we use the poor man's describe method: Run a collect
	// and send the descriptors of all the collected metrics. The problem
	// here is that we need to connect to the Oracle DB. If it is currently
	// unavailable, the descriptors will be incomplete. Since this is a
	// stand-alone exporter and not used as a library within other code
	// implementing additional metrics, the worst that can happen is that we
	// don't detect inconsistent metrics created by this exporter
	// itself. Also, a change in the monitored Oracle instance may change the
	// exported metrics during the runtime of the exporter.

	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh

}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)
	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.error
	e.scrapeErrors.Collect(ch)
	ch <- e.up
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	var err error
	defer func(begun time.Time) {
		e.duration.Set(time.Since(begun).Seconds())
		if err == nil {
			e.error.Set(0)
		} else {
			e.error.Set(1)
		}
	}(time.Now())

	// Noop function for simple SELECT 1 FROM DUAL
	noop := func(row map[string]string) error { return nil }
	if err = GeneratePrometheusMetrics(e.db, noop, "SELECT 1 FROM DUAL"); err != nil {
		log.Errorln("Error pinging oracle:", err)
		e.up.Set(0)
		return
	}
	e.up.Set(1)

	for _, metric := range append(defaultMetrics, additionalMetrics.Metric...) {
		ScrapeMetric(e, ch, metric)
	}
	if err = ScrapeSessions(e.db, ch); err != nil {
		log.Errorln("Error scraping for sessions:", err)
		e.scrapeErrors.WithLabelValues("sessions").Inc()
	}

}

// ScrapeSessions collects session metrics from the v$session view.
func ScrapeSessions(db *sql.DB, ch chan<- prometheus.Metric) error {
	activeCount := 0.
	inactiveCount := 0.
	parseSessions := func(row map[string]string) error {
		value, err := strconv.ParseFloat(row["value"], 64)
		status := row["status"]
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "sessions", "activity"),
				"Gauge metric with count of sessions by status and type", []string{"status", "type"}, nil),
			prometheus.GaugeValue,
			value,
			status,
			row["type"],
		)

		// These metrics are deprecated though so as to not break existing monitoring straight away, are included for the next few releases.
		if status == "ACTIVE" {
			activeCount += value
		}

		if status == "INACTIVE" {
			inactiveCount += value
		}
		return nil
	}
	// Retrieve status and type for all sessions.
	if err := GeneratePrometheusMetrics(db, parseSessions, "SELECT status, type, COUNT(*) as value FROM v$session GROUP BY status, type"); err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, "sessions", "active"),
			"Gauge metric with count of sessions marked ACTIVE. DEPRECATED: use sum(oracledb_sessions_activity{status='ACTIVE}) instead.", []string{}, nil),
		prometheus.GaugeValue,
		activeCount,
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, "sessions", "inactive"),
			"Gauge metric with count of sessions marked INACTIVE. DEPRECATED: use sum(oracledb_sessions_activity{status='INACTIVE'}) instead.", []string{}, nil),
		prometheus.GaugeValue,
		inactiveCount,
	)
	return nil
}

// interface method to call ScrapeGenericValues using Metric struct values
func ScrapeMetric(e *Exporter, ch chan<- prometheus.Metric, metric Metric) {
	if err := ScrapeGenericValues(e.db, ch, metric.Context, metric.Labels,
                                metric.MetricsDesc, metric.FieldToAppend,
                                metric.IgnoreZeroResult, metric.Request); err != nil {
		log.Errorln("Error scraping for", metric.Context, ":", err)
		e.scrapeErrors.WithLabelValues(metric.Context).Inc()
	}
	return
}

// generic method for retrieving metrics.
func ScrapeGenericValues(db *sql.DB, ch chan<- prometheus.Metric, context string, labels []string,
                         metricsDesc map[string]string, fieldToAppend string, ignoreZeroResult bool, request string) error {
	metricsCount := 0
	genericParser := func(row map[string]string) error {
		// Construct labels value
		labelsValues := []string{}
		for _, label := range labels {
			labelsValues = append(labelsValues, row[label])
		}
		// Construct Prometheus values to sent back
		for metric, metricHelp := range metricsDesc {
			value, err := strconv.ParseFloat(strings.TrimSpace(row[metric]), 64)
			// If not a float, skip current metric
			if err != nil {
				continue
			}
			// If metric do not use a field content in metric's name
			if strings.Compare(fieldToAppend, "") == 0 {
				desc := prometheus.NewDesc(
					prometheus.BuildFQName(namespace, context, metric),
					metricHelp,
					labels, nil,
				)
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labelsValues...)
			// If no labels, use metric name
			} else {
				desc := prometheus.NewDesc(
					prometheus.BuildFQName(namespace, context, cleanName(row[fieldToAppend])),
					metricHelp,
					nil, nil,
				)
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value)
			}
			metricsCount ++
		}
		return nil
	}
	err := GeneratePrometheusMetrics(db, genericParser, request)
	if err != nil {
		return err
	}
	if !ignoreZeroResult && metricsCount == 0 {
		return errors.New("No metrics found while parsing")
	}
	return err
}

// inspired by https://kylewbanks.com/blog/query-result-to-map-in-golang
// Parse SQL result and call parsing function to each row
func GeneratePrometheusMetrics(db *sql.DB, parse func(row map[string]string) error, query string) error {

	// Add a timeout
	timeout, err := strconv.Atoi(*queryTimeout)
	if err != nil {
		log.Fatal("error while converting timeout option value: ", err)
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, query)

	if ctx.Err() == context.DeadlineExceeded {
		return errors.New("Oracle query timed out")
	}

	if err != nil {
		return err
	}
	cols, err := rows.Columns()
	defer rows.Close()

	for rows.Next() {
		// Create a slice of interface{}'s to represent each column,
		// and a second slice to contain pointers to each item in the columns slice.
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i, _ := range columns {
			columnPointers[i] = &columns[i]
		}

		// Scan the result into the column pointers...
		if err := rows.Scan(columnPointers...); err != nil {
			return err
		}

		// Create our map, and retrieve the value for each column from the pointers slice,
		// storing it in the map with the name of the column as the key.
		m := make(map[string]string)
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			m[strings.ToLower(colName)] = (*val).(string)
		}
		// Call function to parse row
		if err := parse(m); err != nil {
			return err
		}
	}

	return nil

}

// Oracle gives us some ugly names back. This function cleans things up for Prometheus.
func cleanName(s string) string {
	s = strings.Replace(s, " ", "_", -1) // Remove spaces
	s = strings.Replace(s, "(", "", -1)  // Remove open parenthesis
	s = strings.Replace(s, ")", "", -1)  // Remove close parenthesis
	s = strings.Replace(s, "/", "", -1)  // Remove forward slashes
	s = strings.ToLower(s)
	return s
}

func main() {
	flag.Parse()
	log.Infoln("Starting oracledb_exporter " + Version)
	dsn := os.Getenv("DATA_SOURCE_NAME")
	// If custom metrics, load it
	if strings.Compare(*customMetrics, "") != 0 {
		if _, err := toml.DecodeFile(*customMetrics, &additionalMetrics); err != nil {
			log.Errorln(err)
			panic(errors.New("Error while loading " + *customMetrics))
		}
	}
	exporter := NewExporter(dsn)
	prometheus.MustRegister(exporter)
	http.Handle(*metricPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage)
	})
	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
