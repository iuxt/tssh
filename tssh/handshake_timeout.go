package tssh

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// ConnectTimeout covers version exchange and the initial key exchange, not
// interactive host verification or user authentication. The initial packets
// are unencrypted, so SSH_MSG_NEWKEYS marks the end of the timed phase without
// relying on private x/crypto APIs or wrapping individual authentication methods.
type handshakeTimeoutConn struct {
	net.Conn
	mu        sync.Mutex
	timer     *time.Timer
	remaining time.Duration
	deadline  time.Time
	epoch     uint64
	finished  bool
	expired   bool

	// The SSH transport has one reader. These fields are only used by Read.
	version      bool
	linePrefix   [4]byte
	lineLength   int
	header       [6]byte // packet length, padding length, message type
	headerLength int
	packetLeft   uint32
	newKeys      bool
	packetDone   bool
}

func (c *handshakeTimeoutConn) armLocked() {
	c.epoch++
	epoch := c.epoch
	c.deadline = time.Now().Add(c.remaining)
	c.timer = time.AfterFunc(max(0, c.remaining), func() {
		c.mu.Lock()
		if c.finished || c.epoch != epoch {
			c.mu.Unlock()
			return
		}
		c.expired = true
		c.mu.Unlock()
		_ = c.Conn.Close()
	})
}

func (c *handshakeTimeoutConn) pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.finished || c.expired || c.timer == nil {
		return
	}
	c.remaining = time.Until(c.deadline)
	c.epoch++
	c.timer.Stop()
	c.timer = nil
}

func (c *handshakeTimeoutConn) resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.finished && !c.expired && c.timer == nil {
		c.armLocked()
	}
}

func (c *handshakeTimeoutConn) finish() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.finished = true
	c.epoch++
	if c.timer != nil {
		c.timer.Stop()
	}
}

func (c *handshakeTimeoutConn) observe(data []byte) {
	for len(data) > 0 && !c.packetDone {
		if !c.version {
			ch := data[0]
			data = data[1:]
			if c.lineLength < len(c.linePrefix) {
				c.linePrefix[c.lineLength] = ch
			}
			c.lineLength++
			if ch == '\n' {
				c.version = c.lineLength >= 4 && string(c.linePrefix[:]) == "SSH-"
				c.lineLength = 0
			}
			continue
		}
		if c.headerLength < len(c.header) {
			n := copy(c.header[c.headerLength:], data)
			c.headerLength += n
			data = data[n:]
			if c.headerLength < len(c.header) {
				continue
			}
			length := binary.BigEndian.Uint32(c.header[:4])
			if length < 6 || length > 256*1024 || uint32(c.header[4])+2 > length {
				// Leave validation to ssh; malformed framing must not cancel
				// the timeout or cause unbounded allocations here.
				c.packetDone = true
				return
			}
			c.packetLeft = length - 2
			c.newKeys = c.header[5] == 21 && length-uint32(c.header[4])-1 == 1
		}
		n := min(len(data), int(c.packetLeft))
		c.packetLeft -= uint32(n)
		data = data[n:]
		if c.packetLeft == 0 {
			c.headerLength = 0
			if c.newKeys {
				c.packetDone = true
				c.finish()
			}
		}
	}
}

func (c *handshakeTimeoutConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		lastServerAliveTime.Store(time.Now().UnixMilli())
		c.observe(p[:n])
	}
	return n, err
}

func newClientConnWithTimeout(conn net.Conn, addr string, config *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	c := &handshakeTimeoutConn{Conn: conn, remaining: config.Timeout, finished: config.Timeout <= 0}
	if config.Timeout > 0 {
		c.mu.Lock()
		c.armLocked()
		c.mu.Unlock()
	}
	defer c.finish()
	copyConfig := *config
	if callback := config.HostKeyCallback; callback != nil {
		copyConfig.HostKeyCallback = func(host string, remote net.Addr, key ssh.PublicKey) error {
			c.pause()
			defer c.resume()
			return callback(host, remote, key)
		}
	}
	sshConn, channels, requests, err := ssh.NewClientConn(c, addr, &copyConfig)
	c.finish()
	c.mu.Lock()
	expired := c.expired
	c.mu.Unlock()
	if expired {
		if sshConn != nil {
			_ = sshConn.Close()
		}
		if err == nil {
			err = net.ErrClosed
		}
		return nil, nil, nil, fmt.Errorf("connection timed out after %s: %w", config.Timeout, err)
	}
	return sshConn, channels, requests, err
}
