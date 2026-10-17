package httpapi

import (
	"encoding/json"
	"net/http"
)

const maxJSONBody = 1 << 20

// DecodeJSON reads a JSON body of at most 1 MiB into v, rejecting unknown
// fields.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return BadRequest("invalid JSON body: " + err.Error())
	}
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
