package ssh_config

import (
	"strings"
	"testing"
)

var validateTests = []struct {
	key string
	val string
	err string
}{
	{"IdentitiesOnly", "yes", ""},
	{"IdentitiesOnly", "Yes", `ssh_config: value for key "IdentitiesOnly" must be 'yes' or 'no', got "Yes"`},
	{"Port", "22", ``},
	{"Port", "yes", `ssh_config: strconv.ParseUint: parsing "yes": invalid syntax`},
}

func TestValidate(t *testing.T) {
	for _, tt := range validateTests {
		err := validate(tt.key, tt.val)
		if tt.err == "" && err != nil {
			t.Errorf("validate(%q, %q): got %v, want nil", tt.key, tt.val, err)
		}
		if tt.err != "" {
			if err == nil {
				t.Errorf("validate(%q, %q): got nil error, want %v", tt.key, tt.val, tt.err)
			} else if err.Error() != tt.err {
				t.Errorf("validate(%q, %q): got err %v, want %v", tt.key, tt.val, err, tt.err)
			}
		}
	}
}

func TestDefault(t *testing.T) {
	if v := Default("VisualHostKey"); v != "no" {
		t.Errorf("Default(%q): got %v, want 'no'", "VisualHostKey", v)
	}
	if v := Default("visualhostkey"); v != "no" {
		t.Errorf("Default(%q): got %v, want 'no'", "visualhostkey", v)
	}
	if v := Default("notfound"); v != "" {
		t.Errorf("Default(%q): got %v, want ''", "notfound", v)
	}
}

func TestDefaultsSnapshot(t *testing.T) {
	snapshot := Defaults()
	if len(snapshot) != len(defaults) {
		t.Fatalf("missing defaults: %d != %d", len(snapshot), len(defaults))
	}
	for key, value := range snapshot {
		if value != Default(key) {
			t.Errorf("default mismatch for %s", key)
		}
	}
	snapshot["port"] = "12345"
	if Default("Port") != "22" {
		t.Fatal("snapshot changed registered defaults")
	}
}

func TestSupportsMultiple(t *testing.T) {
	for _, key := range []string{"CertificateFile", "IdentityFile", "DynamicForward", "RemoteForward", "SendEnv", "SetEnv"} {
		if !SupportsMultiple(key) || !SupportsMultiple(strings.ToLower(key)) {
			t.Errorf("SupportsMultiple(%q) should be true regardless of case", key)
		}
	}
	if SupportsMultiple("Port") {
		t.Fatal("Port must not support multiple values")
	}
}
