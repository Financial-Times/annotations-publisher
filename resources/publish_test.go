package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Financial-Times/annotations-publisher/annotations"
	"github.com/husobee/vestigo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublish(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{nil, nil}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestBodyNotJSON(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{nil, nil}

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
	pub := &mockPublisher{nil, nil}

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
	pub := &mockPublisher{errors.New("eek"), nil}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "eek", strings.ToLower(resp["message"].(string)))
}

func TestPublishAuthenticationInvalid(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{annotations.ErrInvalidAuthentication, nil}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Publish authentication is invalid", resp["message"])
}

func TestPublishFromStoreNotFound(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{nil, annotations.ErrDraftNotFound}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish?fromStore=true", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, annotations.ErrDraftNotFound.Error(), strings.ToLower(resp["message"].(string)))
}

func TestPublishFromStoreFails(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{nil, errors.New("test error")}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish?fromStore=true", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Unable to read draft annotations", resp["message"])
}

func marshal(body *bytes.Buffer) (map[string]interface{}, error) {
	j := make(map[string]interface{})
	dec := json.NewDecoder(body)
	err := dec.Decode(&j)
	return j, err
}

type mockPublisher struct {
	publishErr error
	draftAnnotationErr error
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
func (m *mockPublisher) GetDraft(ctx context.Context, uuid string) (interface{}, error) {
	return nil, m.draftAnnotationErr
}

func (m *mockPublisher) SaveDraft(ctx context.Context, uuid string, data interface{}) (interface{}, error) {
	return nil, nil
}
