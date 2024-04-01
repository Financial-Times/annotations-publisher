package service

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/Financial-Times/annotations-publisher/notifier"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type timeoutError struct{ msg string }

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

type mockDraftAPI struct {
	mock.Mock
}

func (m *mockDraftAPI) SaveAnnotations(ctx context.Context, uuid string, hash string, body map[string]interface{}) (map[string]interface{}, string, error) {
	args := m.Called(ctx, uuid, hash, body)
	return args.Get(0).(map[string]interface{}), args.String(1), args.Error(2)
}
func (m *mockDraftAPI) GetAnnotations(ctx context.Context, uuid string) (map[string]interface{}, string, error) {
	args := m.Called(ctx, uuid)
	return args.Get(0).(map[string]interface{}), args.String(1), args.Error(2)
}
func (m *mockDraftAPI) GTG() error {
	args := m.Called()
	return args.Error(0)
}
func (m *mockDraftAPI) Endpoint() string {
	args := m.Called()
	return args.String(0)
}

type mockNotifierAPI struct {
	mock.Mock
}

func (m *mockNotifierAPI) GTG() error {
	args := m.Called()
	return args.Error(0)
}
func (m *mockNotifierAPI) Endpoint() string {
	args := m.Called()
	return args.String(0)
}
func (m *mockNotifierAPI) Publish(ctx context.Context, uuid string, body map[string]interface{}) error {
	args := m.Called(ctx, uuid, body)
	return args.Error(0)
}

type testCase struct {
	name                     string
	uuid                     string
	hash                     string
	body                     map[string]interface{}
	mockSaveAnnotationsError error
	mockGetAnnotationsError  error
	mockPublishError         error
	expectedError            error
	saveAnnotationsCalled    bool
	getAnnotationsCalled     bool
	publishCalled            bool
}

func TestSaveAndPublish(t *testing.T) {
	// Create a mock logger
	l := logger.NewUPPLogger("test", "debug")

	testCases := []testCase{
		{
			name:                     "Happy path",
			mockSaveAnnotationsError: nil,
			mockGetAnnotationsError:  nil,
			mockPublishError:         nil,
			expectedError:            nil,
			uuid:                     "test-uuid",
			hash:                     "test-hash",
			body:                     make(map[string]interface{}),
			saveAnnotationsCalled:    true,
			getAnnotationsCalled:     true,
			publishCalled:            true,
		},
		{
			name:                     "SaveAnnotations returns error",
			mockSaveAnnotationsError: fmt.Errorf("mock error"),
			mockGetAnnotationsError:  nil,
			mockPublishError:         nil,
			expectedError:            fmt.Errorf("mock error"),
			uuid:                     "test-uuid",
			hash:                     "test-hash",
			body:                     make(map[string]interface{}),
			saveAnnotationsCalled:    true,
			getAnnotationsCalled:     false,
			publishCalled:            false,
		},
		{
			name:                     "SaveAnnotations returns timeout error",
			mockSaveAnnotationsError: &net.OpError{Err: &timeoutError{}},
			mockGetAnnotationsError:  nil,
			mockPublishError:         nil,
			expectedError:            notifier.ErrServiceTimeout,
			uuid:                     "test-uuid",
			hash:                     "test-hash",
			body:                     make(map[string]interface{}),
			saveAnnotationsCalled:    true,
			getAnnotationsCalled:     false,
			publishCalled:            false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock APIs
			draftAPI := new(mockDraftAPI)
			notifierAPI := new(mockNotifierAPI)

			// Define the behavior of the mock APIs
			draftAPI.On("SaveAnnotations", mock.Anything, tc.uuid, tc.hash, tc.body).Return(tc.body, tc.hash, tc.mockSaveAnnotationsError)
			draftAPI.On("GetAnnotations", mock.Anything, tc.uuid).Return(tc.body, tc.hash, tc.mockGetAnnotationsError)
			notifierAPI.On("Publish", mock.Anything, tc.uuid, tc.body).Return(tc.mockPublishError)

			// Create a new service
			service := NewPublisher(l, draftAPI, notifierAPI)

			// Call the method under test
			err := service.SaveAndPublish(context.Background(), tc.uuid, tc.hash, tc.body)

			if err != nil {
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			// Assert that the mock methods were called with the expected arguments
			assertCalls(t, tc, draftAPI, notifierAPI)
		})
	}
}

func TestPublishFromStore(t *testing.T) {
	// Create a mock logger
	l := logger.NewUPPLogger("test", "debug")

	testCases := []testCase{
		{
			name:                     "Happy path",
			mockGetAnnotationsError:  nil,
			mockSaveAnnotationsError: nil,
			mockPublishError:         nil,
			expectedError:            nil,
			uuid:                     "test-uuid",
			hash:                     "test-hash",
			body:                     make(map[string]interface{}),
			getAnnotationsCalled:     true,
			saveAnnotationsCalled:    true,
			publishCalled:            true,
		},
		{
			name:                     "GetAnnotations returns error",
			mockGetAnnotationsError:  fmt.Errorf("mock error"),
			mockSaveAnnotationsError: nil,
			mockPublishError:         nil,
			expectedError:            fmt.Errorf("mock error"),
			uuid:                     "test-uuid",
			hash:                     "test-hash",
			body:                     make(map[string]interface{}),
			getAnnotationsCalled:     true,
			saveAnnotationsCalled:    false,
			publishCalled:            false,
		},
		{
			name:                     "GetAnnotations returns timeout error",
			mockGetAnnotationsError:  &net.OpError{Err: &timeoutError{}},
			mockSaveAnnotationsError: nil,
			mockPublishError:         nil,
			expectedError:            notifier.ErrServiceTimeout,
			uuid:                     "test-uuid",
			hash:                     "test-hash",
			body:                     make(map[string]interface{}),
			getAnnotationsCalled:     true,
			saveAnnotationsCalled:    false,
			publishCalled:            false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock APIs
			draftAPI := new(mockDraftAPI)
			notifierAPI := new(mockNotifierAPI)

			// Define the behavior of the mock APIs
			draftAPI.On("GetAnnotations", mock.Anything, tc.uuid).Return(make(map[string]interface{}), tc.hash, tc.mockGetAnnotationsError)
			draftAPI.On("SaveAnnotations", mock.Anything, tc.uuid, tc.hash, tc.body).Return(tc.body, tc.hash, tc.mockSaveAnnotationsError)
			notifierAPI.On("Publish", mock.Anything, tc.uuid, mock.Anything).Return(tc.mockPublishError)
			// Create a new service
			service := NewPublisher(l, draftAPI, notifierAPI)

			// Call the method under test
			err := service.PublishFromStore(context.Background(), tc.uuid)

			if err != nil {
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			// Assert that the mock methods were called with the expected arguments
			assertCalls(t, tc, draftAPI, notifierAPI)
		})
	}
}

func TestIsTimeoutErr(t *testing.T) {
	err := &timeoutError{msg: "This is a timeout error"}
	assert.True(t, isTimeoutErr(err))
}

func assertCalls(t *testing.T, tc testCase, mockDraft *mockDraftAPI, mockNotifier *mockNotifierAPI) {
	t.Helper()
	if tc.saveAnnotationsCalled {
		mockDraft.AssertCalled(t, "SaveAnnotations", mock.Anything, tc.uuid, tc.hash, tc.body)
	} else {
		mockDraft.AssertNotCalled(t, "SaveAnnotations", mock.Anything, tc.uuid, tc.hash, tc.body)
	}

	if tc.getAnnotationsCalled {
		mockDraft.AssertCalled(t, "GetAnnotations", mock.Anything, tc.uuid)
	} else {
		mockDraft.AssertNotCalled(t, "GetAnnotations", mock.Anything, tc.uuid)
	}
	if tc.publishCalled {
		mockNotifier.AssertCalled(t, "Publish", mock.Anything, tc.uuid, tc.body)
	} else {
		mockNotifier.AssertNotCalled(t, "Publish", mock.Anything, tc.uuid, tc.body)
	}
}
