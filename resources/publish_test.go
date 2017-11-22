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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPublish(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("Publish", "a-valid-uuid", mock.AnythingOfType("string"), mock.Anything).Return(nil)

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	pub.AssertExpectations(t)
}

func TestBodyNotJSON(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{\`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Failed to process request json. Please provide a valid json request body", resp["message"])

	pub.AssertExpectations(t)
}

func TestRequestHasNoUUID(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content//annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Please specify a valid uuid in the request", resp["message"])

	pub.AssertExpectations(t)
}

func TestPublishFailed(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("Publish", "a-valid-uuid", mock.AnythingOfType("string"), mock.Anything).Return(errors.New("eek"))

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "eek", strings.ToLower(resp["message"].(string)))

	pub.AssertExpectations(t)
}

func TestPublishAuthenticationInvalid(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("Publish", "a-valid-uuid", mock.AnythingOfType("string"), mock.Anything).Return(annotations.ErrInvalidAuthentication)

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Publish authentication is invalid", resp["message"])

	pub.AssertExpectations(t)
}

func TestPublishFromStoreNotFound(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	// unfortunately, mock.AnythingOfType doesn't seem to work with interfaces
	pub.On("PublishFromStore", mock.Anything, "a-valid-uuid").Return(annotations.ErrDraftNotFound)
	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish?fromStore=true", nil)

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, annotations.ErrDraftNotFound.Error(), strings.ToLower(resp["message"].(string)))

	pub.AssertExpectations(t)
}

func TestPublishFromStoreFails(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	// unfortunately, mock.AnythingOfType doesn't seem to work with interfaces
	pub.On("PublishFromStore", mock.Anything, "a-valid-uuid").Return(errors.New("test error"))
	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish?fromStore=true", nil)

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Unable to read draft annotations", resp["message"])

	pub.AssertExpectations(t)
}

func marshal(body *bytes.Buffer) (map[string]interface{}, error) {
	j := make(map[string]interface{})
	dec := json.NewDecoder(body)
	err := dec.Decode(&j)
	return j, err
}

type mockPublisher struct {
	mock.Mock
}

func (m *mockPublisher) GTG() error {
	return nil
}

func (m *mockPublisher) Endpoint() string {
	return ""
}

func (m *mockPublisher) Publish(uuid string, tid string, body map[string]interface{}) error {
	args := m.Called(uuid, tid, body)
	return args.Error(0)
}

func (m *mockPublisher) PublishFromStore(ctx context.Context, uuid string) error {
	args := m.Called(ctx, uuid)
	return args.Error(0)
}

func (m *mockPublisher) GetDraft(ctx context.Context, uuid string) (interface{}, error) {
	args := m.Called(ctx, uuid)
	return args.Get(0), args.Error(1)
}

func (m *mockPublisher) SaveDraft(ctx context.Context, uuid string, data interface{}) (interface{}, error) {
	args := m.Called(ctx, uuid)
	return args.Get(0), args.Error(1)
}
