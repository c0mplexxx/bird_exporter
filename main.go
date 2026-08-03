package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/czerwonk/bird_exporter/protocol"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const version string = "1.5.0"

var (
	showVersion      = flag.Bool("version", false, "Print version information.")
	listenAddress    = flag.String("web.listen-address", ":9324", "Address on which to expose metrics and web interface.")
	metricsPath      = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	birdSocket       = flag.String("bird.socket", "/var/run/bird.ctl", "Socket to communicate with bird routing daemon")
	birdV2           = flag.Bool("bird.v2", false, "Bird major version >= 2.0 (multi channel protocols)")
	tlsEnabled       = flag.Bool("tls.enabled", false, "Enables TLS")
	tlsCertChainPath = flag.String("tls.cert-file", "", "Path to TLS cert file")
	tlsKeyPath       = flag.String("tls.key-file", "", "Path to TLS key file")
	newFormat        = flag.Bool("format.new", true, "New metric format (more convenient / generic)")
	enableBGP        = flag.Bool("proto.bgp", true, "Enables metrics for protocol BGP")
	enableOSPF       = flag.Bool("proto.ospf", true, "Enables metrics for protocol OSPF")
	enableKernel     = flag.Bool("proto.kernel", true, "Enables metrics for protocol Kernel")
	enableStatic     = flag.Bool("proto.static", true, "Enables metrics for protocol Static")
	enableDirect     = flag.Bool("proto.direct", true, "Enables metrics for protocol Direct")
	enableBabel      = flag.Bool("proto.babel", true, "Enables metrics for protocol Babel")
	enableRPKI       = flag.Bool("proto.rpki", true, "Enables metrics for protocol RPKI")
	enableBFD        = flag.Bool("proto.bfd", true, "Enables metrics for protocol BFD")
	// pre bird 2.0
	bird6Socket            = flag.String("bird.socket6", "/var/run/bird6.ctl", "Socket to communicate with bird6 routing daemon (not compatible with -bird.v2)")
	birdEnabled            = flag.Bool("bird.ipv4", true, "Get protocols from bird (not compatible with -bird.v2)")
	bird6Enabled           = flag.Bool("bird.ipv6", true, "Get protocols from bird6 (not compatible with -bird.v2)")
	descriptionLabels      = flag.Bool("format.description-labels", false, "Add labels from protocol descriptions.")
	descriptionLabelsRegex = flag.String("format.description-labels-regex", "(\\w+)=(\\w+)", "Regex to extract labels from protocol description")
	scrapeTimeout          = flag.Duration("web.scrape-timeout", 5*time.Second, "Maximum duration of one metrics scrape, including all BIRD queries.")
	maxConcurrentScrapes   = flag.Int("web.max-concurrent-scrapes", 1, "Maximum number of scrapes allowed to query BIRD concurrently.")
	maxResponseBytes       = flag.Int("bird.max-response-bytes", 4<<20, "Maximum accepted size of one BIRD socket reply in bytes.")
)

func init() {
	flag.Usage = func() {
		fmt.Println("Usage: bird_exporter [ ... ]\n\nParameters:")
		fmt.Println()
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	startServer()
}

func printVersion() {
	fmt.Println("bird_exporter")
	fmt.Printf("Version: %s\n", version)
	fmt.Println("Author(s): Daniel Czerwonk")
	fmt.Println("Metric exporter for bird routing daemon")
}

func startServer() {
	log.Infof("Starting bird exporter (Version: %s)", version)

	if !*newFormat {
		log.Info("INFO: You are using the old metric format. Please consider using the new (more convenient one) by setting -format.new=true.")
	}

	handler, err := newExporterHTTPHandler(exporterHTTPConfig{
		MetricsPath:          *metricsPath,
		ScrapeTimeout:        *scrapeTimeout,
		MaxConcurrentScrapes: *maxConcurrentScrapes,
		CollectorFactory: func(ctx context.Context) (prometheus.Collector, error) {
			return NewMetricCollectorWithContext(
				ctx,
				*newFormat,
				enabledProtocols(),
				*descriptionLabels,
				*birdSocket,
				*scrapeTimeout,
				*maxResponseBytes,
			), nil
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	server := newExporterHTTPServer(*listenAddress, handler, *scrapeTimeout)

	log.Infof("Listening for %s on %s (TLS: %v)", *metricsPath, *listenAddress, *tlsEnabled)
	if *tlsEnabled {
		log.Fatal(server.ListenAndServeTLS(*tlsCertChainPath, *tlsKeyPath))
		return
	}

	log.Fatal(server.ListenAndServe())
}

func enabledProtocols() protocol.Proto {
	res := protocol.Proto(0)

	if *enableBGP {
		res |= protocol.BGP
	}
	if *enableOSPF {
		res |= protocol.OSPF
	}
	if *enableKernel {
		res |= protocol.Kernel
	}
	if *enableStatic {
		res |= protocol.Static
	}
	if *enableDirect {
		res |= protocol.Direct
	}
	if *enableBabel {
		res |= protocol.Babel
	}
	if *enableRPKI {
		res |= protocol.RPKI
	}
	if *enableBFD {
		res |= protocol.BFD
	}

	return res
}
