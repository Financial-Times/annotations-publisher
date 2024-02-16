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
	"time"

	"github.com/Financial-Times/annotations-publisher/annotations"
	"github.com/husobee/vestigo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testPublishBody = `
{
	"annotations":[
		{
			"predicate": "http://www.ft.com/ontology/annotation/mentions",
			"id": "http://www.ft.com/thing/0a619d71-9af5-3755-90dd-f789b686c67a"
		},
		{
			"predicate": "http://www.ft.com/ontology/annotation/hasAuthor",
			"id": "http://www.ft.com/thing/838b3fbe-efbc-3cfe-b5c0-d38c046492a4"
		}
	]
}`

type failingReader struct {
	err error
}

var timeout = 8 * time.Second

func TestPublish(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("SaveAndPublish", mock.Anything, "a-valid-uuid", "hash", mock.Anything).Return(nil)

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(testPublishBody))
	req.Header.Add(annotations.PreviousDocumentHashHeader, "hash")
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	pub.AssertExpectations(t)
}

func TestBodyNotJSON(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{\`))
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Failed to process request json. Please provide a valid json request body", resp["message"])

	pub.AssertExpectations(t)
}

func TestPublishNotFound(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("SaveAndPublish", mock.Anything, "a-valid-uuid", "hash", mock.Anything).Return(annotations.ErrDraftNotFound)

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(testPublishBody))
	req.Header.Add(annotations.PreviousDocumentHashHeader, "hash")
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, annotations.ErrDraftNotFound.Error(), strings.ToLower(resp["message"].(string)))

	pub.AssertExpectations(t)
}

func TestPublishTimedout(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("SaveAndPublish", mock.Anything, "a-valid-uuid", "hash", mock.Anything).Return(annotations.ErrServiceTimeout)

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(testPublishBody))
	req.Header.Add(annotations.PreviousDocumentHashHeader, "hash")
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	assert.Equal(t, annotations.ErrServiceTimeout.Error(), strings.ToLower(resp["message"].(string)))

	pub.AssertExpectations(t)
}

func TestPublishMissingBody(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", nil)
	req.Header.Add(annotations.PreviousDocumentHashHeader, "hash")
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Please provide a valid json request body", resp["message"])

	pub.AssertExpectations(t)
}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, f.err
}

func TestPublishBodyReadFail(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", &failingReader{err: errors.New("failed to read request body. Please provide a valid json request body")})

	req.Header.Add(annotations.PreviousDocumentHashHeader, "hash")
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)
	resp, err := marshal(w.Body)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Failed to read request body. Please provide a valid json request body", resp["message"])

	pub.AssertExpectations(t)
}

func TestPublishNoHashHeader(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("SaveAndPublish", mock.Anything, "a-valid-uuid", "", mock.Anything).Return(nil)

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(testPublishBody))
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	pub.AssertExpectations(t)
}

func TestRequestHasNoUUID(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content//annotations/publish", strings.NewReader(`{}`))
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

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
	pub.On("SaveAndPublish", mock.Anything, "a-valid-uuid", "hash", mock.Anything).Return(errors.New("eek"))

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(testPublishBody))
	req.Header.Add(annotations.PreviousDocumentHashHeader, "hash")
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "eek", strings.ToLower(resp["message"].(string)))

	pub.AssertExpectations(t)
}

func TestPublishFromStore(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("PublishFromStore", mock.Anything, "a-valid-uuid").Return(nil)
	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish?fromStore=true", nil)
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "Publish accepted", resp["message"])

	pub.AssertExpectations(t)
}

func TestPublishFromStoreNotFound(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("PublishFromStore", mock.Anything, "a-valid-uuid").Return(annotations.ErrDraftNotFound)
	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish?fromStore=true", nil)
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, annotations.ErrDraftNotFound.Error(), strings.ToLower(resp["message"].(string)))

	pub.AssertExpectations(t)
}

func TestPublishFromStoreTimeout(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("PublishFromStore", mock.Anything, "a-valid-uuid").Return(annotations.ErrServiceTimeout)
	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish?fromStore=true", nil)
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	assert.Equal(t, annotations.ErrServiceTimeout.Error(), strings.ToLower(resp["message"].(string)))

	pub.AssertExpectations(t)
}

func TestPublishFromStoreTrueWithBody(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}

	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish?fromStore=true", strings.NewReader(testPublishBody))
	req.Header.Add(annotations.PreviousDocumentHashHeader, "hash")
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "A request body cannot be provided when fromStore=true", resp["message"])

	pub.AssertExpectations(t)
}

func TestPublishFromStoreFails(t *testing.T) {
	r := vestigo.NewRouter()
	pub := &mockPublisher{}
	pub.On("PublishFromStore", mock.Anything, "a-valid-uuid").Return(errors.New("test error"))
	r.Post("/drafts/content/:uuid/annotations/publish", Publish(pub, timeout))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish?fromStore=true", nil)
	req.Header.Add(annotations.OriginSystemIDHeader, "originSystemId")

	r.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Unable to publish annotations from store", resp["message"])

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

func (m *mockPublisher) Publish(ctx context.Context, uuid string, body map[string]interface{}) error {
	args := m.Called(ctx, uuid, body)
	return args.Error(0)
}

func (m *mockPublisher) PublishFromStore(ctx context.Context, uuid string) error {
	args := m.Called(ctx, uuid)
	return args.Error(0)
}

func (m *mockPublisher) SaveAndPublish(ctx context.Context, uuid string, hash string, body map[string]interface{}) error {
	args := m.Called(ctx, uuid, hash, body)
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
