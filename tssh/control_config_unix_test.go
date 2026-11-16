//go:build !windows

package tssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControlMasterUsesActiveConfiguration(t *testing.T) {
	previous := userConfig
	defer func() { userConfig = previous }()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	included := filepath.Join(dir, "included")
	require.NoError(t, os.WriteFile(included, []byte("Host prod\n IdentityFile ~/review-key\n EnableSessionLog yes\nHost other\n 6e616d653a20Command unused-secret-command\n"), 0600))
	require.NoError(t, os.WriteFile(path, []byte("IgnoreUnknown ReviewCustom\nReviewCustom value\nInclude "+included+"\nHost prod\n HostName config-host\n User config-user\n Port 2200\n GroupLabels production\n ControlMaster auto\n ControlPath /tmp/review-%C\n PasswordCommand review-secret-command\n"), 0600))
	userConfig = &tsshConfig{configPath: path}
	args := &sshArgs{Destination: "dest-user@prod:2300", originalDest: "dest-user@prod:2300"}
	param, err := getSshParam(args, false)
	require.NoError(t, err)
	argv := controlMasterArgs(param)
	assert.Contains(t, argv, path)
	assert.Contains(t, argv, "-oHostName=config-host")
	assert.Contains(t, argv, "dest-user")
	assert.Contains(t, argv, "2300")
	assert.Equal(t, "prod", argv[len(argv)-2])
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("OpenSSH unavailable for config validation")
	}
	// -G evaluates the child configuration without connecting or executing the
	// remote command. Extensions in both the root and Include must be accepted.
	out, err := exec.Command("ssh", append([]string{"-G"}, argv...)...).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "hostname config-host\n")
	assert.Contains(t, string(out), "user dest-user\n")
	assert.Contains(t, string(out), "port 2300\n")
	assert.True(t, strings.Contains(string(out), "identityfile ~/review-key"))

	// A ProxyJump child inherits only -F, so the extension rules must also
	// live in the wrapper file rather than solely in the parent's argv.
	content, err := controlConfigContent(args)
	require.NoError(t, err)
	wrapper := filepath.Join(dir, "wrapper config")
	require.NoError(t, os.WriteFile(wrapper, []byte(content), 0600))
	out, err = exec.Command("ssh", "-G", "-F", wrapper, "other").CombinedOutput()
	require.NoError(t, err, string(out))
}
