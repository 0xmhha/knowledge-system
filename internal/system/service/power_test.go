package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// pmsetOutput is a real `pmset -g custom` capture from a Mac mini serving this
// deployment: idle sleep on, so the host drops off the network by itself.
const pmsetOutput = `AC Power:
 Sleep On Power Button 1
 autorestartatconnect 0
 lowpowermode         0
 standby              0
 powernap             1
 ttyskeepawake        1
 womp                 1
 displaysleep         10
 networkoversleep     0
 sleep                1
 autorestart          0
 tcpkeepalive         1
 disksleep            10
`

func TestParsePMSet(t *testing.T) {
	profiles := ParsePMSet(pmsetOutput)
	ac, ok := profiles[acProfile]
	if !ok {
		t.Fatalf("no %q profile in %v", acProfile, profiles)
	}
	cases := []struct {
		setting Setting
		want    int
	}{
		{SettingSleep, 1},
		{SettingStandby, 0},
		{SettingWakeOnLAN, 1},
		{SettingAutoRestart, 0},
	}
	for _, tc := range cases {
		t.Run(string(tc.setting), func(t *testing.T) {
			if got := ac[tc.setting]; got != tc.want {
				t.Errorf("%s = %d, want %d", tc.setting, got, tc.want)
			}
		})
	}
	if _, present := ac[Setting("Sleep")]; present {
		t.Error("a multi-word setting name was parsed as a value")
	}
}

func TestParsePMSetSeparatesProfiles(t *testing.T) {
	out := pmsetOutput + "\nBattery Power:\n sleep 5\n"
	profiles := ParsePMSet(out)
	if got := profiles[acProfile][SettingSleep]; got != 1 {
		t.Errorf("AC sleep = %d, want 1", got)
	}
	if got := profiles["Battery Power"][SettingSleep]; got != 5 {
		t.Errorf("battery sleep = %d, want 5 — profiles must not bleed into each other", got)
	}
}

func TestViolations(t *testing.T) {
	cases := []struct {
		name     string
		profile  Profile
		wantKeys []Setting
	}{
		{
			name:     "the observed host",
			profile:  ParsePMSet(pmsetOutput)[acProfile],
			wantKeys: []Setting{SettingSleep, SettingAutoRestart},
		},
		{
			name: "a compliant host",
			profile: Profile{
				SettingSleep: 0, SettingStandby: 0, SettingWakeOnLAN: 1, SettingAutoRestart: 1,
			},
			wantKeys: nil,
		},
		{
			name:     "settings the host does not expose are not violations",
			profile:  Profile{SettingSleep: 0},
			wantKeys: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Violations(tc.profile)
			if len(got) != len(tc.wantKeys) {
				t.Fatalf("got %d violations %v, want %d", len(got), got, len(tc.wantKeys))
			}
			for i, want := range tc.wantKeys {
				if got[i].Setting != want {
					t.Errorf("violation %d = %s, want %s (policy order)", i, got[i].Setting, want)
				}
			}
		})
	}
}

func TestRemediationCommand(t *testing.T) {
	got := RemediationCommand(Violations(ParsePMSet(pmsetOutput)[acProfile]))
	want := "sudo /usr/bin/pmset -a sleep 0 autorestart 1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if RemediationCommand(nil) != "" {
		t.Error("nothing to fix must produce no command")
	}
}

// fakeRunner records calls and replays a canned result.
type fakeRunner struct {
	out  string
	err  error
	last []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.last = append([]string{name}, args...)
	return f.out, f.err
}

func TestReadPowerProfile(t *testing.T) {
	t.Run("reads the AC profile", func(t *testing.T) {
		r := &fakeRunner{out: pmsetOutput}
		p, err := ReadPowerProfile(context.Background(), r)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if p[SettingSleep] != 1 {
			t.Errorf("sleep = %d, want 1", p[SettingSleep])
		}
		if got := strings.Join(r.last, " "); got != "/usr/bin/pmset -g custom" {
			t.Errorf("ran %q", got)
		}
	})
	t.Run("propagates a failure to read", func(t *testing.T) {
		_, err := ReadPowerProfile(context.Background(), &fakeRunner{err: errors.New("boom")})
		if err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("rejects output with no AC block", func(t *testing.T) {
		_, err := ReadPowerProfile(context.Background(), &fakeRunner{out: "Battery Power:\n sleep 5\n"})
		if err == nil {
			t.Fatal("want an error when the profile this policy governs is absent")
		}
	})
}
