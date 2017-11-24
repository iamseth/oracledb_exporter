package main

import (
	"flag"
	"net/http"
	"os"
	"io/ioutil"
	"github.com/BurntSushi/toml"
	"strings"
  "./exporter"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	pageHeader = "<html><head><title>Oracle DB exporter</title></head><body><h1>Oracle DB exporter</h1>"
	pageFooter = "</body></html>"
)

var (
	// Version will be set at build time.
	Version       = "0.0.0.dev"
	listenAddress = flag.String("web.listen-address", ":9161", "Address to listen on for web interface and telemetry.")
	metricPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	landingPage   = []byte(pageHeader + "<p><a href='" + *metricPath + "'>Metrics</a></p>" + pageFooter)
	readConfigDir = flag.String("config-dir", "", "Directory where we store file config.")
	listenerPort  = flag.String("listener-port", "1521", "Default Oracle listener port.")
	listenerAddr  = flag.String("listener-address", "localhost", "Default Oracle listener address.")
)

type DBConfig struct {
	Name string
	String string
	User string
	Pass string
	Host string
	Port string
	Service string
}

func loadConfigFile(path string, file string) (string, string) {
	name := strings.TrimSuffix(file, ".toml")
	var cfg DBConfig
	_, err := toml.DecodeFile(path + "/" + file, &cfg)
	if err != nil { panic(err) }
	// Read name from file or use filename prefix
	if cfg.Name == "" { cfg.Name = name }
	// get connection string or construct it
	oracleConnectionString := cfg.String
	if oracleConnectionString == "" {
		// default value
		if cfg.Host == "" { cfg.Host = *listenerAddr }
		if cfg.Port == "" { cfg.Port = *listenerPort }
		oracleConnectionString = cfg.User + "/" + cfg.Pass + "@" + cfg.Host + ":" + cfg.Port + "/" + cfg.Service
	}
	return cfg.Name, oracleConnectionString
}

func main() {
	flag.Parse()
	log.Infoln("Starting oracledb_exporter " + Version)
	if *readConfigDir == "" {
		exporter.NewExporter(os.Getenv("DATA_SOURCE_NAME"), "default")
	} else {
		files, err := ioutil.ReadDir(*readConfigDir)
		if err != nil { panic(err) }
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".toml") {
				name, connectionString := loadConfigFile(*readConfigDir, f.Name())
				log.Infoln("New Oracle instance detected: " + name)
				exporter.NewExporter(connectionString, name)
			}
		}
	}
	http.Handle(*metricPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage)
	})
	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
