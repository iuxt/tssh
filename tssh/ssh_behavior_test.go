package tssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func newExitStatusTestClient(t *testing.T, requestType string, status uint32) SshClient {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)
	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		serverConn, err := listener.Accept()
		if err != nil {
			return
		}
		server, channels, requests, err := ssh.NewServerConn(serverConn, serverConfig)
		if err != nil {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for request := range channels {
			if request.ChannelType() != "session" {
				_ = request.Reject(ssh.UnknownChannelType, "unexpected channel")
				continue
			}
			channel, channelRequests, err := request.Accept()
			if err != nil {
				return
			}
			for channelRequest := range channelRequests {
				if channelRequest.Type != requestType {
					_ = channelRequest.Reply(false, nil)
					continue
				}
				_ = channelRequest.Reply(true, nil)
				_, _ = channel.Write([]byte("remote output"))
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
				_ = channel.Close()
				return
			}
		}
	}()
	clientConn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	conn, channels, requests, err := ssh.NewClientConn(clientConn, "test-host:22", &ssh.ClientConfig{
		User:            "test",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	require.NoError(t, err)
	client := sshNewClient(conn, channels, requests)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRemoteExitStatus(t *testing.T) {
	client := newExitStatusTestClient(t, "exec", 42)
	session, err := client.NewSession()
	require.NoError(t, err)
	defer session.Close()
	require.NoError(t, session.Start("false"))
	require.Error(t, session.Wait())
	assert.Equal(t, 42, session.GetExitCode())
}

func TestSubsystemReturnsRemoteStatusWithoutStdinEOF(t *testing.T) {
	client := newExitStatusTestClient(t, "subsystem", 42)
	stdin, stdinWriter, err := os.Pipe()
	require.NoError(t, err)
	stdout, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	require.NoError(t, err)
	stderr, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	require.NoError(t, err)
	previousStdin, previousStdout, previousStderr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = stdin, stdout, stderr
	defer func() {
		os.Stdin, os.Stdout, os.Stderr = previousStdin, previousStdout, previousStderr
		_ = stdinWriter.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
	}()
	type result struct {
		code int
		err  error
	}
	done := make(chan result, 1)
	go func() {
		code, err := subsystemForward(client, "sftp")
		done <- result{code, err}
	}()
	select {
	case result := <-done:
		require.NoError(t, result.err)
		assert.Equal(t, 42, result.code)
	case <-time.After(2 * time.Second):
		_ = stdinWriter.Close()
		t.Fatal("subsystem waited for stdin EOF after the remote exit")
	}
	_, err = stdout.WriteString("still open")
	require.NoError(t, err)
	_, err = stderr.WriteString("still open")
	require.NoError(t, err)
}

func TestExitOnForwardFailure(t *testing.T) {
	previousConfig := userConfig
	userConfig = &tsshConfig{}
	defer func() { userConfig = previousConfig }()
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer occupied.Close()
	bindAddr := "127.0.0.1"
	port := occupied.Addr().(*net.TCPAddr).Port
	forward := &forwardCfg{
		argument: "127.0.0.1:" + strconv.Itoa(port) + ":127.0.0.1:22",
		bindAddr: &bindAddr, bindPort: port, destHost: "127.0.0.1", destPort: 22,
	}
	args := &sshArgs{Destination: "test-host", LocalForward: forwardArgs{cfgs: []*forwardCfg{forward}},
		Option: sshOption{options: map[string][]string{"exitonforwardfailure": {"yes"}}}}
	connection := &sshConnection{param: &sshParam{args: args}}
	require.Error(t, sshPortForward(connection))
	args.Option.options["exitonforwardfailure"] = []string{"no"}
	require.NoError(t, sshPortForward(connection))
}
