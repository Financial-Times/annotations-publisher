package notifier

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type timeoutError struct{ msg string }

func (e *timeoutError) Error() string   { return e.msg }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

type mockHTTPClient struct{ mock.Mock }

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestPublish(t *testing.T) {
	mockClient := new(mockHTTPClient)
	log := logger.NewUPPLogger("draft_test", "DEBUG")
	mockAPI := &API{
		client:          mockClient,
		publishEndpoint: "http://cms-metadata-notifier:8080/notify",
		gtgEndpoint:     "http://cms-metadata-notifier:8080/__gtg",
		logger:          log,
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, "transaction_id", "test")
	ctx = context.WithValue(ctx, CtxOriginSystemIDKey(OriginSystemIDHeader), "test")

	testCases := []struct {
		name          string
		mockError     error
		expectedError error
		ctx           context.Context
		body          map[string]interface{}
		mockResponse  *http.Response
		useMockClient bool
	}{
		{
			name:          "Happy path",
			mockError:     nil,
			expectedError: nil,
			ctx:           ctx,
			body:          map[string]interface{}{},
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`))},
			useMockClient: true,
		},
		{
			name:          "Brake marshalling",
			mockError:     nil,
			expectedError: fmt.Errorf("json: unsupported type: func()"),
			ctx:           ctx,
			body: map[string]interface{}{
				"func": func() {},
			},
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
			},
			useMockClient: false,
		},
		{
			name:          "Successful do with non-200 status code",
			mockError:     nil,
			expectedError: fmt.Errorf("publish to %v returned a %v status code", mockAPI.publishEndpoint, http.StatusBadRequest),
			ctx:           ctx,
			body:          map[string]interface{}{},
			mockResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`))},
			useMockClient: true,
		},
		{
			name:          "Do error",
			mockError:     fmt.Errorf("do error"),
			ctx:           ctx,
			expectedError: fmt.Errorf("do error"),
			body:          map[string]interface{}{},
			mockResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
			},
			useMockClient: true,
		},
		{
			name:          "Timeout error",
			mockError:     &timeoutError{msg: "This is a timeout error"},
			ctx:           ctx,
			expectedError: ErrServiceTimeout,
			body:          map[string]interface{}{},
			mockResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
			},
			useMockClient: true,
		},
		{
			name:          "Brake new request",
			expectedError: fmt.Errorf("parse \":\": missing protocol scheme"),
			ctx:           ctx,
			body:          map[string]interface{}{},
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
			},
			useMockClient: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.useMockClient {
				mockClient.On("Do", mock.Anything).Return(tc.mockResponse, tc.mockError)
			}
			if tc.name == "Brake new request" {
				mockAPI.publishEndpoint = ":"
			}
			err := mockAPI.Publish(tc.ctx, "test", tc.body)
			if err != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			if tc.useMockClient {
				mockClient.AssertExpectations(t)
			}
		})
	}
}

func TestGTG(t *testing.T) {
	mockClient := new(mockHTTPClient)
	log := logger.NewUPPLogger("draft_test", "DEBUG")
	mockAPI := &API{
		client:          mockClient,
		publishEndpoint: "http://cms-metadata-notifier:8080/notify",
		gtgEndpoint:     "http://cms-metadata-notifier:8080/__gtg",
		logger:          log,
	}
	testCases := []struct {
		name          string
		mockError     error
		expectedError error
		mockResponse  *http.Response
		useMockClient bool
	}{
		{
			name:          "Status OK",
			mockResponse:  &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			useMockClient: true,
		},
		{
			name:          "Do error",
			mockResponse:  &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			mockError:     fmt.Errorf("do error"),
			expectedError: fmt.Errorf("do error"),
			useMockClient: true,
		},
		{
			name:          "Status Bad Request",
			mockResponse:  &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			expectedError: fmt.Errorf("GTG %s returned a %d status code for UPP cms-metadata-notifier service", mockAPI.gtgEndpoint, http.StatusBadRequest),
			useMockClient: true,
		},
		{
			name:          "Break new request",
			expectedError: fmt.Errorf("parse \":\": missing protocol scheme"),
			mockResponse:  &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			useMockClient: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.useMockClient {
				mockClient.On("Do", mock.Anything).Return(tc.mockResponse, tc.mockError).Once()
			}

			// Break the new request test case
			if tc.name == "Break new request" {
				mockAPI.gtgEndpoint = ":"
			}

			err := mockAPI.GTG()
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			if tc.useMockClient {
				mockClient.AssertExpectations(t)
			}
		})
	}
}

func TestNewAPI(t *testing.T) {
	log := logger.NewUPPLogger("draft_test", "DEBUG")
	api := NewAPI("http://cms-metadata-notifier:8080/notify", "http://cms-metadata-notifier:8080/__gtg", &http.Client{}, log)
	assert.Equal(t, "http://cms-metadata-notifier:8080/notify", api.publishEndpoint)
	assert.Equal(t, "http://cms-metadata-notifier:8080/__gtg", api.gtgEndpoint)
	assert.Equal(t, log, api.logger)
}

func TestEndpoint(t *testing.T) {
	log := logger.NewUPPLogger("draft_test", "DEBUG")
	api := NewAPI("http://cms-metadata-notifier:8080/notify", "http://cms-metadata-notifier:8080/__gtg", &http.Client{}, log)
	assert.Equal(t, "http://cms-metadata-notifier:8080/notify", api.Endpoint())
}

func TestIsTimeoutErr(t *testing.T) {
	err := &timeoutError{msg: "This is a timeout error"}
	assert.True(t, isTimeoutErr(err))

}
