package parser

import (
	"strings"
	"testing"
	"time"

	"github.com/czerwonk/bird_exporter/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProtocolsWithErrorRejectsOversizeLine(t *testing.T) {
	data := strings.Repeat("x", maxProtocolLineBytes+1) + "\n"
	protocols, err := ParseProtocolsWithError([]byte(data), "4")
	require.Error(t, err)
	require.Empty(t, protocols)
}

func TestParseProtocolsIgnoresOrphanDescription(t *testing.T) {
	protocols, err := ParseProtocolsWithError([]byte("  Description: untrusted=value\n0000\n"), "4")
	require.NoError(t, err)
	require.Empty(t, protocols)
}

func TestMultiChannelPreservesProtocolMetadata(t *testing.T) {
	data := []byte("peer BGP master up 00:01:00 Established\n" +
		"  Description: site=ams\n" +
		"  Channel ipv4\n" +
		"    Routes: 1 imported, 2 exported, 1 preferred\n" +
		"  Channel ipv6\n" +
		"    Routes: 3 imported, 4 exported, 3 preferred\n")

	protocols, err := ParseProtocolsWithError(data, "")
	require.NoError(t, err)
	require.Len(t, protocols, 2)
	for _, parsedProtocol := range protocols {
		require.Equal(t, "site=ams", parsedProtocol.Description)
		require.Equal(t, "Established", parsedProtocol.State)
	}
	require.Equal(t, "4", protocols[0].IPVersion)
	require.Equal(t, "6", protocols[1].IPVersion)
}

func TestRPKIChannelsRemainSeparate(t *testing.T) {
	data := []byte("r3k RPKI --- up 00:01:00 Established\n" +
		"  Channel roa4\n" +
		"    Routes: 10 imported, 0 exported, 10 preferred\n" +
		"  Channel roa6\n" +
		"    Routes: 20 imported, 0 exported, 20 preferred\n")

	protocols, err := ParseProtocolsWithError(data, "")
	require.NoError(t, err)
	require.Len(t, protocols, 2)
	require.Equal(t, "4", protocols[0].IPVersion)
	require.EqualValues(t, 10, protocols[0].Imported)
	require.Equal(t, "6", protocols[1].IPVersion)
	require.EqualValues(t, 20, protocols[1].Imported)
}

func TestEstablishedBgpOldTimeFormat(t *testing.T) {
	overrideNowFunc(func() time.Time {
		return time.Date(2018, 1, 1, 2, 0, 0, 0, time.UTC)
	})

	data := "foo    BGP      master   up     1514768400  Established\ntest\nbar\n  Routes:         12 imported, 1 filtered, 34 exported, 100 preferred\nxxx"
	p := ParseProtocols([]byte(data), "4")

	x := p[0]
	assert.EqualValues(t, "foo", x.Name, "name")
	assert.EqualValues(t, int(protocol.BGP), int(x.Proto), "proto")
	assert.EqualValues(t, 1, x.Up, "established")
	assert.EqualValues(t, 12, x.Imported, "imported")
	assert.EqualValues(t, 34, x.Exported, "exported")
	assert.EqualValues(t, 1, x.Filtered, "filtered")
	assert.EqualValues(t, 100, x.Preferred, "preferred")
	assert.EqualValues(t, "4", x.IPVersion, "ipVersion")
	assert.EqualValues(t, 3600, int64(x.Uptime), "uptime")
}

func TestEstablishedBgpCurrentTimeFormat(t *testing.T) {
	data := "foo    BGP      master   up     00:01:00  Established\ntest\nbar\n  Routes:         12 imported, 1 filtered, 34 exported, 100 preferred\nxxx"
	p := ParseProtocols([]byte(data), "4")
	assert.EqualValues(t, 1, len(p), "protocols")

	x := p[0]
	assert.EqualValues(t, "foo", x.Name, "name")
	assert.EqualValues(t, int(protocol.BGP), int(x.Proto), "proto")
	assert.EqualValues(t, 1, x.Up, "established")
	assert.EqualValues(t, 12, x.Imported, "imported")
	assert.EqualValues(t, 34, x.Exported, "exported")
	assert.EqualValues(t, 1, x.Filtered, "filtered")
	assert.EqualValues(t, 100, x.Preferred, "preferred")
	assert.EqualValues(t, "4", x.IPVersion, "ipVersion")
	assert.EqualValues(t, 60, x.Uptime, "uptime")
}

func TestEstablishedBgpIsoLongTimeFormat(t *testing.T) {
	overrideNowFunc(func() time.Time {
		return time.Date(2018, 1, 1, 2, 0, 0, 0, time.Local)
	})

	data := "foo    BGP      master   up     2018-01-01 01:00:00  Established\ntest\nbar\n  Routes:         12 imported, 1 filtered, 34 exported, 100 preferred\nxxx"
	p := ParseProtocols([]byte(data), "4")

	assert.EqualValues(t, 1, len(p), "protocols")

	x := p[0]
	assert.EqualValues(t, "foo", x.Name, "name")
	assert.EqualValues(t, int(protocol.BGP), int(x.Proto), "proto")
	assert.EqualValues(t, 1, x.Up, "established")
	assert.EqualValues(t, 12, x.Imported, "imported")
	assert.EqualValues(t, 34, x.Exported, "exported")
	assert.EqualValues(t, 1, x.Filtered, "filtered")
	assert.EqualValues(t, 100, x.Preferred, "preferred")
	assert.EqualValues(t, "4", x.IPVersion, "ipVersion")
	assert.EqualValues(t, 3600, int64(x.Uptime), "uptime")
}

func TestIpv6BGP(t *testing.T) {
	data := "foo    BGP      master   up     00:01:00  Established\ntest\nbar\n  Routes:         12 imported, 1 filtered, 34 exported, 100 preferred\nxxx"
	p := ParseProtocols([]byte(data), "6")
	assert.EqualValues(t, 1, len(p), "protocols")

	x := p[0]
	assert.EqualValues(t, "6", x.IPVersion, "ipVersion")
}

func TestActiveBGP(t *testing.T) {
	data := "bar    BGP      master   start   2016-01-01    Active\ntest\nbar"
	p := ParseProtocols([]byte(data), "4")
	assert.EqualValues(t, 1, len(p), "protocols")

	x := p[0]
	assert.EqualValues(t, "bar", x.Name, "name")
	assert.EqualValues(t, int(protocol.BGP), int(x.Proto), "proto")
	assert.EqualValues(t, 0, x.Up, "established")
	assert.EqualValues(t, 0, int(x.Imported), "imported")
	assert.EqualValues(t, 0, int(x.Exported), "exported")
	assert.EqualValues(t, "4", x.IPVersion, "ipVersion")
	assert.EqualValues(t, 0, int(x.Uptime), "uptime")
}

func Test2BGPSessions(t *testing.T) {
	data := "foo    BGP      master   up     00:01:00  Established\ntest\n  Routes:         12 imported, 1 filtered, 34 exported, 100 preferred\nbar    BGP      master   start   2016-01-01    Active\nxxx"
	p := ParseProtocols([]byte(data), "4")
	assert.EqualValues(t, 2, len(p), "protocols")
}

func TestUpdateAndWithdrawCounts(t *testing.T) {
	data := "foo    BGP      master   up     00:01:00  Established\ntest\n" +
		"  Routes:         12 imported, 1 filtered, 34 exported, 100 preferred\n" +
		"  Route change stats:     received   rejected   filtered    ignored   accepted\n" +
		"    Import updates:              1          2          3          4          5\n" +
		"    Import withdraws:            6          7          8          9         10\n" +
		"    Export updates:             11         12         13         14         15\n" +
		"    Export withdraws:           16         17         18         19        ---"
	p := ParseProtocols([]byte(data), "4")
	x := p[0]

	assert.EqualValues(t, 1, x.ImportUpdates.Received, "import updates received")
	assert.EqualValues(t, 2, x.ImportUpdates.Rejected, "import updates rejected")
	assert.EqualValues(t, 3, x.ImportUpdates.Filtered, "import updates filtered")
	assert.EqualValues(t, 4, x.ImportUpdates.Ignored, "import updates ignored")
	assert.EqualValues(t, 0, x.ImportUpdates.RxLimit, "import updates rx limit")
	assert.EqualValues(t, 0, x.ImportUpdates.Limit, "import updates limit")
	assert.EqualValues(t, 5, x.ImportUpdates.Accepted, "import updates accepted")
	assert.EqualValues(t, 6, x.ImportWithdraws.Received, "import withdraws received")
	assert.EqualValues(t, 7, x.ImportWithdraws.Rejected, "import withdraws rejected")
	assert.EqualValues(t, 8, x.ImportWithdraws.Filtered, "import withdraws filtered")
	assert.EqualValues(t, 9, x.ImportWithdraws.Ignored, "import withdraws ignored")
	assert.EqualValues(t, 0, x.ImportWithdraws.RxLimit, "import withdraws rx limit")
	assert.EqualValues(t, 0, x.ImportWithdraws.Limit, "import withdraws limit")
	assert.EqualValues(t, 10, x.ImportWithdraws.Accepted, "import withdraws accepted")
	assert.EqualValues(t, 11, x.ExportUpdates.Received, "export updates received")
	assert.EqualValues(t, 12, x.ExportUpdates.Rejected, "export updates rejected")
	assert.EqualValues(t, 13, x.ExportUpdates.Filtered, "export updates filtered")
	assert.EqualValues(t, 14, x.ExportUpdates.Ignored, "export updates ignored")
	assert.EqualValues(t, 0, x.ExportUpdates.RxLimit, "export updates rx limit")
	assert.EqualValues(t, 0, x.ExportUpdates.Limit, "export updates limit")
	assert.EqualValues(t, 15, x.ExportUpdates.Accepted, "export updates accepted")
	assert.EqualValues(t, 16, x.ExportWithdraws.Received, "export withdraws received")
	assert.EqualValues(t, 17, x.ExportWithdraws.Rejected, "export withdraws rejected")
	assert.EqualValues(t, 18, x.ExportWithdraws.Filtered, "export withdraws filtered")
	assert.EqualValues(t, 19, x.ExportWithdraws.Ignored, "export withdraws ignored")
	assert.EqualValues(t, 0, x.ExportWithdraws.RxLimit, "export withdraws rx limit")
	assert.EqualValues(t, 0, x.ExportWithdraws.Limit, "export withdraws limit")
	assert.EqualValues(t, 0, x.ExportWithdraws.Accepted, "export withdraws accepted")
}

func TestUpdateAndWithdrawCountsBird3(t *testing.T) {
	data := "foo    BGP      master   up     00:01:00  Established\ntest\n" +
		"  Routes:         12 imported, 1 filtered, 34 exported, 100 preferred\n" +
		"  Route change stats:     received   rejected   filtered    ignored   RX limit      limit   accepted\n" +
		"    Import updates:              1          2          3          4          9         10          5\n" +
		"    Import withdraws:            6          7          8          9         10         11         12\n" +
		"    Export updates:             11         12         13         14         15         16         17\n" +
		"    Export withdraws:           16         17         18         19         20         21        ---"
	p := ParseProtocols([]byte(data), "4")
	x := p[0]

	assert.EqualValues(t, 1, x.ImportUpdates.Received, "import updates received")
	assert.EqualValues(t, 2, x.ImportUpdates.Rejected, "import updates rejected")
	assert.EqualValues(t, 3, x.ImportUpdates.Filtered, "import updates filtered")
	assert.EqualValues(t, 4, x.ImportUpdates.Ignored, "import updates ignored")
	assert.EqualValues(t, 9, x.ImportUpdates.RxLimit, "import updates rx limit")
	assert.EqualValues(t, 10, x.ImportUpdates.Limit, "import updates limit")
	assert.EqualValues(t, 5, x.ImportUpdates.Accepted, "import updates accepted")
	assert.EqualValues(t, 6, x.ImportWithdraws.Received, "import withdraws received")
	assert.EqualValues(t, 7, x.ImportWithdraws.Rejected, "import withdraws rejected")
	assert.EqualValues(t, 8, x.ImportWithdraws.Filtered, "import withdraws filtered")
	assert.EqualValues(t, 9, x.ImportWithdraws.Ignored, "import withdraws ignored")
	assert.EqualValues(t, 10, x.ImportWithdraws.RxLimit, "import withdraws rx limit")
	assert.EqualValues(t, 11, x.ImportWithdraws.Limit, "import withdraws limit")
	assert.EqualValues(t, 12, x.ImportWithdraws.Accepted, "import withdraws accepted")
	assert.EqualValues(t, 11, x.ExportUpdates.Received, "export updates received")
	assert.EqualValues(t, 12, x.ExportUpdates.Rejected, "export updates rejected")
	assert.EqualValues(t, 13, x.ExportUpdates.Filtered, "export updates filtered")
	assert.EqualValues(t, 14, x.ExportUpdates.Ignored, "export updates ignored")
	assert.EqualValues(t, 15, x.ExportUpdates.RxLimit, "export updates rx limit")
	assert.EqualValues(t, 16, x.ExportUpdates.Limit, "export updates limit")
	assert.EqualValues(t, 17, x.ExportUpdates.Accepted, "export updates accepted")
	assert.EqualValues(t, 16, x.ExportWithdraws.Received, "export withdraws received")
	assert.EqualValues(t, 17, x.ExportWithdraws.Rejected, "export withdraws rejected")
	assert.EqualValues(t, 18, x.ExportWithdraws.Filtered, "export withdraws filtered")
	assert.EqualValues(t, 19, x.ExportWithdraws.Ignored, "export withdraws ignored")
	assert.EqualValues(t, 20, x.ExportWithdraws.RxLimit, "export withdraws rx limit")
	assert.EqualValues(t, 21, x.ExportWithdraws.Limit, "export withdraws limit")
	assert.EqualValues(t, 0, x.ExportWithdraws.Accepted, "export withdraws accepted")
}

func TestWithBird2(t *testing.T) {
	data := "Name       Proto      Table      State  Since         Info\n" +
		"bgp1       BGP        master     up     1494926415\n" +
		"  Channel ipv6\n" +
		"    Routes:         1 imported, 2 filtered, 3 exported, 4 preferred\n" +
		"    Input filter:   none\n" +
		"    Output filter:  all\n" +
		"\n" +
		"direct1    Direct     ---        up     1513027903\n" +
		"  Channel ipv4\n" +
		"    State:          UP\n" +
		"    Table:          master4\n" +
		"    Preference:     240\n" +
		"    Input filter:   ACCEPT\n" +
		"    Output filter:  REJECT\n" +
		"    Routes:         12 imported, 1 filtered, 34 exported, 100 preferred\n" +
		"    Route change stats:     received   rejected   filtered    ignored   accepted\n" +
		"      Import updates:              1          2          3          4          5\n" +
		"      Import withdraws:            6          7          8          9         10\n" +
		"      Export updates:             11         12         13         14         15\n" +
		"      Export withdraws:           16         17         18         19        ---\n" +
		"  Channel ipv6\n" +
		"    State:          UP\n" +
		"    Table:          master6\n" +
		"    Preference:     240\n" +
		"    Input filter:   ACCEPT\n" +
		"    Output filter:  REJECT\n" +
		"    Routes:         3 imported, 7 filtered, 5 exported, 13 preferred\n" +
		"    Route change stats:     received   rejected   filtered    ignored   accepted\n" +
		"      Import updates:             20         21         22         23         24\n" +
		"      Import withdraws:           25         26         27         28         29\n" +
		"      Export updates:             30         31         32         33         34\n" +
		"      Export withdraws:           35         36         37         38        ---\n" +
		"\n" +
		"ospf1      OSPF       master     up     1494926415\n" +
		"  Channel ipv4\n" +
		"    Routes:         4 imported, 3 filtered, 2 exported, 1 preferred\n" +
		"\n"

	p := ParseProtocols([]byte(data), "")
	assert.EqualValues(t, 4, len(p), "protocols")

	x := p[0]
	assert.EqualValues(t, "bgp1", x.Name, "BGP ipv6 name")
	assert.EqualValues(t, int(protocol.BGP), int(x.Proto), "BGP ipv6 proto")
	assert.EqualValues(t, "6", x.IPVersion, "BGP ipv6 ip version")
	assert.EqualValues(t, 1, x.Imported, "BGP ipv6 imported")
	assert.EqualValues(t, 3, x.Exported, "BGP ipv6 exported")
	assert.EqualValues(t, 2, x.Filtered, "BGP ipv6 filtered")
	assert.EqualValues(t, 4, x.Preferred, "BGP ipv6 preferred")
	assert.EqualValues(t, "none", x.ImportFilter, "BGP import filter")
	assert.EqualValues(t, "all", x.ExportFilter, "BGP export filter")

	x = p[1]
	assert.EqualValues(t, "direct1", x.Name, "Direct ipv4 name")
	assert.EqualValues(t, int(protocol.Direct), int(x.Proto), "Direct ipv4 proto")
	assert.EqualValues(t, "4", x.IPVersion, "Direct ipv4 ip version")
	assert.EqualValues(t, 12, x.Imported, "Direct ipv4 imported")
	assert.EqualValues(t, 34, x.Exported, "Direct ipv4 exported")
	assert.EqualValues(t, 1, x.Filtered, "Direct ipv4 filtered")
	assert.EqualValues(t, 100, x.Preferred, "Direct ipv4 preferred")
	assert.EqualValues(t, 1, x.ImportUpdates.Received, "Direct ipv4 import updates received")
	assert.EqualValues(t, 2, x.ImportUpdates.Rejected, "Direct ipv4 import updates rejected")
	assert.EqualValues(t, 3, x.ImportUpdates.Filtered, "Direct ipv4 import updates filtered")
	assert.EqualValues(t, 4, x.ImportUpdates.Ignored, "Direct ipv4 import updates ignored")
	assert.EqualValues(t, 5, x.ImportUpdates.Accepted, "Direct ipv4 import updates accepted")
	assert.EqualValues(t, 6, x.ImportWithdraws.Received, "Direct ipv4 import withdraws received")
	assert.EqualValues(t, 7, x.ImportWithdraws.Rejected, "Direct ipv4 import withdraws rejected")
	assert.EqualValues(t, 8, x.ImportWithdraws.Filtered, "Direct ipv4 import withdraws filtered")
	assert.EqualValues(t, 9, x.ImportWithdraws.Ignored, "Direct ipv4 import withdraws ignored")
	assert.EqualValues(t, 10, x.ImportWithdraws.Accepted, "Direct ipv4 import withdraws accepted")
	assert.EqualValues(t, 11, x.ExportUpdates.Received, "Direct ipv4 export updates received")
	assert.EqualValues(t, 12, x.ExportUpdates.Rejected, "Direct ipv4 export updates rejected")
	assert.EqualValues(t, 13, x.ExportUpdates.Filtered, "Direct ipv4 export updates filtered")
	assert.EqualValues(t, 14, x.ExportUpdates.Ignored, "Direct ipv4 export updates ignored")
	assert.EqualValues(t, 15, x.ExportUpdates.Accepted, "Direct ipv4 export updates accepted")
	assert.EqualValues(t, 16, x.ExportWithdraws.Received, "Direct ipv4 export withdraws received")
	assert.EqualValues(t, 17, x.ExportWithdraws.Rejected, "Direct ipv4 export withdraws rejected")
	assert.EqualValues(t, 18, x.ExportWithdraws.Filtered, "Direct ipv4 export withdraws filtered")
	assert.EqualValues(t, 19, x.ExportWithdraws.Ignored, "Direct ipv4 export withdraws ignored")
	assert.EqualValues(t, 0, x.ExportWithdraws.Accepted, "Direct ipv4 export withdraws accepted")

	x = p[2]
	assert.EqualValues(t, "direct1", x.Name, "Direct ipv6 name")
	assert.EqualValues(t, int(protocol.Direct), int(x.Proto), "Direct ipv6 proto")
	assert.EqualValues(t, "6", x.IPVersion, "Direct ipv6 ip version")
	assert.EqualValues(t, 3, x.Imported, "Direct ipv6 imported")
	assert.EqualValues(t, 5, x.Exported, "Direct ipv6 exported")
	assert.EqualValues(t, 7, x.Filtered, "Direct ipv6 filtered")
	assert.EqualValues(t, 13, x.Preferred, "Direct ipv6 preferred")
	assert.EqualValues(t, 20, x.ImportUpdates.Received, "Direct ipv6 import updates received")
	assert.EqualValues(t, 21, x.ImportUpdates.Rejected, "Direct ipv6 import updates rejected")
	assert.EqualValues(t, 22, x.ImportUpdates.Filtered, "Direct ipv6 import updates filtered")
	assert.EqualValues(t, 23, x.ImportUpdates.Ignored, "Direct ipv6 import updates ignored")
	assert.EqualValues(t, 24, x.ImportUpdates.Accepted, "Direct ipv6 import updates accepted")
	assert.EqualValues(t, 25, x.ImportWithdraws.Received, "Direct ipv6 import withdraws received")
	assert.EqualValues(t, 26, x.ImportWithdraws.Rejected, "Direct ipv6 import withdraws rejected")
	assert.EqualValues(t, 27, x.ImportWithdraws.Filtered, "Direct ipv6 import withdraws filtered")
	assert.EqualValues(t, 28, x.ImportWithdraws.Ignored, "Direct ipv6 import withdraws ignored")
	assert.EqualValues(t, 29, x.ImportWithdraws.Accepted, "Direct ipv6 import withdraws accepted")
	assert.EqualValues(t, 30, x.ExportUpdates.Received, "Direct ipv6 export updates received")
	assert.EqualValues(t, 31, x.ExportUpdates.Rejected, "Direct ipv6 export updates rejected")
	assert.EqualValues(t, 32, x.ExportUpdates.Filtered, "Direct ipv6 export updates filtered")
	assert.EqualValues(t, 33, x.ExportUpdates.Ignored, "Direct ipv6 export updates ignored")
	assert.EqualValues(t, 34, x.ExportUpdates.Accepted, "Direct ipv6 export updates accepted")
	assert.EqualValues(t, 35, x.ExportWithdraws.Received, "Direct ipv6 export withdraws received")
	assert.EqualValues(t, 36, x.ExportWithdraws.Rejected, "Direct ipv6 export withdraws rejected")
	assert.EqualValues(t, 37, x.ExportWithdraws.Filtered, "Direct ipv6 export withdraws filtered")
	assert.EqualValues(t, 38, x.ExportWithdraws.Ignored, "Direct ipv6 export withdraws ignored")
	assert.EqualValues(t, 0, x.ExportWithdraws.Accepted, "Direct ipv6 export withdraws accepted")

	x = p[3]
	assert.EqualValues(t, "ospf1", x.Name, "OSPF ipv4 name")
	assert.EqualValues(t, int(protocol.OSPF), int(x.Proto), "OSPF ipv4 proto")
	assert.EqualValues(t, "4", x.IPVersion, "OSPF ipv4 ip version")
	assert.EqualValues(t, 4, x.Imported, "OSPF ipv4 imported")
	assert.EqualValues(t, 2, x.Exported, "OSPF ipv4 exported")
	assert.EqualValues(t, 3, x.Filtered, "OSPF ipv4 filtered")
	assert.EqualValues(t, 1, x.Preferred, "OSPF ipv4 preferred")
}

func TestOSPFOldTimeFormat(t *testing.T) {
	data := "ospf1    OSPF      master   up     1481973060  Running\ntest\nbar\n  Routes:         12 imported, 34 exported, 100 preferred\nxxx"
	p := ParseProtocols([]byte(data), "4")
	assert.EqualValues(t, 1, len(p), "protocols")

	x := p[0]
	assert.EqualValues(t, "ospf1", x.Name, "name")
	assert.EqualValues(t, int(protocol.OSPF), int(x.Proto), "proto")
	assert.EqualValues(t, 1, x.Up, "up")
	assert.EqualValues(t, 12, x.Imported, "imported")
	assert.EqualValues(t, 34, x.Exported, "exported")
	assert.EqualValues(t, 100, x.Preferred, "preferred")
	assert.EqualValues(t, "4", x.IPVersion, "ipVersion")
}

func TestOSPFCurrentTimeFormat(t *testing.T) {
	data := "ospf1    OSPF      master   up     00:01:00  Running\ntest\nbar\n  Routes:         12 imported, 34 exported, 100 preferred\nxxx"
	p := ParseProtocols([]byte(data), "4")
	assert.EqualValues(t, 1, len(p), "protocols")

	x := p[0]
	assert.EqualValues(t, "ospf1", x.Name, "name")
	assert.EqualValues(t, int(protocol.OSPF), int(x.Proto), "proto")
	assert.EqualValues(t, 1, x.Up, "up")
	assert.EqualValues(t, 12, x.Imported, "imported")
	assert.EqualValues(t, 34, x.Exported, "exported")
	assert.EqualValues(t, 100, x.Preferred, "preferred")
	assert.EqualValues(t, "4", x.IPVersion, "ipVersion")
	assert.EqualValues(t, 60, x.Uptime, "uptime")
}

func TestOSPFIsoMsecTimeFormat(t *testing.T) {
	data := "ospf1    OSPF      master   up     2017-12-31 22:16:29.765  Running\ntest\nbar\n  Routes:         12 imported, 34 exported, 100 preferred\nxxx"
	p := ParseProtocols([]byte(data), "4")
	assert.EqualValues(t, 1, len(p), "protocols")

	x := p[0]
	assert.EqualValues(t, "ospf1", x.Name, "name")
	assert.EqualValues(t, int(protocol.OSPF), int(x.Proto), "proto")
	assert.EqualValues(t, 1, x.Up, "up")
	assert.EqualValues(t, 12, x.Imported, "imported")
	assert.EqualValues(t, 34, x.Exported, "exported")
	assert.EqualValues(t, 100, x.Preferred, "preferred")
	assert.EqualValues(t, "4", x.IPVersion, "ipVersion")
	assert.EqualValues(t, 13410, x.Uptime, "uptime")
}

func TestRPKIUp(t *testing.T) {
	data := "rpki1      RPKI       ---        up     2021-12-31 13:04:29  Established"
	p := ParseProtocols([]byte(data), "4")
	assert.EqualValues(t, 1, len(p), "protocols")

	x := p[0]
	assert.EqualValues(t, "rpki1", x.Name, "name")
	assert.EqualValues(t, int(protocol.RPKI), int(x.Proto), "proto")
	assert.EqualValues(t, "Established", x.State, "state")
	assert.EqualValues(t, 1, x.Up, "up")
}
