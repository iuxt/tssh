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
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpectStopsWhenOutputCloses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out := make(chan []byte)
	errOut := make(chan []byte)
	close(out)
	close(errOut)
	expect := &sshExpect{ctx: ctx, out: out, err: errOut}
	_, err := expect.waitForPattern("ready", &caseSendList{})
	require.Error(t, err)
	assert.ErrorIs(t, err, io.EOF)
}

type expectBufferCloser struct{ bytes.Buffer }

func (w *expectBufferCloser) Close() error { return nil }

func TestExpectForwardsUnconsumedOutputOnReadError(t *testing.T) {
	for _, readError := range []error{io.EOF, errors.New("connection lost")} {
		t.Run(readError.Error(), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			expect := &sshExpect{ctx: ctx}
			ch := make(chan []byte, 10)
			writer := &expectBufferCloser{}
			reader := io.MultiReader(strings.NewReader("final output"), expectErrorReader{readError})
			expect.wrapOutput(reader, writer, ch)
			assert.Equal(t, "final output", writer.String())
		})
	}
}

type expectErrorReader struct{ err error }

func (r expectErrorReader) Read([]byte) (int, error) { return 0, r.err }
