package annotations

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/husobee/vestigo"
	"github.com/stretchr/testify/assert"
)

func TestAnnotationsRWGTG(t *testing.T) {
	server := mockGtgServer(t, true)
	defer server.Close()

	client, err := NewPublishedAnnotationsWriter(server.URL + "/%s")
	assert.NoError(t, err)

	err = client.GTG()
	assert.NoError(t, err)
}

func TestAnnotationsRWGTGFails(t *testing.T) {
	server := mockGtgServer(t, false)
	defer server.Close()

	client, err := NewPublishedAnnotationsWriter(server.URL + "/%s")
	err = client.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v returned a %v status code", server.URL+"/__gtg", http.StatusServiceUnavailable))
}

func TestAnnotationsRWGTGInvalidURL(t *testing.T) {
	client, err := NewPublishedAnnotationsWriter(":#")
	assert.Nil(t, client, "New PublishedAnnotationsWriter should not have returned a client")
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func mockGtgServer(t *testing.T, gtgOk bool) *httptest.Server {
	r := vestigo.NewRouter()
	r.Get(status.GTGPath, func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC annotations-publisher", userAgent)

		if !gtgOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	return httptest.NewServer(r)
}
