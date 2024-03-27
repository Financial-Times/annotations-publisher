package server

import (
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	l "github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/service-status-go/gtg"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockHandler struct {
	mock.Mock
}

func (m *mockHandler) Publish(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

func (m *mockHandler) Validate(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

func (m *mockHandler) ListSchemas(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

func (m *mockHandler) GetSchema(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

type mockHealthChecker struct {
	mock.Mock
}

func (m *mockHealthChecker) GTG() gtg.Status {
	args := m.Called()
	return args.Get(0).(gtg.Status)
}

func (m *mockHealthChecker) HealthCheckHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	args := m.Called()
	return args.Get(0).(func(w http.ResponseWriter, r *http.Request))
}

func TestSetupRoutes(t *testing.T) {
	testCases := []struct {
		name           string
		route          string
		expectedStatus int
		method         string
	}{
		{
			name:           "Publish",
			route:          "/drafts/content/{uuid}/annotations/publish",
			expectedStatus: http.StatusOK,
			method:         http.MethodPost,
		},
		{
			name:           "Validate",
			route:          "/validate",
			expectedStatus: http.StatusOK,
			method:         http.MethodPost,
		},
		{
			name:           "ListSchemas",
			route:          "/schemas",
			expectedStatus: http.StatusOK,
			method:         http.MethodGet,
		},
		{
			name:           "GetSchema",
			route:          "/schemas/{schemaName}",
			expectedStatus: http.StatusOK,
			method:         http.MethodGet,
		},
	}

	h := new(mockHandler)
	healthService := new(mockHealthChecker)
	logger := l.NewUPPLogger("test", "info")
	s := New(nil, nil, h, healthService, logger)
	routerFunc := s.setupRoutes()
	router := mux.NewRouter()
	routerFunc(router)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, tc.route, nil)
			if err != nil {
				t.Fatal(err)
			}

			h.On(tc.name, mock.Anything, mock.Anything).Return()

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			h.AssertCalled(t, tc.name, mock.Anything, mock.Anything)
		})
	}
}

func TestStartServer(t *testing.T) {
	// Create a mock handler
	h := new(mockHandler)
	h.On("ListSchemas", mock.Anything, mock.Anything).Return()

	// Create a mock health checker
	healthService := new(mockHealthChecker)
	healthService.On("GTG").Return(gtg.Status{GoodToGo: true})
	healthService.On("HealthCheckHandleFunc").Return(func(_ http.ResponseWriter, _ *http.Request) {})

	// Create a mock logger
	logger := l.NewUPPLogger("test", "info")
	port := 8181
	apiYml := "../api/api.yml"
	// Call the Start function
	s := New(&port, &apiYml, h, healthService, logger)
	go s.Start()

	time.Sleep(3 * time.Second)

	resp, err := http.Get("http://localhost:8181/schemas")
	assert.NoError(t, err)
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", resp.Status)
	}

	// Assert that the handler's ListSchemas method was called
	h.AssertCalled(t, "ListSchemas", mock.Anything, mock.Anything)
}
func TestListenForShutdownSignal(t *testing.T) {
	h := new(mockHandler)
	healthService := new(mockHealthChecker)

	// Create a mock logger
	logger := l.NewUPPLogger("test", "info")
	port := 8080
	apiYml := "../api/api.yml"
	// Create a new Server instance
	s := New(&port, &apiYml, h, healthService, logger)

	shutdown := s.listenForShutdownSignal()

	// Send a signal after a short delay
	go func() {
		time.Sleep(time.Millisecond * 100)
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	select {
	case <-shutdown:
		// Pass the test if we received a shutdown signal
	case <-time.After(time.Second):
		// Fail the test if we didn't receive a shutdown signal within a second
		t.Fatal("did not receive shutdown signal")
	}
}

type mockServer struct {
	mock.Mock
}

func (m *mockServer) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockServer) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestWaitForShutdownSignal(t *testing.T) {
	h := new(mockHandler)
	healthService := new(mockHealthChecker)
	logger := l.NewUPPLogger("test", "info")
	port := 8080
	apiYml := "../api/api.yml"

	s := New(&port, &apiYml, h, healthService, logger)

	// Create a mock server
	srv := new(mockServer)
	srv.On("Close").Return(nil)

	// Create a shutdown channel and send a signal
	shutdown := make(chan bool, 1)
	shutdown <- true

	// Call the function
	s.waitForShutdownSignal(srv, shutdown)

	// Assert that the server's Close method was called
	srv.AssertCalled(t, "Close")
}
