package explain

import (
	"net/http"
	"net/http/httptest"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type inMemoryRoundTripper struct {
	handler http.Handler
}

func newInMemoryRoundTripper(handler http.Handler) http.RoundTripper {
	return inMemoryRoundTripper{handler: handler}
}

func (t inMemoryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}
