package ssh_config

import (
	"strings"
	"testing"
)

func TestMatchCompoundConditionsDoNotBroadenScope(t *testing.T) {
	for _, condition := range []string{
		"host prod user alice", "host prod localuser alice", "host prod exec false",
		"host prod !exec false", "host prod originalhost other", "host prod final",
		"host prod localnetwork 127.0.0.0/8", "all user alice",
	} {
		t.Run(condition, func(t *testing.T) {
			config, err := Decode(strings.NewReader("Match " + condition + "\n Password restricted\n Include /invalid/[\nHost *\n User fallback\n"))
			if err != nil {
				t.Fatal(err)
			}
			for _, host := range []string{"prod", "user", "alice", "other"} {
				got, err := config.Get(host, "Password")
				if err != nil || got != "" {
					t.Fatalf("restricted Password applied to %s: %q, %v", host, got, err)
				}
				got, err = config.Get(host, "User")
				if err != nil || got != "fallback" {
					t.Fatalf("following Host block lost: %q, %v", got, err)
				}
			}
		})
	}
}

func TestMatchCommaSeparatedPatterns(t *testing.T) {
	config, err := Decode(strings.NewReader("Match host \"prod*,dev*,!prod-secret\"\n User deploy\n"))
	if err != nil {
		t.Fatal(err)
	}
	for host, want := range map[string]string{"prod1": "deploy", "dev1": "deploy", "prod-secret": "", "other": ""} {
		got, err := config.Get(host, "User")
		if err != nil || got != want {
			t.Errorf("%s: got %q, want %q (%v)", host, got, want, err)
		}
	}
}

func TestQuestionMarkMatchesExactlyOneCharacter(t *testing.T) {
	pattern, err := NewPattern("prod?")
	if err != nil {
		t.Fatal(err)
	}
	for host, want := range map[string]bool{"prod": false, "prod1": true, "prod12": false, "prod中": true} {
		if got := pattern.Regex().MatchString(host); got != want {
			t.Errorf("%s: got %v, want %v", host, got, want)
		}
	}
}

func TestHostPatternsSeparatedByTabs(t *testing.T) {
	config, err := Decode(strings.NewReader("Host first\tsecond \tthird\n User deploy\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"first", "second", "third"} {
		if got, err := config.Get(host, "User"); err != nil || got != "deploy" {
			t.Errorf("%s: got %q (%v)", host, got, err)
		}
	}
}
