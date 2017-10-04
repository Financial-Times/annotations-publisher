package resources

import (
	"bytes"
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
	"github.com/stretchr/testify/suite"
)

type publishResourceTestSuite struct {
	suite.Suite
	router            *vestigo.Router
	mockPublisher     *mockPublisher
	mockWriter        *mockWriter
	expectedWriteBody map[string]interface{}
}

func (p *publishResourceTestSuite) SetupTest() {
	p.mockPublisher = new(mockPublisher)
	p.router = vestigo.NewRouter()
	p.expectedWriteBody = make(map[string]interface{})
	p.expectedWriteBody["inserted"] = "something"
	p.mockWriter = &mockWriter{nil}

	p.router.Post("/drafts/content/:uuid/annotations/publish", Publish(p.mockWriter, p.mockPublisher))
}

func (p *publishResourceTestSuite) TearDownTest() {
	p.mockPublisher.AssertExpectations(p.T())
}

func TestPublishResourceSuite(t *testing.T) {
	suite.Run(t, &publishResourceTestSuite{})
}

func (p *publishResourceTestSuite) TestPublishSucceeds() {
	p.mockPublisher.On("Publish", "a-valid-uuid", "tid_1234", p.expectedWriteBody).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))
	req.Header.Add("X-Request-Id", "tid_1234")

	p.router.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), http.StatusAccepted, w.Code)
	assert.Equal(p.T(), "Publish accepted", resp["message"])
}

func (p *publishResourceTestSuite) TestWriteFails() {
	p.mockWriter.writeErr = errors.New("eek")

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))

	p.router.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(p.T(), "eek", resp["message"])
}

func (p *publishResourceTestSuite) TestBodyNotJSON() {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{\`))

	p.router.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), http.StatusBadRequest, w.Code)
	assert.Equal(p.T(), "Failed to process request json. Please provide a valid json request body", resp["message"])
}

func (p *publishResourceTestSuite) TestRequestHasNoUUID() {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content//annotations/publish", strings.NewReader(`{}`))

	p.router.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), http.StatusBadRequest, w.Code)
	assert.Equal(p.T(), "Please specify a valid uuid in the request", resp["message"])
}

func (p *publishResourceTestSuite) TestPublishFailed() {
	p.mockPublisher.On("Publish", "a-valid-uuid", "tid_1234", p.expectedWriteBody).Return(errors.New("eek"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))
	req.Header.Add("X-Request-Id", "tid_1234")

	p.router.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), http.StatusServiceUnavailable, w.Code)
	assert.Equal(p.T(), "eek", resp["message"])
}

func (p *publishResourceTestSuite) TestPublishAuthenticationInvalid() {
	p.mockPublisher.On("Publish", "a-valid-uuid", "tid_1234", p.expectedWriteBody).Return(annotations.ErrInvalidAuthentication)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/drafts/content/a-valid-uuid/annotations/publish", strings.NewReader(`{}`))
	req.Header.Add("X-Request-Id", "tid_1234")

	p.router.ServeHTTP(w, req)

	resp, err := marshal(w.Body)
	require.NoError(p.T(), err)
	assert.Equal(p.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(p.T(), annotations.ErrInvalidAuthentication.Error(), resp["message"])
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
	args := m.Called()
	return args.Error(0)
}

func (m *mockPublisher) Endpoint() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockPublisher) Publish(uuid string, tid string, body map[string]interface{}) error {
	args := m.Called(uuid, tid, body)
	return args.Error(0)
}

type mockWriter struct {
	writeErr error
}

func (m *mockWriter) GTG() error {
	return nil
}

func (m *mockWriter) Endpoint() string {
	return ""
}

func (m *mockWriter) Write(uuid string, tid string, body map[string]interface{}) (map[string]interface{}, error) {
	body["inserted"] = "something"
	return body, m.writeErr
}
