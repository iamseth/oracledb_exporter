package exporter

import (
	"database/sql"
	"strings"
	"time"

	_ "github.com/mattn/go-oci8"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

// Metric name parts.
const (
	namespace = "oracledb"
	exporter  = "exporter"
)

// Exporter collects Oracle DB metrics. It implements prometheus.Collector.
type Exporter struct {
	dsn             string
	labels          prometheus.Labels
	duration, error prometheus.Gauge
	totalScrapes    prometheus.Counter
	scrapeErrors    *prometheus.CounterVec
	up              prometheus.Gauge
}

// NewExporter returns a new Oracle DB exporter for the provided DSN.
func NewExporter(dsn string, name string) *Exporter {
	labels := prometheus.Labels{ "Name": name }
	exporter := &Exporter{
		dsn: dsn,
		labels: labels,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from Oracle DB.",
			ConstLabels: labels,
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrapes_total",
			Help:      "Total number of times Oracle DB was scraped for metrics.",
			ConstLabels: labels,
		}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occured scraping a Oracle database.",
			ConstLabels: labels,
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from Oracle DB resulted in an error (1 for error, 0 for success).",
			ConstLabels: labels,
		}),
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the Oracle database server is up.",
			ConstLabels: labels,
		}),
	}
	prometheus.MustRegister(exporter)
	return exporter
}

// Describe describes all the metrics exported by the Oracle exporter.
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

	db, err := sql.Open("oci8", e.dsn)
	if err != nil {
		log.Errorln("Error opening connection to database:", err)
		return
	}
	defer db.Close()

	isUpRows, err := db.Query("SELECT 1 FROM DUAL")
	if err != nil {
		log.Errorln("Error pinging oracle:", err)
		e.up.Set(0)
		return
	}
	isUpRows.Close()
	e.up.Set(1)

	if err = e.ScrapeActivity(db, ch); err != nil {
		log.Errorln("Error scraping for activity:", err)
		e.scrapeErrors.WithLabelValues("activity").Inc()
	}

	if err = e.ScrapeTablespace(db, ch); err != nil {
		log.Errorln("Error scraping for tablespace:", err)
		e.scrapeErrors.WithLabelValues("tablespace").Inc()
  }

	if err = e.ScrapeWaitTime(db, ch); err != nil {
		log.Errorln("Error scraping for wait_time:", err)
		e.scrapeErrors.WithLabelValues("wait_time").Inc()
	}

	if err = e.ScrapeSessions(db, ch); err != nil {
		log.Errorln("Error scraping for sessions:", err)
		e.scrapeErrors.WithLabelValues("sessions").Inc()
	}
}

// ScrapeSessions collects session metrics from the v$session view.
func (e *Exporter) ScrapeSessions(db *sql.DB, ch chan<- prometheus.Metric) error {
	var err error
	var activeCount float64
	var inactiveCount float64

	// There is probably a better way to do this with a single query. #FIXME when I figure that out.
	err = db.QueryRow("SELECT COUNT(*) FROM v$session WHERE status = 'ACTIVE'").Scan(&activeCount)
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, "sessions", "active"),
			"Gauge metric with count of sessions marked ACTIVE", []string{}, e.labels),
		prometheus.GaugeValue,
		activeCount,
	)

	err = db.QueryRow("SELECT COUNT(*) FROM v$session WHERE status = 'INACTIVE'").Scan(&inactiveCount)
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, "sessions", "inactive"),
			"Gauge metric with count of sessions marked INACTIVE.", []string{}, e.labels),
		prometheus.GaugeValue,
		inactiveCount,
	)

	return nil
}

// ScrapeWaitTime collects wait time metrics from the v$waitclassmetric view.
func (e *Exporter) ScrapeWaitTime(db *sql.DB, ch chan<- prometheus.Metric) error {
	var (
		rows *sql.Rows
		err  error
	)
	rows, err = db.Query("SELECT n.wait_class, round(m.time_waited/m.INTSIZE_CSEC,3) AAS from v$waitclassmetric  m, v$system_wait_class n where m.wait_class_id=n.wait_class_id and n.wait_class != 'Idle'")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			return err
		}
		name = cleanName(name)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "wait_time", name),
				"Generic counter metric from v$waitclassmetric view in Oracle.", []string{}, e.labels),
			prometheus.CounterValue,
			value,
		)
	}
	return nil
}

// ScrapeActivity collects activity metrics from the v$sysstat view.
func (e *Exporter) ScrapeActivity(db *sql.DB, ch chan<- prometheus.Metric) error {
	var (
		rows *sql.Rows
		err  error
	)
	rows, err = db.Query("SELECT name, value FROM v$sysstat WHERE name IN ('parse count (total)', 'execute count', 'user commits', 'user rollbacks')")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			return err
		}
		name = cleanName(name)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "activity", name),
				"Generic counter metric from v$sysstat view in Oracle.", []string{}, e.labels),
			prometheus.CounterValue,
			value,
		)
	}
	return nil
}

// ScrapeTablespace collects tablespace size.
func (e *Exporter) ScrapeTablespace(db *sql.DB, ch chan<- prometheus.Metric) error {
	var (
		rows *sql.Rows
		err  error
	)
	rows, err = db.Query(`SELECT
    a.tablespace_name         "Tablespace",
    b.status                  "Status",
    b.contents                "Type",
    b.extent_management       "Extent Mgmt",
    a.bytes                   bytes,
    a.maxbytes                bytes_max,
    c.bytes_free + NVL(d.bytes_expired,0)  bytes_free
FROM
  (
    -- belegter und maximal verfuegbarer platz pro datafile
    -- nach tablespacenamen zusammengefasst
    -- => bytes
    -- => maxbytes
    SELECT
        a.tablespace_name,
        SUM(a.bytes)          bytes,
        SUM(DECODE(a.autoextensible, 'YES', a.maxbytes, 'NO', a.bytes)) maxbytes
    FROM
        dba_data_files a
    GROUP BY
        tablespace_name
  ) a,
  sys.dba_tablespaces b,
  (
    -- freier platz pro tablespace
    -- => bytes_free
    SELECT
        a.tablespace_name,
        SUM(a.bytes) bytes_free
    FROM
        dba_free_space a
    GROUP BY
        tablespace_name
  ) c,
  (
    -- freier platz durch expired extents
    -- speziell fuer undo tablespaces
    -- => bytes_expired
    SELECT
        a.tablespace_name,
        SUM(a.bytes) bytes_expired
    FROM
        dba_undo_extents a
    WHERE
        status = 'EXPIRED'
    GROUP BY
        tablespace_name
  ) d
WHERE
    a.tablespace_name = c.tablespace_name (+)
    AND a.tablespace_name = b.tablespace_name
    AND a.tablespace_name = d.tablespace_name (+)
UNION ALL
SELECT
    d.tablespace_name "Tablespace",
    b.status "Status",
    b.contents "Type",
    b.extent_management "Extent Mgmt",
    sum(a.bytes_free + a.bytes_used) bytes,   -- allocated
    SUM(DECODE(d.autoextensible, 'YES', d.maxbytes, 'NO', d.bytes)) bytes_max,
    SUM(a.bytes_free + a.bytes_used - NVL(c.bytes_used, 0)) bytes_free
FROM
    sys.v_$TEMP_SPACE_HEADER a,
    sys.dba_tablespaces b,
    sys.v_$Temp_extent_pool c,
    dba_temp_files d
WHERE
    c.file_id(+)             = a.file_id
    and c.tablespace_name(+) = a.tablespace_name
    and d.file_id            = a.file_id
    and d.tablespace_name    = a.tablespace_name
    and b.tablespace_name    = a.tablespace_name
GROUP BY
    b.status,
    b.contents,
    b.extent_management,
    d.tablespace_name
ORDER BY
    1
`)
	if err != nil {
		return err
	}
	defer rows.Close()
	tablespaceBytesDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "tablespace", "bytes"),
		"Generic counter metric of tablespaces bytes in Oracle.",
		[]string{"tablespace", "type"}, e.labels,
	)
	tablespaceMaxBytesDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "tablespace", "max_bytes"),
		"Generic counter metric of tablespaces max bytes in Oracle.",
		[]string{"tablespace", "type"}, e.labels,
	)
	tablespaceFreeBytesDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "tablespace", "free"),
		"Generic counter metric of tablespaces free bytes in Oracle.",
		[]string{"tablespace", "type"}, e.labels,
	)

	for rows.Next() {
		var tablespace_name string
		var status string
		var contents string
		var extent_management string
		var bytes float64
		var max_bytes float64
		var bytes_free float64

		if err := rows.Scan(&tablespace_name, &status, &contents, &extent_management, &bytes, &max_bytes, &bytes_free); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(tablespaceBytesDesc,     prometheus.GaugeValue, float64(bytes),      tablespace_name, contents)
		ch <- prometheus.MustNewConstMetric(tablespaceMaxBytesDesc,  prometheus.GaugeValue, float64(max_bytes),  tablespace_name, contents)
		ch <- prometheus.MustNewConstMetric(tablespaceFreeBytesDesc, prometheus.GaugeValue, float64(bytes_free), tablespace_name, contents)
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
