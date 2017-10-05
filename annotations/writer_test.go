package annotations

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/husobee/vestigo"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWrite(t *testing.T) {
	uuid := uuid.New()
	server := startMockWriteServer(t, uuid, true, true, false)
	defer server.Close()

	writer := NewWriter(http.DefaultClient, server.URL+"/drafts/content/%v/annotations", server.URL+"/__gtg")

	body := make(map[string]interface{})
	resp, err := writer.Write(uuid, "tid_"+uuid, body)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, resp["worked"], "yep", "Response should match what the mock server writes back")
}

func TestJSONMarshalFails(t *testing.T) {
	writer := NewWriter(http.DefaultClient, "/drafts/content/%v/annotations", "/__gtg")

	body := make(map[string]interface{})
	body["test"] = func() {}

	_, err := writer.Write("uuid", "tid", body)
	assert.EqualError(t, err, "json: unsupported type: func()")
}

func TestWriteRequestFails(t *testing.T) {
	writer := NewWriter(http.DefaultClient, ":#%v", "/__gtg")

	body := make(map[string]interface{})

	_, err := writer.Write("", "tid", body)
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func TestWriteRequestURLInvalid(t *testing.T) {
	writer := NewWriter(http.DefaultClient, "#:%v", "/__gtg")

	body := make(map[string]interface{})

	_, err := writer.Write("", "tid", body)
	assert.EqualError(t, err, "Put #:: unsupported protocol scheme \"\"")
}

func TestWriteResponseFails(t *testing.T) {
	uuid := uuid.New()
	server := startMockWriteServer(t, uuid, false, true, false)
	defer server.Close()

	writer := NewWriter(http.DefaultClient, server.URL+"/drafts/content/%v/annotations", server.URL+"/__gtg")

	body := make(map[string]interface{})
	_, err := writer.Write(uuid, "tid_"+uuid, body)
	assert.EqualError(t, err, fmt.Sprintf("Save to %v/drafts/content/%v/annotations returned a 503 status code", server.URL, uuid))
}

func TestWriteResponseInvalidJSON(t *testing.T) {
	uuid := uuid.New()
	server := startMockWriteServer(t, uuid, true, true, true)
	defer server.Close()

	writer := NewWriter(http.DefaultClient, server.URL+"/drafts/content/%v/annotations", server.URL+"/__gtg")

	body := make(map[string]interface{})
	_, err := writer.Write(uuid, "tid_"+uuid, body)
	assert.EqualError(t, err, "invalid character 'd' looking for beginning of object key string")
}

func TestWriteGTG(t *testing.T) {
	server := startMockWriteServer(t, "uuid", true, true, false)
	defer server.Close()

	writer := NewWriter(http.DefaultClient, server.URL+"/drafts/content/%v/annotations", server.URL+"/__gtg")

	err := writer.GTG()
	assert.NoError(t, err)
}

func TestWriteGTGNon200Status(t *testing.T) {
	server := startMockWriteServer(t, "uuid", true, false, false)
	defer server.Close()

	writer := NewWriter(http.DefaultClient, server.URL+"/drafts/content/%v/annotations", server.URL+"/__gtg")

	err := writer.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v/__gtg returned a 503 status code", server.URL))
}

func TestWriteGTGRequestFails(t *testing.T) {
	writer := NewWriter(http.DefaultClient, "/drafts/content/%v/annotations", "#:")

	err := writer.GTG()
	assert.EqualError(t, err, "Get #:: unsupported protocol scheme \"\"")
}

func TestWriteGTGInvalidURL(t *testing.T) {
	writer := NewWriter(http.DefaultClient, "/drafts/content/%v/annotations", ":#")

	err := writer.GTG()
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func TestWriterEndpoint(t *testing.T) {
	writer := NewWriter(http.DefaultClient, "/drafts/content/%v/annotations", ":#")

	actual := writer.Endpoint()
	assert.Equal(t, "/drafts/content/%v/annotations", actual)
}

func startMockWriteServer(t *testing.T, uuid string, saveOk bool, gtgOk bool, incorrectResponseJSON bool) *httptest.Server {
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

	r.Put("/drafts/content/:uuid/annotations", func(w http.ResponseWriter, r *http.Request) {
		reqUUID := vestigo.Param(r, "uuid")
		assert.Equal(t, uuid, reqUUID, "Request path uuid should match the expected uuid")

		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC annotations-publisher", userAgent)

		contentType := r.Header.Get("Content-Type")
		assert.Equal(t, "application/json", contentType)

		tid := r.Header.Get("X-Request-Id")
		assert.Equal(t, "tid_"+uuid, tid)

		dec := json.NewDecoder(r.Body)
		data := make(map[string]interface{})
		err := dec.Decode(&data)
		assert.NoError(t, err)

		if !saveOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		if incorrectResponseJSON {
			w.Write([]byte(`{didn't work}`))
		} else {
			w.Write([]byte(`{"worked":"yep"}`))
		}
	})
	return httptest.NewServer(r)
}
