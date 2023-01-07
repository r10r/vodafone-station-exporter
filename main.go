package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/fluepke/vodafone-station-exporter/api"
	"github.com/fluepke/vodafone-station-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const version = "0.0.1"

var (
	showVersion                 = flag.Bool("version", false, "Print version and exit")
	showMetrics                 = flag.Bool("show-metrics", false, "Show available metrics and exit")
	listenAddress               = flag.String("web.listen-address", "[::]:9420", "Address to listen on")
	metricsPath                 = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics")
	docsisMetricsPath           = flag.String("web.docsis-metrics", "/docsis/metrics", "Path under which to expose the docsis metrics")
	vodafoneStationUrl          = flag.String("vodafone.station-url", "https://192.168.0.1", "Vodafone station URL. For bridge mode this is 192.168.100.1 (note: Configure a route if using bridge mode)")
	vodafoneStationPassword     = flag.String("vodafone.station-password", "How is the default password calculated? mhmm", "Password for logging into the Vodafone station")
	vodafoneStationPasswordFile = flag.String("vodafone.station-password-file", "", "Password file")
)

func main() {
	flag.Parse()

	log.SetFlags(log.Flags() | log.Lmicroseconds)

	if *showMetrics {
		describeMetrics()
		os.Exit(0)
	}

	if *vodafoneStationPasswordFile != "" {
		log.Printf("Using password file (%s)", *vodafoneStationPasswordFile)
		data, err := os.ReadFile(*vodafoneStationPasswordFile)
		if err != nil {
			panic(fmt.Errorf("failed to read password file: %w", err))
		}
		*vodafoneStationPassword = strings.TrimSpace(string(data))
	}

	if *showVersion {
		fmt.Println("vodafone-station-exporter")
		fmt.Printf("Version: %s\n", version)
		fmt.Println("Author: @fluepke")
		fmt.Println("Prometheus Exporter for the Vodafone Station (CGA4233DE)")
		os.Exit(0)
	}

	s := server{
		station: api.NewVodafoneStation(*vodafoneStationUrl, *vodafoneStationPassword),
	}

	s.start()
}

func describeMetrics() {
	fmt.Println("Exported metrics")
	c := &collector.Collector{}
	ch := make(chan *prometheus.Desc)
	go func() {
		defer close(ch)
		c.Describe(ch)
	}()
	for desc := range ch {
		if desc == nil {
			continue
		}
		describeMetric(desc)
	}
}

func describeMetric(desc *prometheus.Desc) {
	fqName := reflect.ValueOf(desc).Elem().FieldByName("fqName").String()
	help := reflect.ValueOf(desc).Elem().FieldByName("help").String()
	labels := reflect.ValueOf(desc).Elem().FieldByName("variableLabels")
	fmt.Println("  * `" + fqName + "`: " + help)
	if labels.Len() == 0 {
		return
	}
	fmt.Print("    - Labels: ")
	first := true
	for i := 0; i < labels.Len(); i++ {
		if !first {
			fmt.Print(", ")
		}
		first = false
		fmt.Print("`" + labels.Index(i).String() + "`")
	}
	fmt.Println("")
}

type server struct {
	station *api.VodafoneStation
}

func (s *server) start() {
	log.Printf("Starting vodafone-station-exporter (version %s)", version)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
            <head><title>vodafone-station-exporter (Version ` + version + `)</title></head>
            <body>
            <h1>vodafone-station-exporter</h1>
            <p><a href="` + *vodafoneStationUrl + `">Station Login</a></p>
            <h2>Metrics</h2>
            <ul>
            <li><a href="` + *metricsPath + `">metrics</a>
            <li><a href="` + *docsisMetricsPath + `">docsis metrics</a>
            </ul>
            </body>
            </html>`))
	})
	log.Printf("serving metrics on %s", *metricsPath)
	http.HandleFunc(*metricsPath, s.handleMetricsRequest)
	log.Printf("serving docsis metrics on %s", *docsisMetricsPath)
	http.HandleFunc(*docsisMetricsPath, s.handleDocsisMetricsRequest)

	log.Printf("Listening on %s", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Printf("Failed to listen: %s", err)
	}
}

func (s *server) handleMetricsRequest(w http.ResponseWriter, request *http.Request) {
	registry := prometheus.NewRegistry()
	c := &collector.Collector{}
	c.Station = s.station
	registry.MustRegister(c)
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:      log.Default(),
		ErrorHandling: promhttp.ContinueOnError,
	}).ServeHTTP(w, request)
}

func (s *server) handleDocsisMetricsRequest(w http.ResponseWriter, request *http.Request) {
	registry := prometheus.NewRegistry()
	c := &collector.DocsisCollector{}
	c.Station = s.station
	registry.MustRegister(c)
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:      log.Default(),
		ErrorHandling: promhttp.ContinueOnError,
	}).ServeHTTP(w, request)
}
