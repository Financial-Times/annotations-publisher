package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublishCheck(t *testing.T) {
	mockPublisher := &mockPublisher{gtg: nil, endpoint: "/__gtg"}
	mockWriter := &mockWriter{gtg: nil, endpoint: "/__gtg"}

	health := NewHealthService("appSystemCode", "appName", "appDescription", mockPublisher, mockWriter)

	check := health.publishCheck()
	assert.Equal(t, "check-annotations-publish-health", check.ID)
	assert.Equal(t, "Annotations Publishes to UPP may fail", check.BusinessImpact)
	assert.Equal(t, "Check UPP for failures in the Publishing pipeline", check.Name)
	assert.Equal(t, "https://dewey.ft.com/annotations-publisher.html", check.PanicGuide)
	assert.Equal(t, uint8(1), check.Severity)
	assert.Equal(t, "UPP Publishing Pipeline is not available at /__gtg", check.TechnicalSummary)

	msg, err := check.Checker()
	assert.Equal(t, "UPP Publishing Pipeline is healthy", msg)
	assert.NoError(t, err)
}

func TestPublishCheckFails(t *testing.T) {
	mockPublisher := &mockPublisher{gtg: errors.New("eek"), endpoint: "/__gtg"}
	mockWriter := &mockWriter{gtg: nil, endpoint: "/__gtg"}

	health := NewHealthService("appSystemCode", "appName", "appDescription", mockPublisher, mockWriter)

	msg, err := health.publishCheck().Checker()
	assert.Equal(t, "UPP Publishing Pipeline is not healthy", msg)
	assert.EqualError(t, err, "eek")
}

func TestWriterCheckFails(t *testing.T) {
	mockPublisher := &mockPublisher{gtg: nil, endpoint: "/__gtg"}
	mockWriter := &mockWriter{gtg: errors.New("eek"), endpoint: "/__gtg"}

	health := NewHealthService("appSystemCode", "appName", "appDescription", mockPublisher, mockWriter)

	msg, err := health.writeCheck().Checker()
	assert.Equal(t, "PAC annotations writer is not healthy", msg)
	assert.EqualError(t, err, "eek")
}

func TestHealthServiceHandler(t *testing.T) {
	mockPublisher := &mockPublisher{gtg: nil, endpoint: "/__gtg"}
	mockWriter := &mockWriter{gtg: nil, endpoint: "/__gtg"}

	health := NewHealthService("appSystemCode", "appName", "appDescription", mockPublisher, mockWriter)

	handler := health.HealthCheckHandleFunc()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/__health", nil)

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body)
}

func TestGTG(t *testing.T) {
	mockPublisher := &mockPublisher{gtg: errors.New("eek"), endpoint: "/__gtg"}
	mockWriter := &mockWriter{gtg: nil, endpoint: "/__gtg"}

	health := NewHealthService("appSystemCode", "appName", "appDescription", mockPublisher, mockWriter)

	gtg := health.GTG()
	assert.True(t, gtg.GoodToGo)
	assert.Equal(t, "OK", gtg.Message)
}

type mockPublisher struct {
	gtg      error
	endpoint string
}

func (m *mockPublisher) GTG() error {
	return m.gtg
}

func (m *mockPublisher) Publish(uuid string, tid string, body map[string]interface{}) error {
	return nil
}

func (m *mockPublisher) Endpoint() string {
	return m.endpoint
}

type mockWriter struct {
	gtg      error
	endpoint string
}

func (m *mockWriter) GTG() error {
	return m.gtg
}

func (m *mockWriter) Write(uuid string, tid string, body map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockWriter) Endpoint() string {
	return m.endpoint
}
