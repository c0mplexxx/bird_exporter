package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOSPFWithErrorRejectsOversizeLine(t *testing.T) {
	areas, err := ParseOSPFWithError([]byte(strings.Repeat("x", maxProtocolLineBytes+1) + "\n"))
	require.Error(t, err)
	require.Empty(t, areas)
}

func TestOSPFArea(t *testing.T) {
	data := "ospf1:\n" +
		"RFC1583 compatibility: disabled\n" +
		"Stub router: No\n" +
		"RT scheduler tick: 1\n" +
		"Number of areas: 2\n" +
		"Number of LSAs in DB:   33\n" +
		"    Area: 0.0.0.0 (0) [BACKBONE]\n" +
		"        Stub:   No\n" +
		"        NSSA:   No\n" +
		"        Transit:    No\n" +
		"        Number of interfaces:   3\n" +
		"        Number of neighbors:    2\n" +
		"        Number of adjacent neighbors:   1\n" +
		"    Area: 0.0.0.1 (1)\n" +
		"        Stub:   No\n" +
		"        NSSA:   No\n" +
		"        Transit:    No\n" +
		"        Number of interfaces:   4\n" +
		"        Number of neighbors:    6\n" +
		"        Number of adjacent neighbors:   5\n"
	a := ParseOSPF([]byte(data))
	assert.EqualValues(t, 2, len(a), "areas")

	a1 := a[0]
	assert.EqualValues(t, "0", a1.Name, "Area1 Name")
	assert.EqualValues(t, 3, a1.InterfaceCount, "Area1 InterfaceCount")
	assert.EqualValues(t, 2, a1.NeighborCount, "Area1 NeighborCount")
	assert.EqualValues(t, 1, a1.NeighborAdjacentCount, "Area1 NeighborAdjacentCount")

	a2 := a[1]
	assert.EqualValues(t, "1", a2.Name, "Area2 Name")
	assert.EqualValues(t, 4, a2.InterfaceCount, "Area2 InterfaceCount")
	assert.EqualValues(t, 6, a2.NeighborCount, "Area2 NeighborCount")
	assert.EqualValues(t, 5, a2.NeighborAdjacentCount, "Area2 NeighborAdjacentCount")
}
