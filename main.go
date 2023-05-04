package main

import (
	"context"
	"net/http"
	"os"

	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	_ "github.com/sijms/go-ora/v2"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"

	// Required for debugging
	// _ "net/http/pprof"

	"github.com/iamseth/oracledb_exporter/collector"
)

var (
	// Version will be set at build time.
	Version            = "0.0.0.dev"
	metricPath         = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics. (env: TELEMETRY_PATH)").Default(getEnv("TELEMETRY_PATH", "/metrics")).String()
	defaultFileMetrics = kingpin.Flag("default.metrics", "File with default metrics in a TOML file. (env: DEFAULT_METRICS)").Default(getEnv("DEFAULT_METRICS", "default-metrics.toml")).String()
	customMetrics      = kingpin.Flag("custom.metrics", "File that may contain various custom metrics in a TOML file. (env: CUSTOM_METRICS)").Default(getEnv("CUSTOM_METRICS", "")).String()
	queryTimeout       = kingpin.Flag("query.timeout", "Query timeout (in seconds). (env: QUERY_TIMEOUT)").Default(getEnv("QUERY_TIMEOUT", "5")).Int()
	maxIdleConns       = kingpin.Flag("database.maxIdleConns", "Number of maximum idle connections in the connection pool. (env: DATABASE_MAXIDLECONNS)").Default(getEnv("DATABASE_MAXIDLECONNS", "0")).Int()
	maxOpenConns       = kingpin.Flag("database.maxOpenConns", "Number of maximum open connections in the connection pool. (env: DATABASE_MAXOPENCONNS)").Default(getEnv("DATABASE_MAXOPENCONNS", "10")).Int()
	scrapeInterval     = kingpin.Flag("scrape.interval", "Interval between each scrape. Default is to scrape on collect requests").Default("0s").Duration()
	toolkitFlags       = webflag.AddFlags(kingpin.CommandLine, ":9161")
)

func main() {
	promLogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promLogConfig)
	kingpin.HelpFlag.Short('\n')
	kingpin.Version(version.Print("oracledb_exporter"))
	kingpin.Parse()
	logger := promlog.New(promLogConfig)
	dsn := os.Getenv("DATA_SOURCE_NAME")

	config := &collector.Config{
		DSN:           dsn,
		MaxOpenConns:  *maxOpenConns,
		MaxIdleConns:  *maxIdleConns,
		CustomMetrics: *customMetrics,
		QueryTimeout:  *queryTimeout,
	}
	exporter, err := collector.NewExporter(logger, config)
	if err != nil {
		level.Error(logger).Log("unable to connect to DB", err)
	}

	if *scrapeInterval != 0 {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go exporter.RunScheduledScrapes(ctx, *scrapeInterval)
	}

	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("oracledb_exporter"))

	level.Info(logger).Log("msg", "Starting oracledb_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build", version.BuildContext())
	level.Info(logger).Log("msg", "Collect from: ", "metricPath", *metricPath)

	opts := promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	}
	http.Handle(*metricPath, promhttp.HandlerFor(prometheus.DefaultGatherer, opts))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><head><title>Oracle DB Exporter " + Version + "</title></head><body><h1>Oracle DB Exporter " + Version + "</h1><p><a href='" + *metricPath + "'>Metrics</a></p></body></html>"))
	})

	server := &http.Server{}
	if err := web.ListenAndServe(server, toolkitFlags, logger); err != nil {
		level.Error(logger).Log("msg", "Listening error", "reason", err)
		os.Exit(1)
	}
}

// getEnv returns the value of an environment variable, or returns the provided fallback value
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
