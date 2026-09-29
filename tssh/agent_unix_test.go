//go:build !windows

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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentClientFollowsIdentityAgentForEachLogin(t *testing.T) {
	defer cleanupAfterLogin()
	dir, err := os.MkdirTemp("", "agent-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	firstPath := filepath.Join(dir, "first.sock")
	secondPath := filepath.Join(dir, "second.sock")
	firstListener, err := net.Listen("unix", firstPath)
	require.NoError(t, err)
	defer firstListener.Close()
	secondListener, err := net.Listen("unix", secondPath)
	require.NoError(t, err)
	defer secondListener.Close()

	param := func(path string) *sshParam {
		return &sshParam{args: &sshArgs{Destination: "host", Option: sshOption{options: map[string][]string{
			"identityagent": {path},
		}}}}
	}
	first := getAgentClient(param(firstPath))
	second := getAgentClient(param(secondPath))
	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.NotSame(t, first, second)

	cleanupAfterLogin()
	assert.NotNil(t, getAgentClient(param(firstPath)))
}
