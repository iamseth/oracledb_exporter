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
  "fmt"

  //Required for debugging
  //_ "net/http/pprof"
)

var (
  // Version will be set at build time.
  Version       = "0.0.0.dev"
  listenAddress = flag.String("web.listen-address", getEnv("LISTEN_ADDRESS", ":9161"), "Address to listen on for web interface and telemetry. (env: LISTEN_ADDRESS)")
  metricPath    = flag.String("web.telemetry-path", getEnv("TELEMETRY_PATH", "/metrics"), "Path under which to expose metrics. (env: TELEMETRY_PATH)")
  landingPage   = []byte("<html><head><title>Oracle DB Exporter " + Version + "</title></head><body><h1>Oracle DB Exporter " + Version + "</h1><p><a href='" + *metricPath + "'>Metrics</a></p></body></html>")
  defaultFileMetrics = flag.String("default.metrics", getEnv("DEFAULT_METRICS", "default-metrics.toml"), "File with default metrics in a TOML file. (env: DEFAULT_METRICS)")
  customMetrics = flag.String("custom.metrics", getEnv("CUSTOM_METRICS", ""), "File that may contain various custom metrics in a TOML file. (env: CUSTOM_METRICS)")
  queryTimeout  = flag.String("query.timeout", getEnv("QUERY_TIMEOUT", "5"), "Query timeout (in seconds). (env: QUERY_TIMEOUT)")
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
  MetricsType   map[string]string
  FieldToAppend string
  Request       string
  IgnoreZeroResult bool
}

// Used to load multiple metrics from file
type Metrics struct {
  Metric []Metric
}

// Metrics to scrap. Use external file (default-metrics.toml and custom if provided)
var (
  metricsToScrap Metrics
  additionalMetrics Metrics
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

// getEnv returns the value of an environment variable, or returns the provided fallback value
func getEnv(key, fallback string) string {
  if value, ok := os.LookupEnv(key); ok {
    return value
  }
  return fallback
}

// NewExporter returns a new Oracle DB exporter for the provided DSN.
func NewExporter(dsn string) *Exporter {
  db, err := sql.Open("oci8", dsn)
  db.SetMaxIdleConns(0)
  db.SetMaxOpenConns(10)
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

// Describe describes all the metrics exported by the Oracle DB exporter.
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

  if err = e.db.Ping(); err != nil {
    if strings.Contains(err.Error(), "sql: database is closed") {
      log.Infoln("Reconnecting to DB")
      e.db, err = sql.Open("oci8", e.dsn)
      e.db.SetMaxIdleConns(0)
      e.db.SetMaxOpenConns(10)
    }
  }
  if err = e.db.Ping(); err != nil {
    log.Errorln("Error pinging oracle:", err)
    //e.db.Close()
    e.up.Set(0)
    return
  } else {
    e.up.Set(1)
  }

  for _, metric := range metricsToScrap.Metric {
    if err = ScrapeMetric(e.db, ch, metric); err != nil {
      log.Errorln("Error scraping for", metric.Context, ":", err)
      e.scrapeErrors.WithLabelValues(metric.Context).Inc()
    }
  }

}

func GetMetricType(metricType string, metricsType map[string]string) prometheus.ValueType {
  var strToPromType = map[string]prometheus.ValueType{
    "gauge":       prometheus.GaugeValue,
    "counter":     prometheus.CounterValue,
  }

  strType, ok := metricsType[strings.ToLower(metricType)]
  if !ok {
    return prometheus.GaugeValue
  }
  valueType, ok := strToPromType[strings.ToLower(strType)]
  if !ok {
    panic(errors.New("Error while getting prometheus type " + strings.ToLower(strType)))
  }
  return valueType
}

// interface method to call ScrapeGenericValues using Metric struct values
func ScrapeMetric(db *sql.DB, ch chan<- prometheus.Metric, metricDefinition Metric) error {
  return ScrapeGenericValues(db, ch, metricDefinition.Context, metricDefinition.Labels,
                             metricDefinition.MetricsDesc, metricDefinition.MetricsType,
                             metricDefinition.FieldToAppend, metricDefinition.IgnoreZeroResult,
                             metricDefinition.Request)
}

// generic method for retrieving metrics.
func ScrapeGenericValues(db *sql.DB, ch chan<- prometheus.Metric, context string, labels []string,
                         metricsDesc map[string]string, metricsType map[string]string, fieldToAppend string, ignoreZeroResult bool, request string) error {
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
        log.Errorln("Unable to convert current value to float (metric=" + metric +
                    ",metricHelp=" + metricHelp + ",value=<" + row[metric] + ">)")
        continue
      }
      // If metric do not use a field content in metric's name
      if strings.Compare(fieldToAppend, "") == 0 {
        desc := prometheus.NewDesc(
          prometheus.BuildFQName(namespace, context, metric),
          metricHelp,
          labels, nil,
        )
        ch <- prometheus.MustNewConstMetric(desc, GetMetricType(metric, metricsType), value, labelsValues...)
      // If no labels, use metric name
      } else {
        desc := prometheus.NewDesc(
          prometheus.BuildFQName(namespace, context, cleanName(row[fieldToAppend])),
          metricHelp,
          nil, nil,
        )
        ch <- prometheus.MustNewConstMetric(desc, GetMetricType(metric, metricsType), value)
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
      m[strings.ToLower(colName)] = fmt.Sprintf("%v", *val)
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
  // Load default metrics
  if _, err := toml.DecodeFile(*defaultFileMetrics, &metricsToScrap); err != nil {
    log.Errorln(err)
    panic(errors.New("Error while loading " + *defaultFileMetrics))
  }

  // If custom metrics, load it
  if strings.Compare(*customMetrics, "") != 0 {
    if _, err := toml.DecodeFile(*customMetrics, &additionalMetrics); err != nil {
      log.Errorln(err)
      panic(errors.New("Error while loading " + *customMetrics))
    }
    metricsToScrap.Metric = append(metricsToScrap.Metric, additionalMetrics.Metric...)
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
