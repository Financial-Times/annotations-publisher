package annotations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"errors"
)

type mockAnnotationsClient struct {
	mock.Mock
}

func (m *mockAnnotationsClient) GetAnnotations(ctx context.Context, uuid string) ([]Annotation, error) {
	args := m.Called(ctx, uuid)
	return args.Get(0).([]Annotation), args.Error(1)
}

func (m *mockAnnotationsClient) SaveAnnotations(ctx context.Context, uuid string, data []Annotation) ([]Annotation, error) {
	args := m.Called(ctx, uuid, data)
	return args.Get(0).([]Annotation), args.Error(1)
}

func (m *mockAnnotationsClient) GTG() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockAnnotationsClient) Endpoint() string {
	return "http://localhost"
}

func TestPublish(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(t, uuid, true, true)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", server.URL+"/__gtg")

	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), uuid, make(map[string]interface{}))
	assert.NoError(t, err)
}

func TestPublishFailsToMarshalBodyToJSON(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"/notify", "user:pass", "/__gtg")

	body := make(map[string]interface{})
	body["dodgy!"] = func() {}
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "json: unsupported type: func()")
}

func TestPublishFailsInvalidURL(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,":#", "user:pass", "/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func TestPublishRequestFailsServerUnavailable(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"/publish", "user:pass", "/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "Post /publish: unsupported protocol scheme \"\"")
}

func TestPublishRequestUnsuccessful(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(t, uuid, false, true)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", server.URL+"/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), uuid, body)
	assert.EqualError(t, err, fmt.Sprintf("Publish to %v/notify returned a 503 status code", server.URL))
}

func TestPublisherEndpoint(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"/publish", "user:pass", "/__gtg")
	assert.Equal(t, "/publish", publisher.Endpoint())
}

func TestPublisherAuthIsInvalid(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"/publish", "user", "/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "Invalid auth configured")

	// Now check for too many ':'s
	publisher = NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"/publish", "user:pass:anotherPass", "/__gtg")

	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "Invalid auth configured")
}

func TestPublisherAuthenticationFails(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(t, uuid, false, true)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:should-fail", server.URL+"/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "publish authentication is invalid")
}

func TestPublisherGTG(t *testing.T) {
	server := startMockServer(t, "", true, true)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"publishEndpoint", "user:pass", server.URL+"/__gtg")
	err := publisher.GTG()
	assert.NoError(t, err)
}

func TestPublisherGTGFails(t *testing.T) {
	server := startMockServer(t, "", true, false)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"publishEndpoint", "user:pass", server.URL+"/__gtg")
	err := publisher.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v returned a %v status code", server.URL+"/__gtg", http.StatusServiceUnavailable))
}

func TestPublisherGTGDoRequestFails(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"publishEndpoint", "user:pass", "/__gtg")
	err := publisher.GTG()
	assert.EqualError(t, err, "Get /__gtg: unsupported protocol scheme \"\"")
}

func TestPublisherGTGInvalidURL(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"publishEndpoint", "user:pass", ":#")
	err := publisher.GTG()
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func TestPublishFromStoreNotFound(t *testing.T) {
	uuid := uuid.New()

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return([]Annotation{}, ErrDraftNotFound)
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"http://www.example.com/notify", "user:pass", "http://www.example.com/__gtg")

	ctx := tid.TransactionAwareContext(context.Background(), "tid_test")
	err := publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, ErrDraftNotFound.Error())
}

func TestPublishFromStoreGetDraftsFails(t *testing.T) {
	uuid := uuid.New()
	msg := "test error"

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return([]Annotation{}, errors.New(msg))
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient,"http://www.example.com/notify", "user:pass", "http://www.example.com/__gtg")

	ctx := tid.TransactionAwareContext(context.Background(), "tid_test")
	err := publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, msg)
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

		tid := r.Header.Get("X-Request-Id")
		assert.Equal(t, "tid", tid)

		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "user", user)

		if pass != "pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		assert.Equal(t, "pass", pass)

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
