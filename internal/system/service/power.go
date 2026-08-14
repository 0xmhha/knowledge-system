package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// pmsetPath is the absolute path to the power management CLI, for the same
// reason launchctlPath is absolute: a launchd job inherits no PATH.
const pmsetPath = "/usr/bin/pmset"

// acProfile is the pmset settings block that applies while the host is on wall
// power. A machine serving MCP continuously is on wall power by definition, so
// this is the profile the policy adjudicates; `pmset -a` remediation writes
// every profile anyway.
const acProfile = "AC Power"

// Setting is one pmset key this deployment has an opinion about.
type Setting string

const (
	// SettingSleep is idle system sleep in minutes; 0 means never.
	SettingSleep Setting = "sleep"
	// SettingStandby is the deeper hibernate-to-disk state entered some time
	// after sleeping.
	SettingStandby Setting = "standby"
	// SettingWakeOnLAN is whether a magic packet wakes the host.
	SettingWakeOnLAN Setting = "womp"
	// SettingAutoRestart is whether the host powers back on after a power cut.
	SettingAutoRestart Setting = "autorestart"
)

// Requirement is one setting's required value and the availability property it
// buys. The Why text is printed to the operator, so it explains the cost of
// leaving the setting as it is.
type Requirement struct {
	Setting Setting
	Want    int
	Why     string
}

// RequiredPolicy returns, in report order, the pmset settings a host serving
// this MCP server continuously must have. It is a function rather than a
// package variable so no caller can mutate the policy.
func RequiredPolicy() []Requirement {
	return []Requirement{
		{SettingSleep, 0, "idle system sleep takes the listener down; a client sees a dead port, not a slow one"},
		{SettingStandby, 0, "standby hibernates to disk, so waking is slow enough to time a client out"},
		{SettingWakeOnLAN, 1, "a magic packet is the only way to wake this host remotely if it does sleep"},
		{SettingAutoRestart, 1, "after a power cut the host must come back without someone pressing the button"},
	}
}

// Violation is one requirement the live profile does not meet, carrying the
// value the host actually reports.
type Violation struct {
	Requirement
	Got int
}

// Profile is one pmset settings block: key to integer value.
type Profile map[Setting]int

// ParsePMSet parses `pmset -g custom` output into one Profile per power source
// block ("AC Power", "Battery Power"). Lines it does not understand — the
// textual settings like "Sleep On Power Button" — are skipped rather than
// guessed at.
func ParsePMSet(out string) map[string]Profile {
	profiles := make(map[string]Profile)
	current := ""
	for _, line := range strings.Split(out, "\n") {
		if header, ok := strings.CutSuffix(strings.TrimSpace(line), ":"); ok {
			current = header
			profiles[current] = Profile{}
			continue
		}
		if current == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue // multi-word setting names carry no value we adjudicate
		}
		value, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		profiles[current][Setting(fields[0])] = value
	}
	return profiles
}

// Violations returns the requirements p does not meet, in policy order. A
// setting absent from p is not reported: the host does not support it.
func Violations(p Profile) []Violation {
	var out []Violation
	for _, req := range RequiredPolicy() {
		got, present := p[req.Setting]
		if !present || got == req.Want {
			continue
		}
		out = append(out, Violation{Requirement: req, Got: got})
	}
	return out
}

// RemediationCommand is the single privileged command that fixes every
// violation, or "" when there is nothing to fix. It is printed for the operator
// to run rather than executed: changing a host's power policy needs root, and a
// server binary should not be asking for it.
func RemediationCommand(vs []Violation) string {
	if len(vs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(vs)*2+3)
	parts = append(parts, "sudo", pmsetPath, "-a")
	for _, v := range vs {
		parts = append(parts, string(v.Setting), strconv.Itoa(v.Want))
	}
	return strings.Join(parts, " ")
}

// ReadPowerProfile runs `pmset -g custom` through the runner and returns the AC
// profile — the one that governs a continuously-powered host.
func ReadPowerProfile(ctx context.Context, r Runner) (Profile, error) {
	out, err := r.Run(ctx, pmsetPath, "-g", "custom")
	if err != nil {
		return nil, fmt.Errorf("read power settings: %w", err)
	}
	profiles := ParsePMSet(out)
	p, ok := profiles[acProfile]
	if !ok {
		return nil, fmt.Errorf("read power settings: no %q block in pmset output", acProfile)
	}
	return p, nil
}
