package tssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func handshakeTestServer(t *testing.T, wrap func(net.Conn) net.Conn) string {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)
	config := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil }}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		if wrap != nil {
			conn = wrap(conn)
		}
		defer conn.Close()
		t.Cleanup(func() { _ = conn.Close() })
		server, channels, requests, err := ssh.NewServerConn(conn, config)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(requests)
		go func() {
			for ch := range channels {
				_ = ch.Reject(ssh.UnknownChannelType, "unused")
			}
		}()
		_ = server.Wait()
	}()
	return listener.Addr().String()
}

func TestHandshakeTimeoutExcludesInteractiveCallbacks(t *testing.T) {
	for _, interactive := range []string{"password", "host key"} {
		t.Run(interactive, func(t *testing.T) {
			addr := handshakeTestServer(t, nil)
			conn, err := net.Dial("tcp", addr)
			require.NoError(t, err)
			defer conn.Close()
			config := &ssh.ClientConfig{User: "test", Timeout: 200 * time.Millisecond, HostKeyCallback: ssh.InsecureIgnoreHostKey()}
			config.Auth = []ssh.AuthMethod{ssh.PasswordCallback(func() (string, error) {
				if interactive == "password" {
					time.Sleep(350 * time.Millisecond)
				}
				return "password", nil
			})}
			if interactive == "host key" {
				config.HostKeyCallback = func(string, net.Addr, ssh.PublicKey) error { time.Sleep(350 * time.Millisecond); return nil }
			}
			client, _, _, err := newClientConnWithTimeout(conn, addr, config)
			require.NoError(t, err)
			require.NoError(t, client.Close())
		})
	}
}

func TestDirectHandshakeTimesOutAfterBanner(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.WriteString(conn, "SSH-2.0-stalled\r\n")
		<-stop
	}()
	param := &sshParam{args: &sshArgs{Destination: "host", IPv4Only: true}, addr: listener.Addr().String()}
	start := time.Now()
	_, err = connectDirectly(param, &ssh.ClientConfig{User: "test", Timeout: 150 * time.Millisecond, HostKeyCallback: ssh.InsecureIgnoreHostKey()})
	require.ErrorContains(t, err, "timed out")
	assert.Less(t, time.Since(start), 2*time.Second)
}

type stallNewKeysConn struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
}

func (c *stallNewKeysConn) Write(p []byte) (int, error) {
	if len(p) > 5 && p[5] == 21 {
		<-c.closed
		return 0, net.ErrClosed
	}
	return c.Conn.Write(p)
}
func (c *stallNewKeysConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func TestHandshakeTimeoutResumesAfterHostVerification(t *testing.T) {
	addr := handshakeTestServer(t, func(conn net.Conn) net.Conn { return &stallNewKeysConn{Conn: conn, closed: make(chan struct{})} })
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()
	config := &ssh.ClientConfig{User: "test", Timeout: 200 * time.Millisecond, HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error { time.Sleep(350 * time.Millisecond); return nil }}
	start := time.Now()
	_, _, _, err = newClientConnWithTimeout(conn, addr, config)
	require.ErrorContains(t, err, "timed out")
	assert.GreaterOrEqual(t, time.Since(start), 350*time.Millisecond)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestInitialPacketObserverHandlesFragmentation(t *testing.T) {
	packet := func(kind byte) []byte {
		buf := make([]byte, 16)
		binary.BigEndian.PutUint32(buf[:4], 12)
		buf[4], buf[5] = 10, kind
		return buf
	}
	data := append([]byte("server notice\r\nSSH-2.0-test\r\n"), packet(20)...)
	data = append(data, packet(21)...)
	data = append(data, []byte("encrypted authentication bytes")...)
	for _, chunk := range []int{1, 3, 6, len(data)} {
		c := &handshakeTimeoutConn{}
		for remaining := data; len(remaining) > 0; {
			n := min(chunk, len(remaining))
			c.observe(remaining[:n])
			remaining = remaining[n:]
		}
		assert.True(t, c.finished, "chunk size %d", chunk)
	}
}
