package health

import (
	"fmt"
	"net/http"

	"github.com/Financial-Times/annotations-publisher/annotations"
	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

// HealthService runs application health checks, and provides the /__health http endpoint
type HealthService struct {
	fthealth.HealthCheck
	publisher annotations.Publisher
	writer    annotations.Writer
}

// NewHealthService returns a new HealthService
func NewHealthService(appSystemCode string, appName string, appDescription string, publisher annotations.Publisher, writer annotations.Writer) *HealthService {
	service := &HealthService{publisher: publisher, writer: writer}
	service.SystemCode = appSystemCode
	service.Name = appName
	service.Description = appDescription
	service.Checks = []fthealth.Check{
		service.publishCheck(),
		service.writeCheck(),
	}
	return service
}

// HealthCheckHandleFunc provides the http endpoint function
func (service *HealthService) HealthCheckHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return fthealth.Handler(service)
}

func (service *HealthService) publishCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "check-annotations-publish-health",
		BusinessImpact:   "Annotations Publishes to UPP may fail",
		Name:             "Check UPP for failures in the Publishing pipeline",
		PanicGuide:       "https://dewey.ft.com/annotations-publisher.html",
		Severity:         1,
		TechnicalSummary: fmt.Sprintf("UPP Publishing Pipeline is not available at %v", service.publisher.Endpoint()),
		Checker:          service.publishHealthChecker,
	}
}

func (service *HealthService) writeCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "check-annotations-save-health",
		BusinessImpact:   "Saving annotations to PAC during a publish may fail",
		Name:             "Check PAC for failures",
		PanicGuide:       "https://dewey.ft.com/annotations-publisher.html",
		Severity:         1,
		TechnicalSummary: fmt.Sprintf("PAC Save endpoint is not available at %v", service.writer.Endpoint()),
		Checker:          service.writerHealthChecker,
	}
}

func (service *HealthService) writerHealthChecker() (string, error) {
	if err := service.writer.GTG(); err != nil {
		return "PAC annotations writer is not healthy", err
	}
	return "PAC annotations writer is healthy", nil
}

func (service *HealthService) publishHealthChecker() (string, error) {
	if err := service.publisher.GTG(); err != nil {
		return "UPP Publishing Pipeline is not healthy", err
	}
	return "UPP Publishing Pipeline is healthy", nil
}

// GTG returns the current gtg status
func (service *HealthService) GTG() gtg.Status {
	return gtg.Status{GoodToGo: true, Message: "OK"} // even if UPP is unhealthy, we should still attempt to publish, and therefore remain ready
}
