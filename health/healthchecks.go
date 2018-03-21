package health

import (
	"fmt"
	"net/http"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

type ExternalService interface {
	Endpoint() string
	GTG() error
}

// HealthService runs application health checks, and provides the /__health http endpoint
type HealthService struct {
	fthealth.HealthCheck
	publisher ExternalService
	writer    ExternalService
	draftsRW  ExternalService
}

// NewHealthService returns a new HealthService
func NewHealthService(appSystemCode string, appName string, appDescription string, publisher ExternalService, writer ExternalService, draftsRW ExternalService) *HealthService {
	service := &HealthService{publisher: publisher, writer: writer, draftsRW: draftsRW}
	service.SystemCode = appSystemCode
	service.Name = appName
	service.Description = appDescription
	service.Checks = []fthealth.Check{
		service.writerCheck(),
		service.publishCheck(),
		service.draftsCheck(),
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

func (service *HealthService) publishHealthChecker() (string, error) {
	if err := service.publisher.GTG(); err != nil {
		return "UPP Publishing Pipeline is not healthy", err
	}
	return "UPP Publishing Pipeline is healthy", nil
}

func (service *HealthService) writerCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "check-annotations-writer-health",
		BusinessImpact:   "Annotations cannot be published to UPP",
		Name:             "Check the PAC annotations R/W service",
		PanicGuide:       "https://dewey.ft.com/annotations-publisher.html",
		Severity:         1,
		TechnicalSummary: fmt.Sprintf("Generic R/W service for saving published annotations is not available at %v", service.writer.Endpoint()),
		Checker:          service.writerHealthChecker,
	}
}

func (service *HealthService) writerHealthChecker() (string, error) {
	if err := service.writer.GTG(); err != nil {
		return "PAC annotations writer is not healthy", err
	}
	return "PAC annotations writer is healthy", nil
}

func (service *HealthService) draftsCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "check-draft-annotations-health",
		BusinessImpact:   "Annotations cannot be published to UPP",
		Name:             "Check the PAC draft annotations api service",
		PanicGuide:       "https://dewey.ft.com/draft-annotations-api.html",
		Severity:         1,
		TechnicalSummary: fmt.Sprintf("Api for reading and saving draft annotations is not available at %v", service.draftsRW.Endpoint()),
		Checker:          service.draftsHealthChecker,
	}
}

func (service *HealthService) draftsHealthChecker() (string, error) {
	if err := service.draftsRW.GTG(); err != nil {
		return "PAC drafts annotations reader writer is not healthy", err
	}
	return "PAC drafts annotations reader writer is healthy", nil
}

func (service *HealthService) GTG() gtg.Status {

	writerCheck := func() gtg.Status {
		msg, err := service.writerCheck().Checker()
		if err != nil {
			return gtg.Status{GoodToGo: false, Message: msg}
		}

		return gtg.Status{GoodToGo: true, Message: "OK"}
	}

	// switch to 'gtg.FailFastParallelCheck' if there are multiple checkers in the future.
	return gtg.FailFastSequentialChecker([]gtg.StatusChecker{writerCheck})()
}
