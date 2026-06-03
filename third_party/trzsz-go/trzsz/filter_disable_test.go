/*
MIT License

Copyright (c) 2022-2026 The Trzsz Authors.

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

package trzsz

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisableTrzszSkipsTriggerDetection(t *testing.T) {
	clientIn, clientInput := io.Pipe()
	clientOutput, clientOut := io.Pipe()
	serverIn, serverInput := io.Pipe()
	serverOut, serverOutput := io.Pipe()

	filter := NewTrzszFilter(clientIn, clientOut, serverInput, serverOut, TrzszOptions{
		DisableTrzsz: true,
	})
	defer filter.Close()
	defer func() { _ = clientInput.Close() }()
	defer func() { _ = serverIn.Close() }()

	trigger := []byte("prefix::TRZSZ:TRANSFER:R:1.0.0:0suffix")
	_, err := serverOutput.Write(trigger)
	require.NoError(t, err)
	require.NoError(t, serverOutput.Close())

	got, err := io.ReadAll(clientOutput)
	require.NoError(t, err)
	assert.Equal(t, trigger, got)
}
