package annotations

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/husobee/vestigo"
	"github.com/stretchr/testify/assert"
	"github.com/twinj/uuid"
)

func TestPublish(t *testing.T) {
	uuid := uuid.NewV4()
	server := startMockServer(t, uuid.String(), true, true)
	publisher := NewPublisher("originSystemID", server.URL+"/notify", server.URL+"/__gtg")

	err := publisher.Publish(uuid.String(), make(map[string]interface{}))
	assert.NoError(t, err)
}

func TestPublishFailsToMarshalBodyToJSON(t *testing.T) {
	uuid := uuid.NewV4()
	server := startMockServer(t, uuid.String(), true, true)
	publisher := NewPublisher("originSystemID", server.URL+"/notify", server.URL+"/__gtg")

	body := make(map[string]interface{})
	body["dodgy!"] = func() {}
	err := publisher.Publish(uuid.String(), body)
	assert.EqualError(t, err, "json: unsupported type: func()")
}

func TestPublishFailsInvalidURL(t *testing.T) {
	publisher := NewPublisher("originSystemID", ":#", "/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish("a-valid-uuid", body)
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func TestPublishRequestFailsServerUnavailable(t *testing.T) {
	publisher := NewPublisher("originSystemID", "/publish", "/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish("a-valid-uuid", body)
	assert.EqualError(t, err, "Post /publish: unsupported protocol scheme \"\"")
}

func TestPublishRequestUnsuccessful(t *testing.T) {
	uuid := uuid.NewV4()
	server := startMockServer(t, uuid.String(), false, true)

	publisher := NewPublisher("originSystemID", server.URL+"/notify", server.URL+"/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish(uuid.String(), body)
	assert.EqualError(t, err, fmt.Sprintf("Publish to %v/notify returned a 503 status code", server.URL))
}

func TestPublisherEndpoint(t *testing.T) {
	publisher := NewPublisher("originSystemID", "publishEndpoint", "/__gtg")
	assert.Equal(t, "publishEndpoint", publisher.Endpoint())
}

func TestPublisherGTG(t *testing.T) {
	server := startMockServer(t, "", true, true)

	publisher := NewPublisher("originSystemID", "publishEndpoint", server.URL+"/__gtg")
	err := publisher.GTG()
	assert.NoError(t, err)
}

func TestPublisherGTGFails(t *testing.T) {
	server := startMockServer(t, "", true, false)

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

func startMockServer(t *testing.T, uuid string, publishOk bool, gtgOk bool) *httptest.Server {
	r := vestigo.NewRouter()
	r.Get("/__gtg", func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC annotations-publisher", userAgent)

		if !gtgOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Post("/notify", func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC annotations-publisher", userAgent)

		contentType := r.Header.Get("Content-Type")
		assert.Equal(t, "application/json", contentType)

		originSystemID := r.Header.Get("X-Origin-System-Id")
		assert.Equal(t, "originSystemID", originSystemID)

		dec := json.NewDecoder(r.Body)
		data := make(map[string]interface{})
		err := dec.Decode(&data)
		assert.NoError(t, err)

		bodyUUID, ok := data["uuid"]
		assert.True(t, ok)
		assert.Equal(t, uuid, bodyUUID.(string))

		if !publishOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	return httptest.NewServer(r)
}
