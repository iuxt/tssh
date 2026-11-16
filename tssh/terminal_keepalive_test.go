package tssh

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trzsz/ssh_config"
)

func TestTerminalKeepAliveConfig(t *testing.T) {
	previous := userConfig
	userConfig = &tsshConfig{}
	t.Cleanup(func() { userConfig = previous })
	for _, test := range []struct {
		name     string
		options  map[string][]string
		interval time.Duration
		chars    string
		invalid  bool
	}{
		{name: "disabled by default"},
		{name: "explicitly disabled", options: map[string][]string{"enableterminalkeepalive": {"no"}}},
		{name: "defaults", options: map[string][]string{"enableterminalkeepalive": {"yes"}}, interval: time.Minute, chars: " "},
		{name: "custom", options: map[string][]string{"enableterminalkeepalive": {"on"}, "terminalkeepaliveinterval": {"30"}, "terminalkeepalivechars": {`\x20\b\t\r\n\\中文`}}, interval: 30 * time.Second, chars: " \b\t\r\n\\中文"},
		{name: "zero interval", options: map[string][]string{"enableterminalkeepalive": {"yes"}, "terminalkeepaliveinterval": {"0"}}, invalid: true},
		{name: "negative interval", options: map[string][]string{"enableterminalkeepalive": {"yes"}, "terminalkeepaliveinterval": {"-1"}}, invalid: true},
		{name: "invalid interval", options: map[string][]string{"enableterminalkeepalive": {"yes"}, "terminalkeepaliveinterval": {"abc"}}, invalid: true},
		{name: "overflow interval", options: map[string][]string{"enableterminalkeepalive": {"yes"}, "terminalkeepaliveinterval": {"4294967296"}}, invalid: true},
		{name: "invalid escape", options: map[string][]string{"enableterminalkeepalive": {"yes"}, "terminalkeepalivechars": {`\z`}}, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			config, err := getTerminalKeepAliveConfig(&sshArgs{Option: sshOption{options: test.options}})
			if test.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.interval, config.interval)
			require.Equal(t, test.chars, string(config.chars))
		})
	}
}

func TestTerminalKeepAliveHostConfigAndOverride(t *testing.T) {
	previous := userConfig
	defer func() { userConfig = previous }()
	config, err := ssh_config.Decode(strings.NewReader("Host test\n EnableTerminalKeepAlive yes\n TerminalKeepAliveInterval 45\n TerminalKeepAliveChars \\x20\\b\n"))
	require.NoError(t, err)
	userConfig = &tsshConfig{config: config}
	args := &sshArgs{Destination: "test"}
	settings, err := getTerminalKeepAliveConfig(args)
	require.NoError(t, err)
	require.Equal(t, 45*time.Second, settings.interval)
	require.Equal(t, []byte(" \b"), settings.chars)
	args.Option.options = map[string][]string{"enableterminalkeepalive": {"no"}}
	settings, err = getTerminalKeepAliveConfig(args)
	require.NoError(t, err)
	require.Zero(t, settings.interval)
}

type keepAliveBuffer struct {
	bytes.Buffer
	closed bool
}

func (b *keepAliveBuffer) Close() error { b.closed = true; return nil }

func TestTerminalKeepAliveIdlePauseAndClose(t *testing.T) {
	buffer := &keepAliveBuffer{}
	start := time.Now()
	paused := false
	w := &terminalKeepAliveWriter{WriteCloser: buffer,
		config:    terminalKeepAliveConfig{interval: time.Minute, chars: []byte(" ")},
		lastInput: start, done: make(chan struct{}), paused: func() bool { return paused }}
	delay, err := w.sendIfIdle(start.Add(10 * time.Second))
	require.NoError(t, err)
	require.Equal(t, 50*time.Second, delay)
	require.Empty(t, buffer.String())
	_, err = w.sendIfIdle(start.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, " ", buffer.String())
	paused = true // Console or file transfer is active.
	_, err = w.sendIfIdle(start.Add(2 * time.Minute))
	require.NoError(t, err)
	require.Equal(t, " ", buffer.String())
	paused = false
	_, err = w.sendIfIdle(start.Add(3 * time.Minute))
	require.NoError(t, err)
	require.Equal(t, "  ", buffer.String())
	_, err = w.Write([]byte("user input"))
	require.NoError(t, err)
	delay, err = w.sendIfIdle(w.lastInput.Add(5 * time.Second))
	require.NoError(t, err)
	require.Equal(t, 55*time.Second, delay)
	require.Equal(t, "  user input", buffer.String())
	require.NoError(t, w.Close())
	require.True(t, buffer.closed)
	_, err = w.sendIfIdle(start.Add(4 * time.Minute))
	require.ErrorIs(t, err, io.ErrClosedPipe)
	_, err = w.Write([]byte("after close"))
	require.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestTerminalKeepAliveTimerAndBlockedWriteShutdown(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	w := &terminalKeepAliveWriter{WriteCloser: writer,
		config:    terminalKeepAliveConfig{interval: time.Millisecond, chars: []byte(" ")},
		lastInput: time.Now(), done: make(chan struct{})}
	exited := make(chan struct{})
	go func() { w.run(); close(exited) }()
	received := make(chan byte, 1)
	go func() {
		var b [1]byte
		if _, err := io.ReadFull(reader, b[:]); err == nil {
			received <- b[0]
		}
	}()
	select {
	case ch := <-received:
		require.Equal(t, byte(' '), ch)
	case <-time.After(2 * time.Second):
		t.Fatal("no automatic character received")
	}
	// The next automatic write may already be blocked on the pipe.
	require.NoError(t, w.Close())
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("keep alive did not exit after closing a blocked writer")
	}
}

func TestTerminalKeepAliveSkipsNonInteractiveSessions(t *testing.T) {
	previousTerminal := isTerminal
	defer func() { isTerminal = previousTerminal }()
	for _, test := range []struct {
		name     string
		terminal bool
		tty      bool
	}{
		{"piped input", false, true},
		{"no remote tty", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			isTerminal = test.terminal
			buffer := &keepAliveBuffer{}
			conn := &sshConnection{serverIn: buffer, tty: test.tty}
			stop := setupTerminalKeepAlive(conn)
			defer stop()
			require.Same(t, buffer, conn.serverIn)
		})
	}
}

func TestTerminalKeepAliveStopsOnWriteError(t *testing.T) {
	reader, writer := io.Pipe()
	require.NoError(t, reader.Close())
	defer writer.Close()
	w := &terminalKeepAliveWriter{WriteCloser: writer,
		config:    terminalKeepAliveConfig{interval: time.Millisecond, chars: []byte(" ")},
		lastInput: time.Now(), done: make(chan struct{})}
	exited := make(chan struct{})
	go func() { w.run(); close(exited) }()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("keep alive kept running after a write failure")
	}
}
