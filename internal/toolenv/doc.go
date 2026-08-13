// Package toolenv discovers, at run time, where the external tools a build
// subprocess needs actually live on this machine.
//
// The graph build shells out to `go list` through go/packages, and the
// file-list derivation does the same; both therefore need `go` and `git` on
// PATH. A server started from a terminal gets them for free, because the
// operator's shell assembled that PATH from /etc/paths.d and their dotfiles.
// A server started by launchd does not: launchd is not a login shell, so its
// jobs run with a fixed minimal PATH that contains neither Homebrew nor any
// version manager's directory. The dependency was always there; only the
// terminal was hiding it.
//
// The fix could be to write the absolute paths into the launchd job at
// install time, but a recorded absolute path is a snapshot: it embeds one
// machine's home directory, and it dies the moment a version manager switches
// releases (~/.gvm/gos/go1.25.11/bin is not a stable address). So this package
// resolves instead of records — the committed code carries strategies, never
// machine paths, and every deployment answers the question for itself.
//
// Three strategies, in order, first hit wins:
//
//  1. the inherited PATH — free, and correct whenever a shell started us;
//  2. the operator's login shell — `$SHELL -lc "command -v go"`, which is how
//     a machine using gvm/asdf/mise answers honestly, since those tools exist
//     only as shell configuration;
//  3. a short list of standard install locations, for hosts whose login shell
//     configures nothing.
//
// Strategy 2 asks the login shell one bounded question and validates the
// answer before trusting it. That is deliberately narrower than running the
// server itself under a login shell: the dotfiles influence one lookup whose
// result must name an executable file, not the whole process environment.
package toolenv
