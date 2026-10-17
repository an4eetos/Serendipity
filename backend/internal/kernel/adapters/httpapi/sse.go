package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Stream writes server-sent events.
type Stream struct {
	w http.ResponseWriter
	f http.Flusher
}

// NewStream sends the event-stream headers. After it succeeds the response
// status is committed, so failures must be reported as events.
func NewStream(w http.ResponseWriter) (*Stream, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming unsupported")
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	f.Flush()
	return &Stream{w: w, f: f}, nil
}

// Send writes v as one JSON data event.
func (s *Stream) Send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", b); err != nil {
		return err
	}
	s.f.Flush()
	return nil
}
