//go:build !windows

package tssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trzsz/shellescape"
	"golang.org/x/crypto/ssh"
)

func TestProxyCommandTimeoutAndProcessCleanup(t *testing.T) {
	param := &sshParam{args: &sshArgs{Destination: "test-host"}, addr: "test-host:22", command: "sleep 10"}
	start := time.Now()
	_, err := connectViaProxyCommand(param, &ssh.ClientConfig{
		User: "test", HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 150 * time.Millisecond,
	})
	require.ErrorContains(t, err, "timed out")
	assert.Less(t, time.Since(start), 2*time.Second)

	conn, _, err := execProxyCommand(param)
	require.NoError(t, err)
	pipe := conn.(*cmdPipe)
	require.NoError(t, pipe.Close())
	assert.NotNil(t, pipe.cmd.ProcessState)
	require.NoError(t, pipe.Close())
}

func TestProxyCommandSlowAuthentication(t *testing.T) {
	addr := handshakeTestServer(t, nil)
	t.Setenv("TSSH_TEST_PROXY_ADDR", addr)
	executable, err := os.Executable()
	require.NoError(t, err)
	param := &sshParam{args: &sshArgs{Destination: "test-host"}, addr: addr,
		command: shellescape.QuoteCommand([]string{executable, "-test.run=^TestProxyConnectionHelper$"})}
	client, err := connectViaProxyCommand(param, &ssh.ClientConfig{
		User: "test", HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: time.Second,
		Auth: []ssh.AuthMethod{ssh.PasswordCallback(func() (string, error) {
			time.Sleep(1200 * time.Millisecond)
			return "password", nil
		})},
	})
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

// Run a byte-transparent proxy in a child test process, without relying on
// netcat or Python being installed on the developer's machine or CI runner.
func TestProxyConnectionHelper(t *testing.T) {
	addr := os.Getenv("TSSH_TEST_PROXY_ADDR")
	if addr == "" {
		return
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	go func() {
		_, _ = io.Copy(conn, os.Stdin)
		_ = conn.(*net.TCPConn).CloseWrite()
	}()
	_, _ = io.Copy(os.Stdout, conn)
	_ = conn.Close()
	os.Exit(0)
}
