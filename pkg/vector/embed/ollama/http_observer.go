package ollama

import (
	"context"
	"io"
	"net/http"
	"time"
)

// HTTPObserver receives method/path and body byte counts, never URL hosts,
// headers, request/response contents or embeddings. A nil completion opts out.
// Elapsed time ends at the last body read (or header arrival with no reads),
// so later model validation requests are not charged to the earlier HTTP call.
type HTTPObserver func(context.Context, string, string, int64) func(int, int64, int64, error)

type observedTransport struct {
	base    http.RoundTripper
	observe HTTPObserver
}

// Observe each transport attempt, including redirects followed by Client.Do.
func (t *observedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	finish := t.observe(req.Context(), req.Method, req.URL.Path, req.ContentLength)
	if finish == nil {
		return t.base.RoundTrip(req)
	}
	started := time.Now()
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		finish(0, 0, time.Since(started).Nanoseconds(), err)
		return resp, err
	}
	if resp.Body == nil {
		finish(resp.StatusCode, 0, time.Since(started).Nanoseconds(), nil)
		return resp, nil
	}
	resp.Body = &observedBody{ReadCloser: resp.Body, status: resp.StatusCode, started: started, lastRead: time.Now(), finish: finish}
	return resp, nil
}

type observedBody struct {
	io.ReadCloser
	status            int
	bytes             int64
	started, lastRead time.Time
	readErr           error
	finished          bool
	finish            func(int, int64, int64, error)
}

func (b *observedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.bytes += int64(n)
	b.lastRead = time.Now()
	if err != nil && err != io.EOF {
		b.readErr = err
	}
	return n, err
}

func (b *observedBody) Close() error {
	err := b.ReadCloser.Close()
	if !b.finished {
		b.finished = true
		observedErr := b.readErr
		if observedErr == nil {
			observedErr = err
		}
		b.finish(b.status, b.bytes, b.lastRead.Sub(b.started).Nanoseconds(), observedErr)
	}
	return err
}
