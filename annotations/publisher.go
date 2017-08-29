package annotations

import (
	"fmt"
	"net/http"
)

type Publisher interface {
	GTG() error
	Endpoint() string
}

type uppPublisher struct {
	originSystemID  string
	publishEndpoint string
	gtgEndpoint     string
}

func NewPublisher(originSystemID string, publishEndpoint string, gtgEndpoint string) Publisher {
	return &uppPublisher{originSystemID, publishEndpoint, gtgEndpoint}
}

func (a *uppPublisher) GTG() error {
	req, err := http.NewRequest("GET", a.gtgEndpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", "PAC annotations-publisher")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GTG %v returned a %v status code", a.gtgEndpoint, resp.StatusCode)
	}

	return nil
}

func (a *uppPublisher) Endpoint() string {
	return a.publishEndpoint
}
