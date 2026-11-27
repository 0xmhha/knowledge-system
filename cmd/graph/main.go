package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// exitCodeError is an error a command ends with under an exit code of its
// own, which its help states.
type exitCodeError struct {
	Code int
	Err  error
}

func (e exitCodeError) Error() string { return e.Err.Error() }

func (e exitCodeError) Unwrap() error { return e.Err }

func main() {
	if err := newRootCmd().Execute(); err != nil {
		var exportErr graphExportError
		if errors.As(err, &exportErr) {
			_ = json.NewEncoder(os.Stderr).Encode(struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Cause   string `json:"cause,omitempty"`
			}{string(exportErr), graphExportMessage(exportErr), graphExportCause(err, exportErr)})
			os.Exit(1)
		}
		// audit subcommand surfaces structured exit codes (1=diff, 2=error)
		// via auditExitCode. Its RunE prints any human-readable diagnostic
		// to stderr before returning, so we don't repeat it here.
		var ae auditExitCode
		if errors.As(err, &ae) {
			os.Exit(int(ae))
		}
		var ve validateExitCode
		if errors.As(err, &ve) {
			os.Exit(int(ve))
		}
		var ce exitCodeError
		if errors.As(err, &ce) {
			_, _ = fmt.Fprintln(os.Stderr, "ckg:", ce.Err)
			os.Exit(ce.Code)
		}
		_, _ = fmt.Fprintln(os.Stderr, "ckg:", err)
		os.Exit(1)
	}
}
