//go:build !windows

package tssh

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
