package intent

import (
	"fmt"
	"os"
	"strconv"
)

// EnvUnknownThreshold names the variable that overrides
// DefaultUnknownThreshold. DefaultUnknownThreshold was calibrated on bge-m3,
// so evaluation runs on other embedding models set it per model.
const EnvUnknownThreshold = "CKS_INTENT_UNKNOWN_THRESHOLD"

// OptionsFromEnv returns the classifier options set by the environment.
// An unset variable returns no options; a value outside [0, 1] is an error.
func OptionsFromEnv() ([]Option, error) {
	v := os.Getenv(EnvUnknownThreshold)
	if v == "" {
		return nil, nil
	}
	t, err := strconv.ParseFloat(v, 64)
	if err != nil || t < 0 || t > 1 {
		return nil, fmt.Errorf("intent: invalid %s %q", EnvUnknownThreshold, v)
	}
	return []Option{WithUnknownThreshold(t)}, nil
}
