package tssh

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

type terminalKeepAliveConfig struct {
	interval time.Duration
	chars    []byte
}

func getTerminalKeepAliveConfig(args *sshArgs) (terminalKeepAliveConfig, error) {
	if !isConfigYes(getExOptionConfig(args, "EnableTerminalKeepAlive")) {
		return terminalKeepAliveConfig{}, nil
	}
	config := terminalKeepAliveConfig{interval: time.Minute, chars: []byte(" ")}
	if value := getExOptionConfig(args, "TerminalKeepAliveInterval"); value != "" {
		seconds, err := strconv.ParseUint(value, 10, 32)
		if err != nil || seconds == 0 {
			return terminalKeepAliveConfig{}, fmt.Errorf("TerminalKeepAliveInterval must be a positive number of seconds")
		}
		config.interval = time.Duration(seconds) * time.Second
	}
	if value := getExOptionConfig(args, "TerminalKeepAliveChars"); value != "" {
		chars, err := decodeTerminalKeepAliveChars(value)
		if err != nil {
			return terminalKeepAliveConfig{}, err
		}
		config.chars = chars
	}
	return config, nil
}

func decodeTerminalKeepAliveChars(value string) ([]byte, error) {
	var chars strings.Builder
	for value != "" {
		ch, multibyte, rest, err := strconv.UnquoteChar(value, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid TerminalKeepAliveChars escape: %w", err)
		}
		if multibyte {
			chars.WriteRune(ch)
		} else {
			chars.WriteByte(byte(ch))
		}
		value = rest
	}
	return []byte(chars.String()), nil
}

// All writes, including console input, share a lock so automatic characters
// cannot split a user's input. Close must remain able to unblock a stuck write.
type terminalKeepAliveWriter struct {
	io.WriteCloser
	config    terminalKeepAliveConfig
	paused    func() bool
	mu        sync.Mutex
	lastInput time.Time
	done      chan struct{}
	stopOnce  sync.Once
}

func (w *terminalKeepAliveWriter) stop() {
	w.stopOnce.Do(func() { close(w.done) })
}

func (w *terminalKeepAliveWriter) Close() error {
	w.stop()
	return w.WriteCloser.Close()
}

func (w *terminalKeepAliveWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.done:
		return 0, io.ErrClosedPipe
	default:
	}
	n, err := w.WriteCloser.Write(data)
	if n > 0 {
		w.lastInput = time.Now()
	}
	return n, err
}

func (w *terminalKeepAliveWriter) sendIfIdle(now time.Time) (time.Duration, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.done:
		return 0, io.ErrClosedPipe
	default:
	}
	if remaining := w.config.interval - now.Sub(w.lastInput); remaining > 0 {
		return remaining, nil
	}
	if w.paused != nil && w.paused() {
		return w.config.interval, nil
	}
	if err := writeAll(w.WriteCloser, w.config.chars); err != nil {
		return 0, err
	}
	w.lastInput = now
	return w.config.interval, nil
}

func (w *terminalKeepAliveWriter) run() {
	timer := time.NewTimer(w.config.interval)
	defer timer.Stop()
	defer w.stop()
	for {
		select {
		case <-w.done:
			return
		case now := <-timer.C:
			delay, err := w.sendIfIdle(now)
			if err != nil {
				debug("terminal keep alive stopped: %v", err)
				return
			}
			timer.Reset(delay)
		}
	}
}

// Install after expect and the transfer filter, so characters follow the same
// input path as keystrokes and never bypass the file transfer protocol.
func setupTerminalKeepAlive(conn *sshConnection) func() {
	if !isTerminal || !conn.tty || conn.serverIn == nil {
		return func() {}
	}
	config, err := getTerminalKeepAliveConfig(conn.param.args)
	if err != nil {
		warning("terminal keep alive disabled: %v", err)
		return func() {}
	}
	if config.interval == 0 {
		return func() {}
	}
	w := &terminalKeepAliveWriter{
		WriteCloser: conn.serverIn, config: config, lastInput: time.Now(),
		done: make(chan struct{}),
		paused: func() bool {
			return conn.closed.Load() || pauseOutput.Load() ||
				(conn.isTransferringFiles != nil && conn.isTransferringFiles())
		},
	}
	conn.serverIn = w
	addOnExitFunc(w.stop)
	go w.run()
	return w.stop
}
