//go:build !darwin && !linux

package setup

import (
	"fmt"
	"os"
)

// Mutable captures are supported on macOS and Linux only. Refuse their
// no-follow read path on other platforms rather than silently weakening it.
func openCapturedNoFollow(_, _ string) (*os.File, error) {
	return nil, fmt.Errorf("no-follow source capture requires macOS or Linux")
}
