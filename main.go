package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"hash"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/log/level"
	_ "github.com/mattn/go-oci8"

	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"gopkg.in/alecthomas/kingpin.v2"
	//Required for debugging
	//_ "net/http/pprof"
)

var (
	// Version will be set at build time.
	Version            = "0.0.0.dev"
	metricPath         = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics. (env: TELEMETRY_PATH)").Default(getEnv("TELEMETRY_PATH", "/metrics")).String()
	defaultFileMetrics = kingpin.Flag("default.metrics", "File with default metrics in a TOML file. (env: DEFAULT_METRICS)").Default(getEnv("DEFAULT_METRICS", "default-metrics.toml")).String()
	customMetrics      = kingpin.Flag("custom.metrics", "File that may contain various custom metrics in a TOML file. (env: CUSTOM_METRICS)").Default(getEnv("CUSTOM_METRICS", "")).String()
	queryTimeout       = kingpin.Flag("query.timeout", "Query timeout (in seconds). (env: QUERY_TIMEOUT)").Default(getEnv("QUERY_TIMEOUT", "5")).String()
	maxIdleConns       = kingpin.Flag("database.maxIdleConns", "Number of maximum idle connections in the connection pool. (env: DATABASE_MAXIDLECONNS)").Default(getEnv("DATABASE_MAXIDLECONNS", "0")).Int()
	maxOpenConns       = kingpin.Flag("database.maxOpenConns", "Number of maximum open connections in the connection pool. (env: DATABASE_MAXOPENCONNS)").Default(getEnv("DATABASE_MAXOPENCONNS", "10")).Int()
	scrapeInterval     = kingpin.Flag("scrape.interval", "Interval between each scrape. Default is to scrape on collect requests").Default("0s").Duration()
)

// Metric name parts.
const (
	namespace = "oracledb"
	exporter  = "exporter"
)

// Metrics object description
type Metric struct {
	Context          string
	Labels           []string
	MetricsDesc      map[string]string
	MetricsType      map[string]string
	MetricsBuckets   map[string]map[string]string
	FieldToAppend    string
	Request          string
	IgnoreZeroResult bool
}

// Used to load multiple metrics from file
type Metrics struct {
	Metric []Metric
}

// Metrics to scrap. Use external file (default-metrics.toml and custom if provided)
var (
	metricsToScrap    Metrics
	additionalMetrics Metrics
	hashMap           map[int][]byte
)

// Exporter collects Oracle DB metrics. It implements prometheus.Collector.
type Exporter struct {
	dsn             string
	duration, error prometheus.Gauge
	totalScrapes    prometheus.Counter
	scrapeErrors    *prometheus.CounterVec
	scrapeResults   []prometheus.Metric
	up              prometheus.Gauge
	db              *sql.DB
	logger          log.Logger
}

// getEnv returns the value of an environment variable, or returns the provided fallback value
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func atoi(stringValue string, logger log.Logger) int {
	intValue, err := strconv.Atoi(stringValue)
	if err != nil {
		level.Error(logger).Log("msg", "error while converting to int", "err", err)
		panic(err)
	}
	return intValue
}

func maskDsn(dsn string) string {
	parts := strings.Split(dsn, "@")
	if len(parts) > 1 {
		maskedUrl := "***@" + parts[1]
		return maskedUrl
	}
	return dsn
}

func connect(dsn string, logger log.Logger) *sql.DB {
	level.Debug(logger).Log("msg", "Launching connection", "dsn", maskDsn(dsn))
	db, err := sql.Open("oci8", dsn)
	if err != nil {
		level.Error(logger).Log("msg", "Error while connecting to", "dsn", dsn)
		panic(err)
	}
	level.Debug(logger).Log("msg", "set max idle connections to", "value", *maxIdleConns)
	db.SetMaxIdleConns(*maxIdleConns)
	level.Debug(logger).Log("msg", "set max open connections to", "value", *maxOpenConns)
	db.SetMaxOpenConns(*maxOpenConns)
	level.Debug(logger).Log("msg", "Successfully connected to", "dsn", maskDsn(dsn))
	return db
}

// NewExporter returns a new Oracle DB exporter for the provided DSN.
func NewExporter(dsn string, logger log.Logger) *Exporter {
	db := connect(dsn, logger)
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
			Help:      "Total number of times an error occurred scraping a Oracle database.",
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
		db:     db,
		logger: logger,
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
	if *scrapeInterval == 0 { // if we are to scrape when the request is made
		e.scrape(ch)
	} else {
		scrapeResults := e.scrapeResults // There is a risk that e.scrapeResults will be replaced while we traverse this look. This should mitigate that risk
		for idx := range scrapeResults {
			ch <- scrapeResults[idx]
		}
	}
	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.error
	e.scrapeErrors.Collect(ch)
	ch <- e.up
}

func (e *Exporter) runScheduledScrapes() {
	if *scrapeInterval == 0 {
		return // Do nothing as scrapes will be done on Collect requests
	}
	ticker := time.NewTicker(*scrapeInterval)
	defer ticker.Stop()
	for {
		metricCh := make(chan prometheus.Metric, 5)
		go func() {
			scrapeResults := []prometheus.Metric{}
			for {
				scrapeResult, more := <-metricCh
				if more {
					scrapeResults = append(scrapeResults, scrapeResult)
				} else {
					e.scrapeResults = scrapeResults
					return
				}
			}
		}()
		e.scrape(metricCh)
		close(metricCh)
		<-ticker.C
	}
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
			level.Info(e.logger).Log("msg", "Reconnecting to DB")
			e.db = connect(e.dsn, e.logger)
		}
	}
	if err = e.db.Ping(); err != nil {
		level.Error(e.logger).Log("msg", "Error pinging oracle", "err", err)
		//e.db.Close()
		e.up.Set(0)
		return
	} else {
		level.Debug(e.logger).Log("msg", "Successfully pinged Oracle database", "dsn", maskDsn(e.dsn))
		e.up.Set(1)
	}

	if checkIfMetricsChanged(e.logger) {
		reloadMetrics(e.logger)
	}

	wg := sync.WaitGroup{}

	for _, metric := range metricsToScrap.Metric {
		wg.Add(1)
		metric := metric //https://golang.org/doc/faq#closures_and_goroutines

		go func() {
			defer wg.Done()

			level.Debug(e.logger).Log("msg", "About to scrape metric")
			level.Debug(e.logger).Log("metricsDesc", metric.MetricsDesc)
			level.Debug(e.logger).Log("context", metric.Context)
			level.Debug(e.logger).Log("metricsType", metric.MetricsType)
			level.Debug(e.logger).Log("metricsBuckets", metric.MetricsBuckets) // , "(Ignored unless Histogram type)"
			level.Debug(e.logger).Log("labels", metric.Labels)
			level.Debug(e.logger).Log("fieldToAppend", metric.FieldToAppend)
			level.Debug(e.logger).Log("ignoreZeroResult", metric.IgnoreZeroResult)
			level.Debug(e.logger).Log("request", metric.Request)

			if len(metric.Request) == 0 {
				level.Error(e.logger).Log("msg", "Error scraping. Did you forget to define request in your toml file?", "metricsDesc", metric.MetricsDesc)
				return
			}

			if len(metric.MetricsDesc) == 0 {
				level.Error(e.logger).Log("msg", "Error scraping for query. Did you forget to define metricsdesc in your toml file?", "request", metric.Request)
				return
			}

			for column, metricType := range metric.MetricsType {
				if metricType == "histogram" {
					_, ok := metric.MetricsBuckets[column]
					if !ok {
						level.Error(e.logger).Log("msg", "Unable to find MetricsBuckets configuration key for metric", "metric", column)
						return
					}
				}
			}

			scrapeStart := time.Now()
			if err = ScrapeMetric(e.db, ch, metric, e.logger); err != nil {
				level.Error(e.logger).Log("msg", "Error scraping for", "context", metric.Context, "metricsDesc", metric.MetricsDesc, "since", time.Since(scrapeStart), "err", err)
				e.scrapeErrors.WithLabelValues(metric.Context).Inc()
			} else {
				level.Debug(e.logger).Log("msg", "Successfully scraped metric", "context", metric.Context, "metricsDesc", metric.MetricsDesc, "since", time.Since(scrapeStart))
			}
		}()
	}
	wg.Wait()
}

func GetMetricType(metricType string, metricsType map[string]string) prometheus.ValueType {
	var strToPromType = map[string]prometheus.ValueType{
		"gauge":     prometheus.GaugeValue,
		"counter":   prometheus.CounterValue,
		"histogram": prometheus.UntypedValue,
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
func ScrapeMetric(db *sql.DB, ch chan<- prometheus.Metric, metricDefinition Metric, logger log.Logger) error {
	level.Debug(logger).Log("msg", "Calling function ScrapeGenericValues()")
	return ScrapeGenericValues(db, ch, metricDefinition.Context, metricDefinition.Labels,
		metricDefinition.MetricsDesc, metricDefinition.MetricsType, metricDefinition.MetricsBuckets,
		metricDefinition.FieldToAppend, metricDefinition.IgnoreZeroResult,
		metricDefinition.Request, logger)
}

// generic method for retrieving metrics.
func ScrapeGenericValues(db *sql.DB, ch chan<- prometheus.Metric, context string, labels []string,
	metricsDesc map[string]string, metricsType map[string]string, metricsBuckets map[string]map[string]string, fieldToAppend string, ignoreZeroResult bool, request string, logger log.Logger) error {
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
				level.Error(logger).Log("msg", "Unable to convert current value to float", "metric", metric,
					"metricHelp", metricHelp, "value", row[metric])
				continue
			}
			level.Debug(logger).Log("msg", "Query result looks like", "value", value)
			// If metric do not use a field content in metric's name
			if strings.Compare(fieldToAppend, "") == 0 {
				desc := prometheus.NewDesc(
					prometheus.BuildFQName(namespace, context, metric),
					metricHelp,
					labels, nil,
				)
				if metricsType[strings.ToLower(metric)] == "histogram" {
					count, err := strconv.ParseUint(strings.TrimSpace(row["count"]), 10, 64)
					if err != nil {
						level.Error(logger).Log("msg", "Unable to convert count value to int",  "metric", metric,
							"metricHelp", metricHelp, "value", row["count"])
						continue
					}
					buckets := make(map[float64]uint64)
					for field, le := range metricsBuckets[metric] {
						lelimit, err := strconv.ParseFloat(strings.TrimSpace(le), 64)
						if err != nil {
							level.Error(logger).Log("msg", "Unable to convert bucket limit value to float", "metric", metric,
								"metricHelp", metricHelp, ",bucketlimit", le)
							continue
						}
						counter, err := strconv.ParseUint(strings.TrimSpace(row[field]), 10, 64)
						if err != nil {
							level.Error(logger).Log("msg", "Unable to convert value to int", "field", field, "metric", metric,
								"metricHelp", metricHelp, "value", row[field])
							continue
						}
						buckets[lelimit] = counter
					}
					ch <- prometheus.MustNewConstHistogram(desc, count, value, buckets, labelsValues...)
				} else {
					ch <- prometheus.MustNewConstMetric(desc, GetMetricType(metric, metricsType), value, labelsValues...)
				}
				// If no labels, use metric name
			} else {
				desc := prometheus.NewDesc(
					prometheus.BuildFQName(namespace, context, cleanName(row[fieldToAppend])),
					metricHelp,
					nil, nil,
				)
				if metricsType[strings.ToLower(metric)] == "histogram" {
					count, err := strconv.ParseUint(strings.TrimSpace(row["count"]), 10, 64)
					if err != nil {
						level.Error(logger).Log("msg", "Unable to convert count value to int", "metric", metric,
							"metricHelp", metricHelp, "value", row["count"])
						continue
					}
					buckets := make(map[float64]uint64)
					for field, le := range metricsBuckets[metric] {
						lelimit, err := strconv.ParseFloat(strings.TrimSpace(le), 64)
						if err != nil {
							level.Error(logger).Log("msg", "Unable to convert bucket limit value to float", "metric", metric,
								"metricHelp", metricHelp, ",bucketlimit", le)
							continue
						}
						counter, err := strconv.ParseUint(strings.TrimSpace(row[field]), 10, 64)
						if err != nil {
							level.Error(logger).Log("msg", "Unable to convert value to int", "field", field, "metric", metric,
								"metricHelp", metricHelp, "value", row[field])
							continue
						}
						buckets[lelimit] = counter
					}
					ch <- prometheus.MustNewConstHistogram(desc, count, value, buckets)
				} else {
					ch <- prometheus.MustNewConstMetric(desc, GetMetricType(metric, metricsType), value)
				}
			}
			metricsCount++
		}
		return nil
	}
	level.Debug(logger).Log("msg", "Calling function GeneratePrometheusMetrics()")
	err := GeneratePrometheusMetrics(db, genericParser, request, logger)
	level.Debug(logger).Log("msg", "ScrapeGenericValues()", "metricsCount", metricsCount)
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
func GeneratePrometheusMetrics(db *sql.DB, parse func(row map[string]string) error, query string, logger log.Logger) error {

	// Add a timeout
	timeout, err := strconv.Atoi(*queryTimeout)
	if err != nil {
		level.Error(logger).Log("msg", "error while converting timeout option", "err", err)
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
		for i := range columns {
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
	s = strings.Replace(s, "*", "", -1)  // Remove asterisks
	s = strings.ToLower(s)
	return s
}

func hashFile(h hash.Hash, fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	return nil
}

func checkIfMetricsChanged(logger log.Logger) bool {
	for i, _customMetrics := range strings.Split(*customMetrics, ",") {
		if len(_customMetrics) == 0 {
			continue
		}
		level.Debug(logger).Log("msg", "Checking modifications in following metrics definition", "file", _customMetrics)
		h := sha256.New()
		if err := hashFile(h, _customMetrics); err != nil {
			level.Error(logger).Log("msg", "Unable to get file hash", "err", err)
			return false
		}
		// If any of files has been changed reload metrics
		if !bytes.Equal(hashMap[i], h.Sum(nil)) {
			level.Info(logger).Log("msg", "Metrics file has been changed. Reloading...", "file", _customMetrics)
			hashMap[i] = h.Sum(nil)
			return true
		}
	}
	return false
}

func reloadMetrics(logger log.Logger) {
	// Truncate metricsToScrap
	metricsToScrap.Metric = []Metric{}

	// Load default metrics
	if _, err := toml.DecodeFile(*defaultFileMetrics, &metricsToScrap); err != nil {
		level.Error(logger).Log("msg", err)
		panic(errors.New("Error while loading " + *defaultFileMetrics))
	} else {
		level.Info(logger).Log("msg", "Successfully loaded default metrics", "file", *defaultFileMetrics)
	}

	// If custom metrics, load it
	if strings.Compare(*customMetrics, "") != 0 {
		for _, _customMetrics := range strings.Split(*customMetrics, ",") {
			if _, err := toml.DecodeFile(_customMetrics, &additionalMetrics); err != nil {
				level.Error(logger).Log("msg", err)
				panic(errors.New("Error while loading " + _customMetrics))
			} else {
				level.Info(logger).Log("msg", "Successfully loaded custom metrics", "file", _customMetrics)
			}
			metricsToScrap.Metric = append(metricsToScrap.Metric, additionalMetrics.Metric...)
		}
	} else {
		level.Info(logger).Log("msg", "No custom metrics defined")
	}
}

func main() {

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	var toolkitFlags = webflag.AddFlags(kingpin.CommandLine, ":9161")

	kingpin.Version("oracledb_exporter " + Version)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting oracledb_exporter", "version", Version)
	dsn := os.Getenv("DATA_SOURCE_NAME")

	// Load default and custom metrics
	hashMap = make(map[int][]byte)
	reloadMetrics(logger)

	exporter := NewExporter(dsn, logger)
	prometheus.MustRegister(exporter)
	go exporter.runScheduledScrapes()

	// See more info on https://github.com/prometheus/client_golang/blob/master/prometheus/promhttp/http.go#L269
	opts := promhttp.HandlerOpts{

		ErrorHandling: promhttp.ContinueOnError,
	}
	http.Handle(*metricPath, promhttp.HandlerFor(prometheus.DefaultGatherer, opts))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><head><title>Oracle DB Exporter " + Version + "</title></head><body><h1>Oracle DB Exporter " + Version + "</h1><p><a href='" + *metricPath + "'>Metrics</a></p></body></html>"))
	})

	server := &http.Server{}
	if err := web.ListenAndServe(server, toolkitFlags, logger); err != nil {
		level.Error(logger).Log("msg", err)
		os.Exit(1)
	}
}
