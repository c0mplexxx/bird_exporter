package main

import (
	"context"
	"errors"
	"html"
	"net/http"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 64 << 10
)

type collectorFactory func(context.Context) (prometheus.Collector, error)

type exporterHTTPConfig struct {
	MetricsPath          string
	ScrapeTimeout        time.Duration
	MaxConcurrentScrapes int
	CollectorFactory     collectorFactory
}

type exporterHTTPHandler struct {
	metricsPath   string
	scrapeTimeout time.Duration
	scrapeSlots   chan struct{}
	newCollector  collectorFactory
	inFlight      prometheus.Gauge
	rejected      prometheus.Counter
	buildInfo     prometheus.Gauge
}

func newExporterHTTPHandler(config exporterHTTPConfig) (*exporterHTTPHandler, error) {
	if config.MetricsPath == "" || !strings.HasPrefix(config.MetricsPath, "/") {
		return nil, errors.New("web telemetry path must start with a slash")
	}
	if config.MetricsPath == "/" || path.Clean(config.MetricsPath) != config.MetricsPath {
		return nil, errors.New("web telemetry path must be a clean non-root path")
	}
	if config.ScrapeTimeout <= 0 {
		return nil, errors.New("web scrape timeout must be positive")
	}
	if config.MaxConcurrentScrapes <= 0 {
		return nil, errors.New("maximum concurrent scrapes must be positive")
	}
	if config.CollectorFactory == nil {
		return nil, errors.New("collector factory must not be nil")
	}

	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bird_exporter_build_info",
		Help: "Build information for the running bird_exporter binary.",
	}, []string{"version", "revision", "go_version"}).WithLabelValues(version, revision, runtime.Version())
	buildInfo.Set(1)

	return &exporterHTTPHandler{
		metricsPath:   config.MetricsPath,
		scrapeTimeout: config.ScrapeTimeout,
		scrapeSlots:   make(chan struct{}, config.MaxConcurrentScrapes),
		newCollector:  config.CollectorFactory,
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bird_exporter_scrapes_in_flight",
			Help: "Number of metrics scrapes currently querying BIRD.",
		}),
		rejected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bird_exporter_scrapes_rejected_total",
			Help: "Total number of metrics scrapes rejected by the concurrency limit.",
		}),
		buildInfo: buildInfo,
	}, nil
}

func newExporterHTTPServer(address string, handler http.Handler, scrapeTimeout time.Duration) *http.Server {
	writeTimeout := scrapeTimeout + 5*time.Second
	if writeTimeout < 10*time.Second {
		writeTimeout = 10 * time.Second
	}

	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}

func (h *exporterHTTPHandler) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/":
		h.serveRoot(w, request)
	case h.metricsPath:
		h.serveMetrics(w, request)
	default:
		http.NotFound(w, request)
	}
}

func (h *exporterHTTPHandler) serveRoot(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	body := `<html>
	<head><title>Bird Routing Daemon Exporter (Version ` + html.EscapeString(version) + `)</title></head>
	<body>
	<h1>Bird Routing Daemon Exporter</h1>
	<p><a href="` + html.EscapeString(h.metricsPath) + `">Metrics</a></p>
	<h2>More information:</h2>
	<p><a href="https://github.com/czerwonk/bird_exporter">github.com/czerwonk/bird_exporter</a></p>
	</body>
	</html>`
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if request.Method == http.MethodGet {
		_, _ = w.Write([]byte(body))
	}
}

func (h *exporterHTTPHandler) serveMetrics(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	select {
	case h.scrapeSlots <- struct{}{}:
		defer func() { <-h.scrapeSlots }()
	default:
		h.rejected.Inc()
		w.Header().Set("Retry-After", "1")
		http.Error(w, "maximum concurrent scrapes reached", http.StatusServiceUnavailable)
		return
	}

	h.inFlight.Inc()
	defer h.inFlight.Dec()

	ctx, cancel := context.WithTimeout(request.Context(), h.scrapeTimeout)
	defer cancel()

	collector, err := h.newCollector(ctx)
	if err != nil {
		http.Error(w, "cannot initialize metrics collector", http.StatusInternalServerError)
		return
	}

	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		http.Error(w, "cannot register metrics collector", http.StatusInternalServerError)
		return
	}
	registry.MustRegister(h.inFlight, h.rejected, h.buildInfo)

	errorLogger := log.New()
	errorLogger.Level = log.ErrorLevel
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:      errorLogger,
		ErrorHandling: promhttp.HTTPErrorOnError,
	}).ServeHTTP(w, request.WithContext(ctx))
}
