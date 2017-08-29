package health

import (
	"fmt"
	"net/http"

	"github.com/Financial-Times/annotations-publisher/annotations"
	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

type HealthService struct {
	fthealth.HealthCheck
	publisher annotations.Publisher
}

func NewHealthService(appSystemCode string, appName string, appDescription string, publisher annotations.Publisher) *HealthService {
	service := &HealthService{publisher: publisher}
	service.SystemCode = appSystemCode
	service.Name = appName
	service.Description = appDescription
	service.Checks = []fthealth.Check{
		service.publishCheck(),
	}
	return service
}

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

func (service *HealthService) publishHealthChecker() (string, error) {
	if err := service.publisher.GTG(); err != nil {
		return "UPP Publishing Pipeline is not healthy", err
	}
	return "UPP Publishing Pipeline is healthy", nil
}

func (service *HealthService) GTG() gtg.Status {
	return gtg.Status{GoodToGo: true, Message: "OK"} // even if UPP is unhealthy, we should still attempt to publish, and therefore remain ready
}
