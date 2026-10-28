package intent

import "testing"

func TestOptionsFromEnv(t *testing.T) {
	t.Setenv(EnvUnknownThreshold, "")
	if opts, err := OptionsFromEnv(); err != nil || len(opts) != 0 {
		t.Fatalf("unset: opts=%d err=%v", len(opts), err)
	}
	t.Setenv(EnvUnknownThreshold, "0.42")
	opts, err := OptionsFromEnv()
	if err != nil || len(opts) != 1 {
		t.Fatalf("0.42: opts=%d err=%v", len(opts), err)
	}
	c := &Classifier{unknownThreshold: DefaultUnknownThreshold}
	opts[0](c)
	if c.unknownThreshold != 0.42 {
		t.Fatalf("threshold = %v, want 0.42", c.unknownThreshold)
	}
	for _, v := range []string{"x", "-0.1", "1.5"} {
		t.Setenv(EnvUnknownThreshold, v)
		if _, err := OptionsFromEnv(); err == nil {
			t.Fatalf("%q accepted", v)
		}
	}
}
