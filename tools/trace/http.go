package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type routeKey struct{}

// Route names the request HTTP is recording by the pattern a router matched,
// for a router that keeps the pattern off the request HTTP holds.
func Route(ctx context.Context, pattern string) {
	if route, ok := ctx.Value(routeKey{}).(*string); ok {
		*route = pattern
	}
}

// HTTP records every request the suite sends to the service's own server, under
// the route it matched rather than the path, which carries ids.
func HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !on.Load() {
			next.ServeHTTP(w, r)
			return
		}
		began := seq.Add(1)
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		rec := &response{ResponseWriter: w, status: http.StatusOK}
		route := new(string)
		r = r.WithContext(context.WithValue(r.Context(), routeKey{}, route))
		next.ServeHTTP(rec, r)

		name := r.Pattern
		if *route != "" {
			name = r.Method + " " + *route
		}
		if name == "" {
			name = r.Method + " " + r.URL.Path
		}
		if len(body) == 0 && r.URL.RawQuery != "" {
			body, _ = json.Marshal(r.URL.Query())
		}
		out := Record{Seq: began, Dir: "in", Kind: "http", Name: name, Request: raw(body), Response: raw(rec.body.Bytes())}
		if rec.status >= http.StatusBadRequest {
			out.Error = fmt.Sprintf("HTTP %d", rec.status)
			var answer struct{ Type string }
			if json.Unmarshal(rec.body.Bytes(), &answer) == nil && answer.Type != "" {
				out.Error += " " + answer.Type
			}
		}
		write(out)
	})
}

type response struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *response) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *response) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}
