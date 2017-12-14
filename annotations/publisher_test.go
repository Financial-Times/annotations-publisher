package annotations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var timeout = time.Duration(8 * time.Second)

type mockAnnotationsClient struct {
	mock.Mock
}

func (m *mockAnnotationsClient) GetAnnotations(ctx context.Context, uuid string) ([]Annotation, string, error) {
	args := m.Called(ctx, uuid)
	return args.Get(0).([]Annotation), args.String(1), args.Error(2)
}

func (m *mockAnnotationsClient) SaveAnnotations(ctx context.Context, uuid string, hash string, data []Annotation) ([]Annotation, string, error) {
	args := m.Called(ctx, uuid, hash, data)
	return args.Get(0).([]Annotation), args.String(1), args.Error(2)
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
	server := startMockServer(t, context.Background(), uuid, true, true)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", server.URL+"/__gtg", timeout)

	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), uuid, make(map[string]interface{}))
	assert.NoError(t, err)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFailsToMarshalBodyToJSON(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/notify", "user:pass", "/__gtg", timeout)

	body := make(map[string]interface{})
	body["dodgy!"] = func() {}
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "json: unsupported type: func()")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFailsInvalidURL(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, ":#", "user:pass", "/__gtg", timeout)

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "parse :: missing protocol scheme")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishRequestFailsServerUnavailable(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/publish", "user:pass", "/__gtg", timeout)

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "Post /publish: unsupported protocol scheme \"\"")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishRequestUnsuccessful(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(t, context.Background(), uuid, false, true)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", server.URL+"/__gtg", timeout)

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), uuid, body)
	assert.EqualError(t, err, fmt.Sprintf("Publish to %v/notify returned a 503 status code", server.URL))

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublisherEndpoint(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/publish", "user:pass", "/__gtg", timeout)
	assert.Equal(t, "/publish", publisher.Endpoint())

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublisherAuthIsInvalid(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/publish", "user", "/__gtg", timeout)

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "Invalid auth configured")

	// Now check for too many ':'s
	publisher = NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/publish", "user:pass:anotherPass", "/__gtg", timeout)

	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "Invalid auth configured")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublisherAuthenticationFails(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(t, context.Background(), uuid, false, true)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:should-fail", server.URL+"/__gtg", timeout)

	body := make(map[string]interface{})
	err := publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "publish authentication is invalid")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublisherGTG(t *testing.T) {
	server := startMockServer(t, context.Background(), "", true, true)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "publishEndpoint", "user:pass", server.URL+"/__gtg", timeout)
	err := publisher.GTG()
	assert.NoError(t, err)
}

func TestPublisherGTGFails(t *testing.T) {
	server := startMockServer(t, context.Background(), "", true, false)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "publishEndpoint", "user:pass", server.URL+"/__gtg", timeout)
	err := publisher.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v returned a %v status code", server.URL+"/__gtg", http.StatusServiceUnavailable))
}

func TestPublisherGTGDoRequestFails(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "publishEndpoint", "user:pass", "/__gtg", timeout)
	err := publisher.GTG()
	assert.EqualError(t, err, "Get /__gtg: unsupported protocol scheme \"\"")
}

func TestPublisherGTGInvalidURL(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "publishEndpoint", "user:pass", ":#", timeout)
	err := publisher.GTG()
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func TestPublishFromStore(t *testing.T) {
	uuid := uuid.New()
	testAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}
	testHash := "hashhashhashhash"
	updatedHash := "newhashnewhash"

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, testHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(testAnnotations, updatedHash, nil)

	publishedAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, updatedHash, testAnnotations).Return(testAnnotations, updatedHash, nil)

	ctx := tid.TransactionAwareContext(context.Background(), "tid_test")
	server := startMockServer(t, ctx, uuid, true, true)
	defer server.Close()

	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", timeout)

	err := publisher.PublishFromStore(ctx, uuid)
	assert.NoError(t, err)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreNotFound(t *testing.T) {
	uuid := uuid.New()

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return([]Annotation{}, "", ErrDraftNotFound)
	publishedAnnotationsClient := &mockAnnotationsClient{}
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "http://www.example.com/notify", "user:pass", "http://www.example.com/__gtg", timeout)

	ctx := tid.TransactionAwareContext(context.Background(), "tid_test")
	err := publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, ErrDraftNotFound.Error())

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreGetDraftsFails(t *testing.T) {
	uuid := uuid.New()
	msg := "test error"

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return([]Annotation{}, "", errors.New(msg))
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "http://www.example.com/notify", "user:pass", "http://www.example.com/__gtg", timeout)

	ctx := tid.TransactionAwareContext(context.Background(), "tid_test")
	err := publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, msg)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreSaveDraftFails(t *testing.T) {
	msg := "test error"
	uuid := uuid.New()
	testAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}

	testHash := "hashhashhashhash"

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, testHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return([]Annotation{}, "", errors.New(msg))

	publishedAnnotationsClient := &mockAnnotationsClient{}

	ctx := tid.TransactionAwareContext(context.Background(), "tid_test")
	server := startMockServer(t, ctx, uuid, true, true)
	defer server.Close()

	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", timeout)

	err := publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, msg)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreSavePublishedFails(t *testing.T) {
	msg := "test error"
	uuid := uuid.New()
	testAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}

	testHash := "hashhashhashhash"
	updatedHash := "newhashnewhash"

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, testHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(testAnnotations, updatedHash, nil)

	publishedAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, updatedHash, testAnnotations).Return([]Annotation{}, "", errors.New(msg))

	ctx := tid.TransactionAwareContext(context.Background(), "tid_test")
	server := startMockServer(t, ctx, uuid, true, true)
	defer server.Close()

	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", timeout)

	err := publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, msg)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStorePublishFails(t *testing.T) {
	uuid := uuid.New()
	testAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, "", nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, "", testAnnotations).Return(testAnnotations, "", nil)

	publishedAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, "", testAnnotations).Return(testAnnotations, "", nil)

	ctx := tid.TransactionAwareContext(context.Background(), "tid_test")
	server := startMockServer(t, ctx, uuid, false, true)
	defer server.Close()

	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", timeout)

	err := publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, fmt.Sprintf("Publish to %v/notify returned a 503 status code", server.URL))

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func startMockServer(t *testing.T, ctx context.Context, uuid string, publishOk bool, gtgOk bool) *httptest.Server {
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

		txid := r.Header.Get("X-Request-Id")
		if expectedTid, err := tid.GetTransactionIDFromContext(ctx); err == nil && expectedTid != "" {
			assert.Equal(t, expectedTid, txid)
		} else {
			assert.Equal(t, "tid", txid)
		}

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
