package resources

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/husobee/vestigo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublish(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{nil}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestBodyNotJSON(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{nil}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{\`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Failed to process request json. Please provide a valid json request body", resp["message"])
}

func TestRequestHasNoUUID(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{nil}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content//annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Please specify a valid uuid in the request", resp["message"])
}

func TestPublishFailed(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{errors.New("eek")}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "eek", resp["message"])
}

func marshal(body *bytes.Buffer) (map[string]interface{}, error) {
	j := make(map[string]interface{})
	dec := json.NewDecoder(body)
	err := dec.Decode(&j)
	return j, err
}

type mockPublisher struct {
	publishErr error
}

func (m *mockPublisher) GTG() error {
	return nil
}

func (m *mockPublisher) Endpoint() string {
	return ""
}

func (m *mockPublisher) Publish(uuid string, tid string, body map[string]interface{}) error {
	return m.publishErr
}
