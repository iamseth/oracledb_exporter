# Operating principles

The exporter itself is dumb and does not do much. The initialization is done as follows:

- Parse flags options
- Load the default toml file (default-metrics.toml) and store each metrics in a Metric struct
- Load the custom toml file (if a custom toml file is given)
- Create an Exporter object
- Register exporter in prometheus library
- Launching a web server to handle incoming requests

These operations are mainly done in the `main` function.

After this initialization phase, the exporter will wait for the arrival of the request.

Each time, it will iterate over the content of the metricsToScrap structure (in the function scrape `func (e * Export) scrape (ch chan <- prometheus.Metric)`).

For each element (of Metric type), a call to the `ScrapeMetric` function will be made which will itself make a call to the` ScrapeGenericValues` function.

The `ScrapeGenericValues` function will read the information from the Metric structure and - depending on the parameters - will generate the metrics to return. In particular, it will use the `GeneratePrometheusMetrics` function which will make SQL calls to the database.
