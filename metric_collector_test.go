package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/czerwonk/bird_exporter/client"
	"github.com/czerwonk/bird_exporter/metrics"
	"github.com/czerwonk/bird_exporter/protocol"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestMetricCollectorReportsAuxiliaryQueryFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets are not available on Windows")
	}
	path, wait := startSequenceBirdServer(t, []string{"0000\n", "9000 status failed\n"})
	birdClient := &client.BirdClient{Options: &client.BirdClientOptions{
		BirdV2:       true,
		BirdSocket:   path,
		Context:      context.Background(),
		QueryTimeout: time.Second,
		MaxResponse:  4 << 20,
	}}
	collector := &MetricCollector{
		exporters: make(map[protocol.Proto][]metrics.MetricExporter),
		client:    birdClient,
		status:    metrics.NewStatusExporter(birdClient, path),
	}

	registry := prometheus.NewRegistry()
	require.NoError(t, registry.Register(collector))
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)
	require.Equal(t, float64(0), metricValue(t, metricFamilies, "bird_socket_query_success"))
	require.Equal(t, float64(0), metricValue(t, metricFamilies, "bird_daemon_up"))
	wait()
}

func TestMetricCollectorReportsDaemonDownWithoutQueryFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets are not available on Windows")
	}
	path, wait := startSequenceBirdServer(t, []string{
		"0000\n",
		"1000-BIRD 2.17.1\n1011-Daemon is down\n0000\n",
	})
	birdClient := &client.BirdClient{Options: &client.BirdClientOptions{
		BirdV2:       true,
		BirdSocket:   path,
		Context:      context.Background(),
		QueryTimeout: time.Second,
		MaxResponse:  4 << 20,
	}}
	collector := &MetricCollector{
		exporters: make(map[protocol.Proto][]metrics.MetricExporter),
		client:    birdClient,
		status:    metrics.NewStatusExporter(birdClient, path),
	}

	registry := prometheus.NewRegistry()
	require.NoError(t, registry.Register(collector))
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)
	require.Equal(t, float64(1), metricValue(t, metricFamilies, "bird_socket_query_success"))
	require.Equal(t, float64(0), metricValue(t, metricFamilies, "bird_daemon_up"))
	wait()
}

func metricValue(t *testing.T, families []*dto.MetricFamily, name string) float64 {
	t.Helper()
	for _, family := range families {
		if family.GetName() == name {
			require.Len(t, family.Metric, 1)
			return family.Metric[0].GetGauge().GetValue()
		}
	}
	t.Fatalf("metric %s not found", name)
	return 0
}

func startSequenceBirdServer(t *testing.T, responses []string) (string, func()) {
	t.Helper()

	path := filepath.Join(os.TempDir(), fmt.Sprintf("bird_exporter_sequence_%d_%d.sock", os.Getpid(), time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		defer close(done)
		for _, response := range responses {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				done <- acceptErr
				return
			}
			if _, writeErr := conn.Write([]byte("0001 BIRD ready.\n")); writeErr != nil {
				_ = conn.Close()
				done <- writeErr
				return
			}
			if _, readErr := bufio.NewReader(conn).ReadString('\n'); readErr != nil {
				_ = conn.Close()
				done <- readErr
				return
			}
			if _, writeErr := conn.Write([]byte(response)); writeErr != nil {
				_ = conn.Close()
				done <- writeErr
				return
			}
			if closeErr := conn.Close(); closeErr != nil {
				done <- closeErr
				return
			}
		}
	}()

	return path, func() {
		t.Helper()
		require.NoError(t, listener.Close())
		select {
		case serverErr := <-done:
			require.NoError(t, serverErr)
		case <-time.After(time.Second):
			t.Fatal("test BIRD server did not stop")
		}
	}
}
