package draft

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/Financial-Times/annotations-publisher/notifier"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestGTG(t *testing.T) {
	mockClient := new(MockHTTPClient)
	log := logger.NewUPPLogger("draft_test", "DEBUG")
	mockAPI := &API{
		client:      mockClient,
		rwEndpoint:  "http://localhost:8080/__drafts/",
		gtgEndpoint: "http://localhost:8080/__gtg",
		logger:      log,
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
			expectedError: fmt.Errorf("GTG %v returned a %v status code for generic-rw-aurora", mockAPI.gtgEndpoint, http.StatusBadRequest),
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

func TestGetAnnotations(t *testing.T) {
	mockClient := new(MockHTTPClient)
	log := logger.NewUPPLogger("draft_test", "DEBUG")
	mockAPI := &API{
		client:      mockClient,
		rwEndpoint:  "http://localhost:8080/drafts/content/%v/annotations",
		gtgEndpoint: "http://localhost:8080/__gtg",
		logger:      log,
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, notifier.CtxOriginSystemIDKey(notifier.OriginSystemIDHeader), "test")

	testCases := []struct {
		name          string
		mockError     error
		expectedError error
		ctx           context.Context
		mockResponse  *http.Response
		useMockClient bool
	}{
		{
			name:          "Status OK",
			mockResponse:  &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			ctx:           ctx,
			useMockClient: true,
		},
		{
			name:          "Missing X-Origin header",
			mockResponse:  &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			ctx:           context.Background(),
			expectedError: ErrMissingOriginHeader,
			useMockClient: false,
		},
		{
			name:          "Do error",
			mockResponse:  &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			mockError:     fmt.Errorf("do error"),
			expectedError: fmt.Errorf("do error"),
			useMockClient: true,
			ctx:           ctx,
		},
		{
			name:          "Status Bad Request",
			mockResponse:  &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			mockError:     nil,
			expectedError: fmt.Errorf("read from http://localhost:8080/drafts/content/test/annotations returned a 400 status code"),
			useMockClient: true,
			ctx:           ctx,
		},
		{
			name:          "Draft not found",
			mockResponse:  &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			mockError:     nil,
			expectedError: notifier.ErrDraftNotFound,
			useMockClient: true,
			ctx:           ctx,
		},

		{
			name:          "Break new request",
			mockResponse:  &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))},
			ctx:           ctx,
			expectedError: fmt.Errorf("parse \"://draft-annotations-api:8080/drafts/content/test/annotations\": missing protocol scheme"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.useMockClient {
				mockClient.On("Do", mock.Anything).Return(tc.mockResponse, tc.mockError).Once()
			}

			if tc.name == "Break new request" {
				mockAPI.rwEndpoint = "://draft-annotations-api:8080/drafts/content/%v/annotations"
			}
			_, _, err := mockAPI.GetAnnotations(tc.ctx, "test")

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

func TestAPI_SaveAnnotations(t *testing.T) {
	mockClient := new(MockHTTPClient)
	log := logger.NewUPPLogger("draft_test", "DEBUG")
	mockAPI := &API{
		client:      mockClient,
		rwEndpoint:  "http://localhost:8080/drafts/content/%v/annotations",
		gtgEndpoint: "http://localhost:8080/__gtg",
		logger:      log,
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, notifier.CtxOriginSystemIDKey(notifier.OriginSystemIDHeader), "test")

	testCases := []struct {
		name               string
		mockError          error
		expectedError      error
		ctx                context.Context
		mockResponse       *http.Response
		useMockClient      bool
		testAnnotation     map[string]interface{}
		expectedAnnotation map[string]interface{}
		expectedHashHeader string
	}{
		{
			name: "Status OK",
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
				Header:     http.Header{"Document-Hash": []string{"hash"}},
			},
			ctx:           ctx,
			useMockClient: true,
			testAnnotation: map[string]interface{}{
				"predicate": "foo",
				"id":        "bar",
			},
			expectedAnnotation: map[string]interface{}{
				"predicate": "foo",
				"id":        "bar",
			},
			expectedHashHeader: "hash",
		},
		{
			name: "Brake marshalling",
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
				Header:     http.Header{"Document-Hash": []string{"hash"}},
			},
			ctx:                ctx,
			useMockClient:      false,
			expectedAnnotation: map[string]interface{}{},
			testAnnotation: map[string]interface{}{
				"Func": func() {},
			},
			expectedHashHeader: "",
			expectedError:      fmt.Errorf("json: unsupported type: func()"),
		},
		{
			name: "Missing origin header",
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
				Header:     http.Header{"Document-Hash": []string{"hash"}},
			},
			ctx:                context.Background(),
			useMockClient:      false,
			expectedAnnotation: map[string]interface{}{},
			expectedError:      ErrMissingOriginHeader,
		},
		{
			name: "Do error",
			mockResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
				Header:     http.Header{"Document-Hash": []string{"hash"}},
			},
			ctx:           ctx,
			useMockClient: true,
			testAnnotation: map[string]interface{}{
				"predicate": "foo",
				"id":        "bar",
			},
			expectedAnnotation: map[string]interface{}{},
			mockError:          fmt.Errorf("do error"),
			expectedError:      fmt.Errorf("do error"),
		},
		{
			name: "Bad request",
			mockResponse: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
				Header:     http.Header{"Document-Hash": []string{"hash"}},
			},
			ctx:           ctx,
			useMockClient: true,
			testAnnotation: map[string]interface{}{
				"predicate": "foo",
				"id":        "bar",
			},
			expectedAnnotation: map[string]interface{}{},
			expectedHashHeader: "",
			expectedError:      fmt.Errorf("write to http://localhost:8080/drafts/content/test/annotations returned a 400 status code"),
		},
		{
			name: "Status OK with non-empty body",
			mockResponse: &http.Response{
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
				Header:        http.Header{"Document-Hash": []string{"hash"}},
				ContentLength: int64(len(`{"predicate": "foo", "id": "bar"}`)),
			},
			ctx:           ctx,
			useMockClient: true,
			testAnnotation: map[string]interface{}{
				"predicate": "foo",
				"id":        "bar",
			},
			expectedAnnotation: map[string]interface{}{
				"predicate": "foo",
				"id":        "bar",
			},
			expectedHashHeader: "hash",
		},
		{
			name: "Brake new request",
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"predicate": "foo", "id": "bar"}`)),
				Header:     http.Header{"Document-Hash": []string{"hash"}},
			},
			ctx:                ctx,
			useMockClient:      false,
			expectedAnnotation: map[string]interface{}{},
			expectedError:      fmt.Errorf("parse \"://draft-annotations-api:8080/drafts/content/test/annotations\": missing protocol scheme"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.useMockClient {
				mockClient.On("Do", mock.Anything).Return(tc.mockResponse, tc.mockError).Once()
			}

			if tc.name == "Brake new request" {
				mockAPI.rwEndpoint = "://draft-annotations-api:8080/drafts/content/%v/annotations"
			}

			ann, hashHeader, err := mockAPI.SaveAnnotations(tc.ctx, "test", "hash", tc.testAnnotation)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			if tc.useMockClient {
				mockClient.AssertExpectations(t)
			}
			assert.Equal(t, tc.expectedAnnotation, ann)
			assert.Equal(t, tc.expectedHashHeader, hashHeader)
		})
	}
}

func TestNewAPI(t *testing.T) {
	log := logger.NewUPPLogger("draft_test", "DEBUG")
	api := NewAPI("http://localhost:8080/drafts/content/%v/annotations", "http://localhost:8080/__gtg", &http.Client{}, log)
	assert.Equal(t, "http://localhost:8080/drafts/content/%v/annotations", api.rwEndpoint)
	assert.Equal(t, "http://localhost:8080/__gtg", api.gtgEndpoint)
	assert.Equal(t, log, api.logger)
}
