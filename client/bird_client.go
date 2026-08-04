package client

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/czerwonk/bird_exporter/parser"
	"github.com/czerwonk/bird_exporter/protocol"
	birdsocket "github.com/c0mplexxx/bird_socket"
)

// BirdClient communicates with the bird socket to retrieve information
type BirdClient struct {
	Options     *BirdClientOptions
	queryFailed atomic.Bool
}

// BirdClientOptions defines options to connect to bird
type BirdClientOptions struct {
	BirdV2       bool
	BirdEnabled  bool
	Bird6Enabled bool
	BirdSocket   string
	Bird6Socket  string
	Context      context.Context
	QueryTimeout time.Duration
	MaxResponse  int
}

// GetProtocols retrieves protocol information and statistics from bird
func (c *BirdClient) GetProtocols() ([]*protocol.Protocol, error) {
	ipVersions := make([]string, 0)
	if c.Options.BirdV2 {
		ipVersions = append(ipVersions, "")
	} else {
		if c.Options.BirdEnabled {
			ipVersions = append(ipVersions, "4")
		}

		if c.Options.Bird6Enabled {
			ipVersions = append(ipVersions, "6")
		}
	}

	return c.protocolsFromBird(ipVersions)
}

// GetOSPFAreas retrieves OSPF specific information from bird
func (c *BirdClient) GetOSPFAreas(protocol *protocol.Protocol) ([]*protocol.OSPFArea, error) {
	sock := c.socketFor(protocol.IPVersion)
	b, err := c.query(sock, fmt.Sprintf("show ospf %s", protocol.Name))
	if err != nil {
		return nil, err
	}

	areas, err := parser.ParseOSPFWithError(b)
	if err != nil {
		c.queryFailed.Store(true)
	}
	return areas, err
}

// GetBFDSessions retrieves BFD specific information from bird
func (c *BirdClient) GetBFDSessions(protocol *protocol.Protocol) ([]*protocol.BFDSession, error) {
	sock := c.socketFor(protocol.IPVersion)
	b, err := c.query(sock, fmt.Sprintf("show bfd sessions %s", protocol.Name))
	if err != nil {
		return nil, err
	}

	sessions, err := parser.ParseBFDSessionsWithError(protocol.Name, b)
	if err != nil {
		c.queryFailed.Store(true)
	}
	return sessions, err
}

func (c *BirdClient) protocolsFromBird(ipVersions []string) ([]*protocol.Protocol, error) {
	protocols := make([]*protocol.Protocol, 0)

	for _, ipVersion := range ipVersions {
		sock := c.socketFor(ipVersion)
		s, err := c.protocolsFromSocket(sock, ipVersion)
		if err != nil {
			return nil, err
		}

		protocols = append(protocols, s...)
	}

	return protocols, nil
}

func (c *BirdClient) protocolsFromSocket(socketPath string, ipVersion string) ([]*protocol.Protocol, error) {
	b, err := c.query(socketPath, "show protocols all")
	if err != nil {
		return nil, err
	}

	protocols, err := parser.ParseProtocolsWithError(b, ipVersion)
	if err != nil {
		c.queryFailed.Store(true)
	}
	return protocols, err
}

// QueriesSucceeded reports whether every query and bounded parser operation
// performed by this client has succeeded.
func (c *BirdClient) QueriesSucceeded() bool {
	return !c.queryFailed.Load()
}

func (c *BirdClient) socketFor(ipVersion string) string {
	if !c.Options.BirdV2 && ipVersion == "6" {
		return c.Options.Bird6Socket
	}

	return c.Options.BirdSocket
}

// StatusFromSocket retrieves status information from bird
func (c *BirdClient) StatusFromSocket(socketPath string) (*parser.Status, error) {
	b, err := c.query(socketPath, "show status")
	if err != nil {
		return nil, err
	}

	status, err := parser.ParseStatusWithError(b)
	if err != nil {
		c.queryFailed.Store(true)
	}
	return status, err
}

func (c *BirdClient) query(socketPath string, query string) ([]byte, error) {
	ctx := c.Options.Context
	if ctx == nil {
		ctx = context.Background()
	}

	options := make([]birdsocket.Option, 0, 2)
	if c.Options.QueryTimeout > 0 {
		options = append(options, birdsocket.WithTimeout(c.Options.QueryTimeout))
	}
	if c.Options.MaxResponse > 0 {
		options = append(options, birdsocket.WithMaxResponseBytes(c.Options.MaxResponse))
	}

	response, err := birdsocket.QueryContext(ctx, socketPath, query, options...)
	if err != nil {
		c.queryFailed.Store(true)
	}
	return response, err
}
