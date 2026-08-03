package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCollectorDesc = prometheus.NewDesc(
	"bird_exporter_test_value",
	"Test-only metric.",
	nil,
	nil,
)

type blockingCollector struct {
	ctx     context.Context
	started chan<- struct{}
	release <-chan struct{}
}

func (*blockingCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- testCollectorDesc
}

func (c *blockingCollector) Collect(ch chan<- prometheus.Metric) {
	select {
	case c.started <- struct{}{}:
	default:
	}

	select {
	case <-c.ctx.Done():
	case <-c.release:
	}
	ch <- prometheus.MustNewConstMetric(testCollectorDesc, prometheus.GaugeValue, 1)
}

type staticCollector struct{}

func (*staticCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- testCollectorDesc
}

func (*staticCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(testCollectorDesc, prometheus.GaugeValue, 1)
}

func TestExporterHTTPHandlerRejectsConcurrentScrapes(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	handler, err := newExporterHTTPHandler(exporterHTTPConfig{
		MetricsPath:          "/metrics",
		ScrapeTimeout:        time.Second,
		MaxConcurrentScrapes: 1,
		CollectorFactory: func(ctx context.Context) (prometheus.Collector, error) {
			return &blockingCollector{ctx: ctx, started: started, release: release}, nil
		},
	})
	require.NoError(t, err)

	firstRecorder := httptest.NewRecorder()
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		handler.ServeHTTP(firstRecorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first scrape did not reach the collector")
	}

	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusServiceUnavailable, secondRecorder.Code)
	assert.Equal(t, "1", secondRecorder.Header().Get("Retry-After"))

	close(release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first scrape did not finish")
	}
	assert.Equal(t, http.StatusOK, firstRecorder.Code)

	thirdRecorder := httptest.NewRecorder()
	handler.ServeHTTP(thirdRecorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusOK, thirdRecorder.Code)
	assert.Contains(t, thirdRecorder.Body.String(), "bird_exporter_scrapes_rejected_total 1")
}

func TestExporterHTTPHandlerPropagatesCancellation(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	handler, err := newExporterHTTPHandler(exporterHTTPConfig{
		MetricsPath:          "/metrics",
		ScrapeTimeout:        time.Second,
		MaxConcurrentScrapes: 1,
		CollectorFactory: func(ctx context.Context) (prometheus.Collector, error) {
			return &blockingCollector{ctx: ctx, started: started, release: release}, nil
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}()

	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("scrape did not reach the collector")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled scrape did not stop")
	}
}

func TestExporterHTTPHandlerRoutesAndMethods(t *testing.T) {
	handler, err := newExporterHTTPHandler(exporterHTTPConfig{
		MetricsPath:          "/metrics",
		ScrapeTimeout:        time.Second,
		MaxConcurrentScrapes: 1,
		CollectorFactory: func(context.Context) (prometheus.Collector, error) {
			return &staticCollector{}, nil
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
		allow      string
	}{
		{name: "root", method: http.MethodGet, path: "/", statusCode: http.StatusOK},
		{name: "root head", method: http.MethodHead, path: "/", statusCode: http.StatusOK},
		{name: "root post", method: http.MethodPost, path: "/", statusCode: http.StatusMethodNotAllowed, allow: "GET, HEAD"},
		{name: "metrics", method: http.MethodGet, path: "/metrics", statusCode: http.StatusOK},
		{name: "metrics head", method: http.MethodHead, path: "/metrics", statusCode: http.StatusMethodNotAllowed, allow: "GET"},
		{name: "unknown", method: http.MethodGet, path: "/anything", statusCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))
			assert.Equal(t, tt.statusCode, recorder.Code)
			assert.Equal(t, tt.allow, recorder.Header().Get("Allow"))
			if tt.method == http.MethodHead && tt.statusCode == http.StatusOK {
				assert.Empty(t, recorder.Body.String())
			}
		})
	}
}

func TestNewExporterHTTPHandlerValidation(t *testing.T) {
	validFactory := func(context.Context) (prometheus.Collector, error) {
		return &staticCollector{}, nil
	}
	tests := []struct {
		name   string
		config exporterHTTPConfig
	}{
		{name: "relative path", config: exporterHTTPConfig{MetricsPath: "metrics", ScrapeTimeout: time.Second, MaxConcurrentScrapes: 1, CollectorFactory: validFactory}},
		{name: "root path", config: exporterHTTPConfig{MetricsPath: "/", ScrapeTimeout: time.Second, MaxConcurrentScrapes: 1, CollectorFactory: validFactory}},
		{name: "unclean path", config: exporterHTTPConfig{MetricsPath: "/a/../metrics", ScrapeTimeout: time.Second, MaxConcurrentScrapes: 1, CollectorFactory: validFactory}},
		{name: "zero timeout", config: exporterHTTPConfig{MetricsPath: "/metrics", MaxConcurrentScrapes: 1, CollectorFactory: validFactory}},
		{name: "zero concurrency", config: exporterHTTPConfig{MetricsPath: "/metrics", ScrapeTimeout: time.Second, CollectorFactory: validFactory}},
		{name: "nil factory", config: exporterHTTPConfig{MetricsPath: "/metrics", ScrapeTimeout: time.Second, MaxConcurrentScrapes: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newExporterHTTPHandler(tt.config)
			require.Error(t, err)
		})
	}
}

func TestExporterHTTPHandlerCollectorFactoryError(t *testing.T) {
	handler, err := newExporterHTTPHandler(exporterHTTPConfig{
		MetricsPath:          "/metrics",
		ScrapeTimeout:        time.Second,
		MaxConcurrentScrapes: 1,
		CollectorFactory: func(context.Context) (prometheus.Collector, error) {
			return nil, errors.New("test failure")
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, strings.Contains(recorder.Body.String(), "test failure"))
}

func TestNewExporterHTTPServerTimeouts(t *testing.T) {
	server := newExporterHTTPServer("127.0.0.1:9324", http.NotFoundHandler(), 30*time.Second)
	assert.Equal(t, readHeaderTimeout, server.ReadHeaderTimeout)
	assert.Equal(t, readTimeout, server.ReadTimeout)
	assert.Equal(t, 35*time.Second, server.WriteTimeout)
	assert.Equal(t, idleTimeout, server.IdleTimeout)
	assert.Equal(t, maxHeaderBytes, server.MaxHeaderBytes)
}
