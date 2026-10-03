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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentValuesKeepWhitespace(t *testing.T) {
	previous := userConfig
	defer func() { userConfig = previous }()
	userConfig = &tsshConfig{}
	t.Setenv("TSSH_PADDED_VALUE", "  padded  ")
	args := &sshArgs{Destination: "host", Option: sshOption{options: map[string][]string{
		"sendenv": {"TSSH_PADDED_VALUE"},
		"setenv":  {`OTHER="  spaced  "`},
	}}}
	sent, err := getSendEnvs(args)
	require.NoError(t, err)
	require.Len(t, sent, 1)
	assert.Equal(t, "  padded  ", sent[0].value)

	set, err := getSetEnvs(args)
	require.NoError(t, err)
	require.Len(t, set, 1)
	assert.Equal(t, "  spaced  ", set[0].value)
}

func TestSetEnvCollectsRepeatedOptionsAndConfig(t *testing.T) {
	previous := userConfig
	defer func() { userConfig = previous }()
	configPath := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(configPath, []byte("Host host\n  SetEnv FROM_CONFIG=one\n  SetEnv SECOND_CONFIG=two\n"), 0600))
	userConfig = &tsshConfig{configPath: configPath}
	args := &sshArgs{Destination: "host", Option: sshOption{options: map[string][]string{
		"setenv": {"FROM_OPTION=three", "SECOND_OPTION=four"},
	}}}
	values, err := getSetEnvs(args)
	require.NoError(t, err)
	require.Len(t, values, 4)
	assert.Equal(t, []string{"FROM_OPTION", "SECOND_OPTION", "FROM_CONFIG", "SECOND_CONFIG"},
		[]string{values[0].name, values[1].name, values[2].name, values[3].name})
}
