package health

import (
	"fmt"
	"net/http"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

type externalServiceChecker interface {
	Endpoint() string
	GTG() error
}

// service runs application health checks, and provides the /__health http endpoint
type service struct {
	fthealth.HealthCheck
	publisher externalServiceChecker
	draftsRW  externalServiceChecker
}

// NewHealthService returns a new HealthService
func NewHealthService(appSystemCode string, appName string, appDescription string, publisher externalServiceChecker, draftsRW externalServiceChecker) *service {
	service := &service{publisher: publisher, draftsRW: draftsRW}
	service.SystemCode = appSystemCode
	service.Name = appName
	service.Description = appDescription
	service.Checks = []fthealth.Check{
		service.publishCheck(),
		service.draftsCheck(),
	}
	return service
}

//nolint:all
func (s *service) GTG() gtg.Status {
	var checks []gtg.StatusChecker

	for idx := range s.Checks {
		check := s.Checks[idx]

		checks = append(checks, func() gtg.Status {
			if _, err := check.Checker(); err != nil {
				return gtg.Status{GoodToGo: false, Message: err.Error()}
			}
			return gtg.Status{GoodToGo: true}
		})
	}
	return gtg.FailFastParallelCheck(checks)()
}

// HealthCheckHandleFunc provides the http endpoint function
func (s *service) HealthCheckHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return fthealth.Handler(s)
}

func (s *service) publishCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "check-annotations-publish-health",
		BusinessImpact:   "Annotations Publishes to UPP may fail",
		Name:             "Check UPP for failures in the Publishing pipeline",
		PanicGuide:       "https://dewey.ft.com/annotations-publisher.html",
		Severity:         1,
		TechnicalSummary: fmt.Sprintf("UPP Publishing Pipeline is not available at %v", s.publisher.Endpoint()),
		Checker:          s.publishHealthChecker,
	}
}

func (s *service) publishHealthChecker() (string, error) {
	if err := s.publisher.GTG(); err != nil {
		return "UPP Publishing Pipeline is not healthy", err
	}
	return "UPP Publishing Pipeline is healthy", nil
}

func (s *service) draftsCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "check-draft-annotations-health",
		BusinessImpact:   "Annotations cannot be published to UPP",
		Name:             "Check the PAC draft annotations api service",
		PanicGuide:       "https://dewey.ft.com/draft-annotations-api.html",
		Severity:         1,
		TechnicalSummary: fmt.Sprintf("Api for reading and saving draft annotations is not available at %v", s.draftsRW.Endpoint()),
		Checker:          s.draftsHealthChecker,
	}
}

func (s *service) draftsHealthChecker() (string, error) {
	if err := s.draftsRW.GTG(); err != nil {
		return "PAC drafts annotations reader writer is not healthy", err
	}
	return "PAC drafts annotations reader writer is healthy", nil
}
