package client

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	birdsocket "github.com/czerwonk/bird_socket"
	"github.com/stretchr/testify/require"
)

func TestGetProtocolsHonorsQueryTimeout(t *testing.T) {
	path, wait := startBirdServer(t, "2002-incomplete\n", true)
	client := &BirdClient{Options: &BirdClientOptions{
		BirdV2:       true,
		BirdSocket:   path,
		Context:      context.Background(),
		QueryTimeout: 50 * time.Millisecond,
		MaxResponse:  4 << 20,
	}}

	_, err := client.GetProtocols()
	require.ErrorIs(t, err, context.DeadlineExceeded)
	wait()
}

func TestGetProtocolsHonorsResponseLimit(t *testing.T) {
	path, wait := startBirdServer(t, strings.Repeat("x", 256), false)
	client := &BirdClient{Options: &BirdClientOptions{
		BirdV2:       true,
		BirdSocket:   path,
		Context:      context.Background(),
		QueryTimeout: time.Second,
		MaxResponse:  64,
	}}

	_, err := client.GetProtocols()
	require.ErrorIs(t, err, birdsocket.ErrResponseTooLarge)
	wait()
}

func startBirdServer(t *testing.T, response string, holdOpen bool) (string, func()) {
	t.Helper()

	path := filepath.Join(os.TempDir(), fmt.Sprintf("bird_exporter_%d_%d.sock", os.Getpid(), time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()

		if _, writeErr := conn.Write([]byte("0001 BIRD ready.\n")); writeErr != nil {
			done <- writeErr
			return
		}
		if _, readErr := bufio.NewReader(conn).ReadString('\n'); readErr != nil {
			done <- readErr
			return
		}
		if _, writeErr := conn.Write([]byte(response)); writeErr != nil {
			done <- writeErr
			return
		}
		if holdOpen {
			buffer := make([]byte, 1)
			_, _ = conn.Read(buffer)
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
