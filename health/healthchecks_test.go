package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublishedAnnotationsWriterCheck(t *testing.T) {
	mockGtg := &mockGtg{gtg: nil, endpoint: "/__gtg"}
	health := NewHealthService("appSystemCode", "appName", "appDescription", mockGtg, mockGtg, mockGtg)

	check := health.writerCheck()
	assert.Equal(t, "check-annotations-writer-health", check.ID)
	assert.Equal(t, "Annotations cannot be published to UPP", check.BusinessImpact)
	assert.Equal(t, "Check the PAC annotations R/W service", check.Name)
	assert.Equal(t, "https://dewey.ft.com/annotations-publisher.html", check.PanicGuide)
	assert.Equal(t, uint8(1), check.Severity)
	assert.Equal(t, "Generic R/W service for saving published annotations is not available at /__gtg", check.TechnicalSummary)

	msg, err := check.Checker()
	assert.Equal(t, "PAC annotations writer is healthy", msg)
	assert.NoError(t, err)
}

func TestPublishedAnnotationsWriterCheckFails(t *testing.T) {
	mockUnhealthy := &mockGtg{gtg: errors.New("eek"), endpoint: "/__gtg"}
	health := NewHealthService("appSystemCode", "appName", "appDescription", &mockGtg{}, mockUnhealthy, &mockGtg{})

	msg, err := health.writerCheck().Checker()
	assert.Equal(t, "PAC annotations writer is not healthy", msg)
	assert.EqualError(t, err, "eek")
}

func TestPublishCheck(t *testing.T) {
	mockGtg := &mockGtg{gtg: nil, endpoint: "/__gtg"}
	health := NewHealthService("appSystemCode", "appName", "appDescription", mockGtg, mockGtg, mockGtg)

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
	mockPublisher := &mockGtg{gtg: errors.New("eek"), endpoint: "/__gtg"}
	health := NewHealthService("appSystemCode", "appName", "appDescription", mockPublisher, &mockGtg{}, &mockGtg{})

	msg, err := health.publishCheck().Checker()
	assert.Equal(t, "UPP Publishing Pipeline is not healthy", msg)
	assert.EqualError(t, err, "eek")
}

func TestDraftsCheck(t *testing.T) {
	mockGtg := &mockGtg{gtg: nil, endpoint: "/__gtg"}
	health := NewHealthService("appSystemCode", "appName", "appDescription", mockGtg, mockGtg, mockGtg)

	check := health.draftsCheck()
	assert.Equal(t, "check-draft-annotations-health", check.ID)
	assert.Equal(t, "Annotations cannot be published to UPP", check.BusinessImpact)
	assert.Equal(t, "Check the PAC draft annotations api service", check.Name)
	assert.Equal(t, "https://dewey.ft.com/draft-annotations-api.html", check.PanicGuide)
	assert.Equal(t, uint8(1), check.Severity)
	assert.Equal(t, "Api for reading and saving draft annotations is not available at /__gtg", check.TechnicalSummary)

	msg, err := check.Checker()
	assert.Equal(t, "PAC drafts annotations reader writer is healthy", msg)
	assert.NoError(t, err)
}

func TestDraftAnnotationsFails(t *testing.T) {
	mockDraftAnnotations := &mockGtg{gtg: errors.New("eek"), endpoint: "/__gtg"}
	health := NewHealthService("appSystemCode", "appName", "appDescription", &mockGtg{}, &mockGtg{} ,mockDraftAnnotations)

	msg, err := health.draftsCheck().Checker()
	assert.Equal(t, "PAC drafts annotations reader writer is not healthy", msg)
	assert.EqualError(t, err, "eek")
}


func TestHealthServiceHandler(t *testing.T) {
	mockGtg := &mockGtg{gtg: nil, endpoint: "/__gtg"}
	health := NewHealthService("appSystemCode", "appName", "appDescription", mockGtg, mockGtg, mockGtg)

	handler := health.HealthCheckHandleFunc()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/__health", nil)

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body)
}

func TestGTGAllGood(t *testing.T) {
	health := NewHealthService("appSystemCode", "appName", "appDescription", &mockGtg{}, &mockGtg{}, &mockGtg{})

	gtg := health.GTG()
	assert.True(t, gtg.GoodToGo)
	assert.Equal(t, "OK", gtg.Message)
}

func TestGTGEvenThoughUPPIsUnhealthy(t *testing.T) {
	mockPublisher := &mockGtg{gtg: errors.New("eek"), endpoint: "/__gtg"}
	health := NewHealthService("appSystemCode", "appName", "appDescription", mockPublisher, &mockGtg{}, &mockGtg{})

	gtg := health.GTG()
	assert.True(t, gtg.GoodToGo)
	assert.Equal(t, "OK", gtg.Message)
}

func TestGTGFailsWhenWriterIsUnhealthy(t *testing.T) {
	mockPublisher := &mockGtg{}
	mockDraftAnnotations := &mockGtg{}
	mockUnhealthy := &mockGtg{gtg: errors.New("eek"), endpoint: "/__gtg"}
	health := NewHealthService("appSystemCode", "appName", "appDescription", mockPublisher, mockUnhealthy, mockDraftAnnotations)

	gtg := health.GTG()
	assert.False(t, gtg.GoodToGo)
	assert.Equal(t, "PAC annotations writer is not healthy", gtg.Message)
}

type mockGtg struct {
	gtg      error
	endpoint string
}

func (m *mockGtg) GTG() error {
	return m.gtg
}

func (m *mockGtg) Endpoint() string {
	return m.endpoint
}
