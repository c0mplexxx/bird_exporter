package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateExporterRuntimeConfig(t *testing.T) {
	valid := exporterRuntimeConfig{
		BirdV2:           true,
		BirdSocket:       "/run/bird/bird.ctl",
		MaxResponseBytes: 4 << 20,
	}

	tests := []struct {
		name   string
		mutate func(*exporterRuntimeConfig)
	}{
		{name: "non-positive response limit", mutate: func(c *exporterRuntimeConfig) { c.MaxResponseBytes = 0 }},
		{name: "empty v2 socket", mutate: func(c *exporterRuntimeConfig) { c.BirdSocket = "" }},
		{name: "no v1 daemon enabled", mutate: func(c *exporterRuntimeConfig) { c.BirdV2 = false }},
		{name: "empty v1 IPv4 socket", mutate: func(c *exporterRuntimeConfig) { c.BirdV2 = false; c.BirdEnabled = true; c.BirdSocket = "" }},
		{name: "empty v1 IPv6 socket", mutate: func(c *exporterRuntimeConfig) { c.BirdV2 = false; c.Bird6Enabled = true; c.Bird6Socket = "" }},
		{name: "TLS certificate without key", mutate: func(c *exporterRuntimeConfig) { c.TLSEnabled = true; c.TLSCertFile = "/cert.pem" }},
	}

	require.NoError(t, validateExporterRuntimeConfig(valid))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := valid
			tt.mutate(&config)
			require.Error(t, validateExporterRuntimeConfig(config))
		})
	}
}
