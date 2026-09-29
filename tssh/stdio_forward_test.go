/*
MIT License

Copyright (c) 2023-2026 The Trzsz SSH Authors.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package tssh

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stdioForwardTestClient struct {
	SshClient
	conn net.Conn
}

func (c *stdioForwardTestClient) DialTimeout(string, string, time.Duration) (net.Conn, error) {
	return c.conn, nil
}

func TestStdioForwardReturnsWhenRemoteCloses(t *testing.T) {
	previousInput, previousOutput := os.Stdin, os.Stdout
	input, inputWriter, err := os.Pipe()
	require.NoError(t, err)
	output, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(t, err)
	os.Stdin, os.Stdout = input, output
	defer func() {
		os.Stdin, os.Stdout = previousInput, previousOutput
		_ = inputWriter.Close()
		_ = input.Close()
		_ = output.Close()
	}()

	local, remote := net.Pipe()
	defer local.Close()
	done := make(chan error, 1)
	go func() {
		done <- stdioForward(&sshArgs{Destination: "host", Option: sshOption{options: map[string][]string{
			"connecttimeout": {"1"},
		}}},
			&stdioForwardTestClient{conn: local}, "remote:22")
	}()
	_ = remote.Close()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("stdio forwarding did not stop after the remote closed")
	}
}
