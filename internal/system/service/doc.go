// Package service keeps a long-running MCP instance available on a macOS host.
//
// A server that an operator started from a shell is available only as long as
// that shell, that login session, and that host's willingness to stay awake
// last. Availability here means three separate things must hold, so the package
// has one file per concern:
//
//   - power.go     the host must not sleep — pmset's live policy is read and
//     adjudicated against what a continuously-serving host needs
//   - plist.go     the server must be owned by launchd, not by a shell — a
//     typed spec renders the user agent that runs it (under
//     caffeinate, so the process itself asserts no-sleep) plus the
//     watchdog agent that probes it
//   - launchctl.go the agents are loaded, unloaded and kicked through
//     launchctl, behind an injected command runner
//   - recover.go   an unhealthy instance is restarted and re-probed — the
//     routine the watchdog runs on a timer and a remote operator
//     triggers over SSH
//   - address.go   which of the host's addresses a client on the same network
//     would use, and the URL that makes
//   - link.go      the address clients hold is kept true across a move to
//     another network. Losing the link is not a move: wireless
//     returns on the same address far more often than not, so an
//     outage never restarts anything
//
// The package is macOS-specific in what it emits (launchd, pmset), but every
// OS interaction goes through the Runner seam and every decision is a pure
// function over parsed text, so the logic is testable on any platform.
package service
