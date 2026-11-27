package impl

type testDecoder struct{}

// Decode implements Decoder.
func (testDecoder) Decode(*Stream) error { return nil }

var _ Decoder = testDecoder{}
