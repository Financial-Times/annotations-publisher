package annotations

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/Financial-Times/annotations-publisher/health"
	status "github.com/Financial-Times/service-status-go/httphandlers"
)

type PublishedAnnotationsWriter interface {
	health.ExternalService
}

type genericRWClient struct {
	client          *http.Client
	rwEndpoint string
	gtgEndpoint     string
}

func NewPublishedAnnotationsWriter(endpoint string) (PublishedAnnotationsWriter, error) {
	v, err := url.Parse(fmt.Sprintf(endpoint, "dummy"))
	if err != nil {
		return nil, err
	}

	gtg, _ := url.Parse(status.GTGPath)
	gtgUrl := v.ResolveReference(gtg)

	return &genericRWClient{client: &http.Client{}, rwEndpoint: endpoint, gtgEndpoint: gtgUrl.String()}, nil
}

func (rw *genericRWClient) GTG() error {
	req, err := http.NewRequest("GET", rw.gtgEndpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", userAgent)
	resp, err := rw.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GTG %v returned a %v status code", rw.gtgEndpoint, resp.StatusCode)
	}

	return nil
}

func (rw *genericRWClient) Endpoint() string {
	return rw.rwEndpoint
}
