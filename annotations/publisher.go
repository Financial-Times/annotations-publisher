package annotations

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrInvalidAuthentication occurs when UPP responds with a 401
var ErrInvalidAuthentication = errors.New("Publish authentication is invalid")

// Publisher provides an interface to publish annotations to UPP
type Publisher interface {
	GTG() error
	Endpoint() string
	Publish(uuid string, tid string, body map[string]interface{}) error
}

type uppPublisher struct {
	client          *http.Client
	originSystemID  string
	publishEndpoint string
	publishAuth     string
	gtgEndpoint     string
}

// NewPublisher returns a new Publisher instance
func NewPublisher(client *http.Client, originSystemID string, publishEndpoint string, publishAuth string, gtgEndpoint string) Publisher {
	return &uppPublisher{client: client, originSystemID: originSystemID, publishEndpoint: publishEndpoint, publishAuth: publishAuth, gtgEndpoint: gtgEndpoint}
}

// Publish sends the annotations to UPP via the configured publishEndpoint. Requests contain X-Origin-System-Id and X-Request-Id and a User-Agent as provided.
func (a *uppPublisher) Publish(uuid string, tid string, body map[string]interface{}) error {
	body["uuid"] = uuid
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", a.publishEndpoint, bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}

	err = a.addBasicAuth(req)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", "PAC annotations-publisher")
	req.Header.Add("X-Origin-System-Id", a.originSystemID)
	req.Header.Add("X-Request-Id", tid)
	req.Header.Add("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrInvalidAuthentication
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Publish to %v returned a %v status code", a.publishEndpoint, resp.StatusCode)
	}

	return nil
}

func (a *uppPublisher) addBasicAuth(r *http.Request) error {
	auth := strings.Split(a.publishAuth, ":")
	if len(auth) != 2 {
		return errors.New("Invalid auth configured")
	}
	r.SetBasicAuth(auth[0], auth[1])
	return nil
}

// GTG performs a health check against the UPP publish endpoint
func (a *uppPublisher) GTG() error {
	req, err := http.NewRequest("GET", a.gtgEndpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", "PAC annotations-publisher")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GTG %v returned a %v status code", a.gtgEndpoint, resp.StatusCode)
	}

	return nil
}

// Endpoint returns the configured publish endpoint
func (a *uppPublisher) Endpoint() string {
	return a.publishEndpoint
}
