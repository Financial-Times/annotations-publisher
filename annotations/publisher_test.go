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

	"github.com/Financial-Times/go-ft-http/fthttp"
	"github.com/Financial-Times/go-logger/v2"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type testTimeoutError struct {
	error
}

func (te testTimeoutError) Timeout() bool {
	return true
}
func (te testTimeoutError) Temporary() bool {
	return false
}

type mockAnnotationsClient struct {
	mock.Mock
}

func (m *mockAnnotationsClient) GetAnnotations(ctx context.Context, uuid string) (AnnotationsBody, string, error) {
	args := m.Called(ctx, uuid)
	return args.Get(0).(AnnotationsBody), args.String(1), args.Error(2)
}

func (m *mockAnnotationsClient) SaveAnnotations(ctx context.Context, uuid string, hash string, data AnnotationsBody) (AnnotationsBody, string, error) {
	args := m.Called(ctx, uuid, hash, data)
	return args.Get(0).(AnnotationsBody), args.String(1), args.Error(2)
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
	server := startMockServer(context.Background(), t, uuid, true, true, time.Duration(0))
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", server.URL+"/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), uuid, make(map[string]interface{}))
	assert.NoError(t, err)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFailsToMarshalBodyToJSON(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/notify", "user:pass", "/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	body := make(map[string]interface{})
	body["dodgy!"] = func() {}
	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "json: unsupported type: func()")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFailsInvalidURL(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, ":#", "user:pass", "/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	body := make(map[string]interface{})
	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "parse \":\": missing protocol scheme")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishRequestFailsServerUnavailable(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/publish", "user:pass", "/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	body := make(map[string]interface{})
	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "Post \"/publish\": unsupported protocol scheme \"\"")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishRequestUnsuccessful(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(context.Background(), t, uuid, false, true, time.Duration(0))
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", server.URL+"/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	body := make(map[string]interface{})
	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), uuid, body)
	assert.EqualError(t, err, fmt.Sprintf("publish to %v/notify returned a 503 status code", server.URL))

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublisherEndpoint(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/publish", "user:pass", "/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	assert.Equal(t, "/publish", publisher.Endpoint())

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublisherAuthIsInvalid(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/publish", "user", "/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	body := make(map[string]interface{})
	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "invalid auth configured")

	// Now check for too many ':'s
	publisher = NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "/publish", "user:pass:anotherPass", "/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "invalid auth configured")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublisherAuthenticationFails(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(context.Background(), t, uuid, false, true, time.Duration(0))
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:should-fail", server.URL+"/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	body := make(map[string]interface{})
	err = publisher.Publish(tid.TransactionAwareContext(context.Background(), "tid"), "a-valid-uuid", body)
	assert.EqualError(t, err, "publish authentication is invalid")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublisherPublishToUppTimeout(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(context.Background(), t, uuid, true, true, 100*time.Millisecond)
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", server.URL+"/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid"), 10*time.Millisecond)
	defer cancel()

	body := make(map[string]interface{})
	err = publisher.Publish(ctx, uuid, body)
	assert.EqualError(t, err, "downstream service timed out")

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublisherGTG(t *testing.T) {
	server := startMockServer(context.Background(), t, "", true, true, time.Duration(0))
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "publishEndpoint", "user:pass", server.URL+"/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	err = publisher.GTG()
	assert.NoError(t, err)
}

func TestPublisherGTGFails(t *testing.T) {
	server := startMockServer(context.Background(), t, "", true, false, time.Duration(0))
	defer server.Close()

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "publishEndpoint", "user:pass", server.URL+"/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	err = publisher.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v returned a %v status code for UPP cms-metadata-notifier service", server.URL+"/__gtg", http.StatusServiceUnavailable))
}

func TestPublisherGTGDoRequestFails(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "publishEndpoint", "user:pass", "/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	err = publisher.GTG()
	assert.EqualError(t, err, "Get \"/__gtg\": unsupported protocol scheme \"\"")
}

func TestPublisherGTGInvalidURL(t *testing.T) {
	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "publishEndpoint", "user:pass", ":#", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	err = publisher.GTG()
	assert.EqualError(t, err, "parse \":\": missing protocol scheme")
}

func TestPublishFromStore(t *testing.T) {
	uuid := uuid.New()
	testAnnotations := AnnotationsBody{[]Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}
	testHash := "hashhashhashhash"
	updatedHash := "newhashnewhash"

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, testHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(testAnnotations, updatedHash, nil)

	publishedAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, updatedHash, testAnnotations).Return(testAnnotations, updatedHash, nil)

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()

	server := startMockServer(ctx, t, uuid, true, true, time.Duration(0))
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	err = publisher.PublishFromStore(ctx, uuid)
	assert.NoError(t, err)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreNotFound(t *testing.T) {
	uuid := uuid.New()

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(AnnotationsBody{}, "", ErrDraftNotFound)
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "http://www.example.com/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	err = publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, ErrDraftNotFound.Error())

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreDraftAnnotationsGetTimeOut(t *testing.T) {
	uuid := uuid.New()

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(AnnotationsBody{}, "", testTimeoutError{errors.New("dealine exceeded")})
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "http://www.example.com/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	err = publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, ErrServiceTimeout.Error())

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreGetDraftsFails(t *testing.T) {
	uuid := uuid.New()
	msg := "test error"

	draftAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(AnnotationsBody{}, "", errors.New(msg))
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "http://www.example.com/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	err = publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, msg)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreSaveDraftFails(t *testing.T) {
	msg := "test error"
	uuid := uuid.New()
	testAnnotations := AnnotationsBody{[]Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}

	testHash := "hashhashhashhash"

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, testHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(AnnotationsBody{}, "", errors.New(msg))

	publishedAnnotationsClient := &mockAnnotationsClient{}

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	server := startMockServer(ctx, t, uuid, true, true, time.Duration(0))
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	err = publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, msg)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreSaveDraftTimeout(t *testing.T) {
	uuid := uuid.New()
	testAnnotations := AnnotationsBody{[]Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}

	testHash := "hashhashhashhash"

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, testHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(AnnotationsBody{}, "", testTimeoutError{errors.New("dealine exceeded")})

	publishedAnnotationsClient := &mockAnnotationsClient{}

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	server := startMockServer(ctx, t, uuid, true, true, time.Duration(0))
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	err = publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, ErrServiceTimeout.Error())

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreSavePublishedFails(t *testing.T) {
	msg := "test error"
	uuid := uuid.New()
	testAnnotations := AnnotationsBody{[]Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}
	testHash := "hashhashhashhash"
	updatedHash := "newhashnewhash"

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, testHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(testAnnotations, updatedHash, nil)

	publishedAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, updatedHash, testAnnotations).Return(AnnotationsBody{}, "", errors.New(msg))

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	server := startMockServer(ctx, t, uuid, true, true, time.Duration(0))
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	err = publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, msg)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStoreSavePublishedTimeout(t *testing.T) {
	uuid := uuid.New()
	testAnnotations := AnnotationsBody{[]Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}
	testHash := "hashhashhashhash"
	updatedHash := "newhashnewhash"

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, testHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(testAnnotations, updatedHash, nil)

	publishedAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, updatedHash, testAnnotations).Return(AnnotationsBody{}, "", testTimeoutError{errors.New("dealine exceeded")})

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	server := startMockServer(ctx, t, uuid, true, true, time.Duration(0))
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	err = publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, ErrServiceTimeout.Error())

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestPublishFromStorePublishFails(t *testing.T) {
	uuid := uuid.New()
	testAnnotations := AnnotationsBody{[]Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, "", nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, "", testAnnotations).Return(testAnnotations, "", nil)

	publishedAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, "", testAnnotations).Return(testAnnotations, "", nil)

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	server := startMockServer(ctx, t, uuid, false, true, time.Duration(0))
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	err = publisher.PublishFromStore(ctx, uuid)
	assert.EqualError(t, err, fmt.Sprintf("publish to %v/notify returned a 503 status code", server.URL))

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestSaveAndPublish(t *testing.T) {
	uuid := uuid.New()
	testAnnotations := AnnotationsBody{[]Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}
	testHash := "hashhashhashhash"
	updatedHash := "newhashnewhash"
	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("GetAnnotations", mock.Anything, uuid).Return(testAnnotations, updatedHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(testAnnotations, updatedHash, nil)
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, updatedHash, testAnnotations).Return(testAnnotations, updatedHash, nil)

	publishedAnnotationsClient := &mockAnnotationsClient{}
	publishedAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, updatedHash, testAnnotations).Return(testAnnotations, updatedHash, nil)

	server := startMockServer(ctx, t, uuid, true, true, time.Duration(0))
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, server.URL+"/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	err = publisher.SaveAndPublish(ctx, uuid, testHash, testAnnotations)
	assert.NoError(t, err)

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestSaveAndPublishNotFound(t *testing.T) {
	uuid := uuid.New()
	testHash := "hashhashhashhash"
	testAnnotations := AnnotationsBody{[]Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(AnnotationsBody{}, "", ErrDraftNotFound)
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "http://www.example.com/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	err = publisher.SaveAndPublish(ctx, uuid, testHash, testAnnotations)
	assert.EqualError(t, err, ErrDraftNotFound.Error())

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func TestSaveAndPublishDraftSaveAnnotationsTimeout(t *testing.T) {
	uuid := uuid.New()
	testHash := "hashhashhashhash"
	testAnnotations := AnnotationsBody{[]Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}

	draftAnnotationsClient := &mockAnnotationsClient{}
	draftAnnotationsClient.On("SaveAnnotations", mock.Anything, uuid, testHash, testAnnotations).Return(AnnotationsBody{}, "", testTimeoutError{errors.New("dealine exceeded")})
	publishedAnnotationsClient := &mockAnnotationsClient{}
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	publisher := NewPublisher("originSystemID", draftAnnotationsClient, publishedAnnotationsClient, "http://www.example.com/notify", "user:pass", "http://www.example.com/__gtg", testingClient, logger.NewUPPLogger("test", "DEBUG"))

	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), "tid_test"), 50*time.Millisecond)
	defer cancel()
	err = publisher.SaveAndPublish(ctx, uuid, testHash, testAnnotations)
	assert.EqualError(t, err, ErrServiceTimeout.Error())

	draftAnnotationsClient.AssertExpectations(t)
	publishedAnnotationsClient.AssertExpectations(t)
}

func startMockServer(ctx context.Context, t *testing.T, uuid string, publishOk bool, gtgOk bool, delay time.Duration) *httptest.Server {
	r := vestigo.NewRouter()
	r.Get("/__gtg", func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC-test-annotations-publisher/Version--is-not-a-semantic-version", userAgent)

		if !gtgOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Post("/notify", func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC-test-annotations-publisher/Version--is-not-a-semantic-version", userAgent)

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

		if delay > 0 {
			time.Sleep(delay)
		}

		if !publishOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	return httptest.NewServer(r)
}

func TestIsTimeoutErr(t *testing.T) {
	r := vestigo.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	})
	s := httptest.NewServer(r)
	req, _ := http.NewRequest("GET", s.URL+"/", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := http.DefaultClient.Do(req.WithContext(ctx))
	assert.True(t, isTimeoutErr(err))
}
