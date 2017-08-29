package annotations

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/husobee/vestigo"
	"github.com/stretchr/testify/assert"
)

func TestPublisherEndpoint(t *testing.T) {
	publisher := NewPublisher("originSystemID", "publishEndpoint", "/__gtg")
	assert.Equal(t, "publishEndpoint", publisher.Endpoint())
}

func TestPublisherGTG(t *testing.T) {
	server := startMockGTGServer(t, true)

	publisher := NewPublisher("originSystemID", "publishEndpoint", server.URL+"/__gtg")
	err := publisher.GTG()
	assert.NoError(t, err)
}

func TestPublisherGTGFails(t *testing.T) {
	server := startMockGTGServer(t, false)

	publisher := NewPublisher("originSystemID", "publishEndpoint", server.URL+"/__gtg")
	err := publisher.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v returned a %v status code", server.URL+"/__gtg", http.StatusServiceUnavailable))
}

func TestPublisherGTGDoRequestFails(t *testing.T) {
	publisher := NewPublisher("originSystemID", "publishEndpoint", "/__gtg")
	err := publisher.GTG()
	assert.EqualError(t, err, "Get /__gtg: unsupported protocol scheme \"\"")
}

func TestPublisherGTGInvalidURL(t *testing.T) {
	publisher := NewPublisher("originSystemID", "publishEndpoint", ":#")
	err := publisher.GTG()
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func startMockGTGServer(t *testing.T, ok bool) *httptest.Server {
	r := vestigo.NewRouter()
	r.Get("/__gtg", func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC annotations-publisher", userAgent)

		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	return httptest.NewServer(r)
}
