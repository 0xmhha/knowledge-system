package persist

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// gzipMagic opens every gzip stream (RFC 1952: ID1, ID2, CM=deflate).
var gzipMagic = []byte{0x1f, 0x8b, 0x08}

// IsGzipBlob reports whether a blobs.source value is gzip-compressed. The
// build stores Hunk patches that way (schema 1.8); source text of other
// nodes is stored as is.
func IsGzipBlob(b []byte) bool { return bytes.HasPrefix(b, gzipMagic) }

// BlobText returns the text a blobs.source value holds: the decompressed
// patch of a gzip blob, any other blob unchanged.
func BlobText(b []byte) ([]byte, error) {
	if !IsGzipBlob(b) {
		return b, nil
	}
	gr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("blob text: %w", err)
	}
	defer func() { _ = gr.Close() }()
	out, err := io.ReadAll(gr)
	if err != nil {
		return nil, fmt.Errorf("blob text: %w", err)
	}
	return out, nil
}
