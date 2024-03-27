package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Financial-Times/annotations-publisher/draft"
	"github.com/Financial-Times/annotations-publisher/notifier"
	"github.com/Financial-Times/go-logger/v2"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPublisher struct {
	mock.Mock
}

func (m *mockPublisher) PublishFromStore(ctx context.Context, uuid string) error {
	args := m.Called(ctx, uuid)
	return args.Error(0)
}

func (m *mockPublisher) SaveAndPublish(ctx context.Context, uuid string, hash string, body map[string]interface{}) error {
	args := m.Called(ctx, uuid, hash, body)
	return args.Error(0)
}

type mockJSONValidator struct {
	mock.Mock
}

func (m *mockJSONValidator) Validate(i interface{}) error {
	args := m.Called(i)
	return args.Error(0)
}

type mockSchemaHandler struct {
	mock.Mock
}

func (m *mockSchemaHandler) ListSchemas(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

func (m *mockSchemaHandler) GetSchema(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

type pubTestCase struct {
	name                      string
	body                      []byte
	uuid                      string
	origin                    string
	tid                       string
	previousHash              string
	mockSavePublishError      error
	mockPublishFromStoreError error
	mockValidationError       error
	expectedMsg               string
	expectedStatus            int
	callValidatorMocks        bool
	callSaveAndPublishMocks   bool
	callPublishFromStoreMocks bool
	fromStore                 bool
}

func TestPublish(t *testing.T) {
	l := logger.NewUPPLogger("test", "debug")

	testCases := []pubTestCase{
		{
			name:                    "Happy path",
			body:                    []byte(`{"predicate": "foo", "id": "bar"}`),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "testOrigin",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    nil,
			mockValidationError:     nil,
			expectedMsg:             "{\"message\":\"Publish accepted\"}",
			expectedStatus:          http.StatusAccepted,
			callSaveAndPublishMocks: true,
			callValidatorMocks:      true,
			fromStore:               false,
		},
		{
			name:                    "Missing origin",
			body:                    []byte(`{"predicate": "foo", "id": "bar"}`),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    nil,
			mockValidationError:     nil,
			expectedMsg:             "{\"message\":\"Invalid request: X-Origin-System-Id header missing\"}",
			expectedStatus:          http.StatusBadRequest,
			callValidatorMocks:      false,
			callSaveAndPublishMocks: false,
			fromStore:               false,
		},
		{
			name:                    "FromStore with body",
			body:                    []byte(`{"predicate": "foo", "id": "bar"}`),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "testOrigin",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    nil,
			mockValidationError:     nil,
			expectedMsg:             "{\"message\":\"A request body cannot be provided when fromStore=true\"}",
			expectedStatus:          http.StatusBadRequest,
			callValidatorMocks:      false,
			callSaveAndPublishMocks: false,
			fromStore:               true,
		},
		{
			name:                    "Request with empty body",
			body:                    []byte(``),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "testOrigin",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    nil,
			mockValidationError:     nil,
			expectedMsg:             "{\"message\":\"Please provide a valid json request body\"}",
			expectedStatus:          http.StatusBadRequest,
			callValidatorMocks:      false,
			callSaveAndPublishMocks: false,
			fromStore:               false,
		},
		{
			name:                    "Request with invalid body",
			body:                    []byte(`{"predicate": "foo", "id": "bar",`),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "testOrigin",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    nil,
			mockValidationError:     nil,
			expectedMsg:             "{\"message\":\"Failed to process request json. Please provide a valid json request body\"}",
			expectedStatus:          http.StatusBadRequest,
			callValidatorMocks:      false,
			callSaveAndPublishMocks: false,
			fromStore:               false,
		},
		{
			name:                    "Request with invalid schema",
			body:                    []byte(`{"invalid": "json"}`),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "testOrigin",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    nil,
			mockValidationError:     fmt.Errorf("invalid schema"),
			expectedMsg:             "{\"message\":\"Failed to validate json schema. Please provide a valid json request body\"}",
			expectedStatus:          http.StatusBadRequest,
			callValidatorMocks:      true,
			callSaveAndPublishMocks: false,
			fromStore:               false,
		},
		{
			name:                    "Request with SaveAndPublish error",
			body:                    []byte(`{"predicate": "foo", "id": "bar"}`),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "testOrigin",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    fmt.Errorf("saveAndPublish error"),
			mockValidationError:     nil,
			expectedMsg:             "{\"message\":\"saveAndPublish error\"}",
			expectedStatus:          http.StatusServiceUnavailable,
			callValidatorMocks:      true,
			callSaveAndPublishMocks: true,
			fromStore:               false,
		},
		{
			name:                    "Request with PublishFromStore error",
			body:                    []byte(`{"predicate": "foo", "id": "bar"}`),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "testOrigin",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    notifier.ErrDraftNotFound,
			mockValidationError:     nil,
			expectedMsg:             "{\"message\":\"draft was not found\"}",
			expectedStatus:          http.StatusNotFound,
			callValidatorMocks:      true,
			callSaveAndPublishMocks: true,
			fromStore:               false,
		},
		{
			name:                    "Request with successful SaveAndPublish",
			body:                    []byte(`{"predicate": "foo", "id": "bar"}`),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "testOrigin",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    nil,
			mockValidationError:     nil,
			expectedMsg:             "{\"message\":\"Publish accepted\"}",
			expectedStatus:          http.StatusAccepted,
			callValidatorMocks:      true,
			callSaveAndPublishMocks: true,
			fromStore:               false,
		},
		{
			name:                    "Request with SaveAndPublish timeout",
			body:                    []byte(`{"predicate": "foo", "id": "bar"}`),
			uuid:                    "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                  "testOrigin",
			tid:                     "tid_1234",
			previousHash:            "testHash",
			mockSavePublishError:    notifier.ErrServiceTimeout,
			mockValidationError:     nil,
			expectedMsg:             "{\"message\":\"downstream service timed out\"}",
			expectedStatus:          http.StatusGatewayTimeout,
			callValidatorMocks:      true,
			callSaveAndPublishMocks: true,
			fromStore:               false,
		}, {
			name:                      "Request with SaveAndPublish generic error",
			body:                      []byte(``),
			uuid:                      "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                    "testOrigin",
			tid:                       "tid_1234",
			previousHash:              "testHash",
			mockPublishFromStoreError: fmt.Errorf("generic error"),
			mockValidationError:       nil,
			expectedMsg:               "{\"message\":\"Unable to publish annotations from store\"}",
			expectedStatus:            http.StatusInternalServerError,
			callValidatorMocks:        false,
			callSaveAndPublishMocks:   false,
			callPublishFromStoreMocks: true,
			fromStore:                 true,
		},
		{
			name:                      "Publish from store",
			body:                      []byte(``),
			uuid:                      "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                    "testOrigin",
			tid:                       "tid_1234",
			previousHash:              "testHash",
			mockPublishFromStoreError: nil,
			mockValidationError:       nil,
			expectedMsg:               "{\"message\":\"Publish accepted\"}",
			expectedStatus:            http.StatusAccepted,
			callValidatorMocks:        false,
			callSaveAndPublishMocks:   false,
			callPublishFromStoreMocks: true,
			fromStore:                 true,
		},
		{
			name:                      "Publish from store with gateway timeout",
			body:                      []byte(``),
			uuid:                      "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                    "testOrigin",
			tid:                       "tid_1234",
			previousHash:              "testHash",
			mockPublishFromStoreError: notifier.ErrServiceTimeout,
			mockValidationError:       nil,
			expectedMsg:               "{\"message\":\"downstream service timed out\"}",
			expectedStatus:            http.StatusGatewayTimeout,
			callValidatorMocks:        false,
			callSaveAndPublishMocks:   false,
			callPublishFromStoreMocks: true,
			fromStore:                 true,
		},
		{
			name:                      "Publish from store with status not found",
			body:                      []byte(``),
			uuid:                      "8f054e68-999f-11e7-a652-cde3f882dd7b",
			origin:                    "testOrigin",
			tid:                       "tid_1234",
			previousHash:              "testHash",
			mockPublishFromStoreError: notifier.ErrDraftNotFound,
			mockValidationError:       nil,
			expectedMsg:               "{\"message\":\"draft was not found\"}",
			expectedStatus:            http.StatusNotFound,
			callValidatorMocks:        false,
			callSaveAndPublishMocks:   false,
			callPublishFromStoreMocks: true,
			fromStore:                 true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPub := new(mockPublisher)
			mockV := new(mockJSONValidator)
			mockSH := new(mockSchemaHandler)
			handler := NewHandler(l, mockPub, mockV, mockSH)

			r := mux.NewRouter()
			r.HandleFunc("/drafts/content/{uuid}/annotations/publish", handler.Publish).Methods("POST")

			endpoint := fmt.Sprintf("/drafts/content/%s/annotations/publish", tc.uuid)
			req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(tc.body))
			if err != nil {
				t.Fatal(err)
			}

			if tc.fromStore {
				q := req.URL.Query()
				q.Add("fromStore", "true")
				req.URL.RawQuery = q.Encode()
			}
			req.Header.Set(tid.TransactionIDHeader, tc.tid)
			req.Header.Set(notifier.OriginSystemIDHeader, tc.origin)
			req.Header.Set(draft.PreviousDocumentHashHeader, tc.previousHash)

			mockPub.On("SaveAndPublish", mock.Anything, tc.uuid, tc.previousHash, mock.Anything).Return(tc.mockSavePublishError)
			mockPub.On("PublishFromStore", mock.Anything, tc.uuid).Return(tc.mockPublishFromStoreError)
			mockV.On("Validate", mock.Anything).Return(tc.mockValidationError)

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			assert.Equal(t, tc.expectedMsg, rr.Body.String())
			assertCalls(t, tc, mockPub, mockV)
		})
	}
}

func TestValidate(t *testing.T) {
	l := logger.NewUPPLogger("test", "debug")
	testCases := []struct {
		name                string
		body                []byte
		tid                 string
		mockValidationError error
		expectedMsg         string
		expectedStatus      int
		callValidatorMocks  bool
	}{
		{
			name:                "Happy path",
			body:                []byte(`{"predicate": "foo", "id": "bar"}`),
			tid:                 "tid_1234",
			mockValidationError: nil,
			expectedMsg:         "",
			expectedStatus:      http.StatusOK,
			callValidatorMocks:  true,
		},
		{
			name:                "Break unmarshalling",
			body:                []byte(`{"predicate": "foo", "id": "bar",`), // Invalid JSON
			tid:                 "tid_1234",
			mockValidationError: nil,
			expectedMsg:         "{\"message\":\"Failed to unmarshal request body: unexpected end of JSON input\"}",
			expectedStatus:      http.StatusBadRequest,
			callValidatorMocks:  false,
		},
		{
			name:                "Validation error",
			body:                []byte(`{"invalid": "json"}`), // JSON that does not conform to the expected schema
			tid:                 "tid_1234",
			mockValidationError: fmt.Errorf("invalid schema"),
			expectedMsg:         "{\"message\":\"Failed to validate request body: invalid schema\"}",
			expectedStatus:      http.StatusBadRequest,
			callValidatorMocks:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPub := new(mockPublisher)
			mockV := new(mockJSONValidator)
			mockSH := new(mockSchemaHandler)
			handler := NewHandler(l, mockPub, mockV, mockSH)

			r := mux.NewRouter()
			r.HandleFunc("/validate", handler.Validate).Methods("POST")

			req, err := http.NewRequest("POST", "/validate", bytes.NewBuffer(tc.body))
			if err != nil {
				t.Fatal(err)
			}

			req.Header.Set(tid.TransactionIDHeader, tc.tid)

			mockV.On("Validate", mock.Anything).Return(tc.mockValidationError)

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			assert.Equal(t, tc.expectedMsg, rr.Body.String())
			if tc.callValidatorMocks {
				mockV.AssertCalled(t, "Validate", mock.Anything)
			} else {
				mockV.AssertNotCalled(t, "Validate", mock.Anything)
			}
		})
	}
}

func TestListSchemas(t *testing.T) {
	l := logger.NewUPPLogger("test", "debug")
	mockPub := new(mockPublisher)
	mockV := new(mockJSONValidator)
	mockSH := new(mockSchemaHandler)
	handler := NewHandler(l, mockPub, mockV, mockSH)

	r := mux.NewRouter()
	r.HandleFunc("/schemas", handler.ListSchemas).Methods("GET")

	req, err := http.NewRequest("GET", "/schemas", nil)
	if err != nil {
		t.Fatal(err)
	}

	mockSH.On("ListSchemas", mock.Anything, mock.Anything)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockSH.AssertCalled(t, "ListSchemas", mock.Anything, mock.Anything)
}

func TestGetSchema(t *testing.T) {
	l := logger.NewUPPLogger("test", "debug")
	mockPub := new(mockPublisher)
	mockV := new(mockJSONValidator)
	mockSH := new(mockSchemaHandler)
	handler := NewHandler(l, mockPub, mockV, mockSH)

	r := mux.NewRouter()
	r.HandleFunc("/schemas/{id}", handler.GetSchema).Methods("GET")

	req, err := http.NewRequest("GET", "/schemas/test-id", nil)
	if err != nil {
		t.Fatal(err)
	}

	mockSH.On("GetSchema", mock.Anything, mock.Anything)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockSH.AssertCalled(t, "GetSchema", mock.Anything, mock.Anything)
}

func assertCalls(t *testing.T, tc pubTestCase, mockPub *mockPublisher, mockV *mockJSONValidator) {
	t.Helper()
	if tc.callSaveAndPublishMocks {
		mockPub.AssertCalled(t, "SaveAndPublish", mock.Anything, tc.uuid, tc.previousHash, mock.Anything)
	} else {
		mockPub.AssertNotCalled(t, "SaveAndPublish", mock.Anything, tc.uuid, tc.previousHash, mock.Anything)
	}

	if tc.callValidatorMocks {
		mockV.AssertCalled(t, "Validate", mock.Anything)
	} else {
		mockV.AssertNotCalled(t, "Validate", mock.Anything)
	}

	if tc.callPublishFromStoreMocks {
		mockPub.AssertCalled(t, "PublishFromStore", mock.Anything, tc.uuid)
	} else {
		mockPub.AssertNotCalled(t, "PublishFromStore", mock.Anything, tc.uuid)
	}
}
