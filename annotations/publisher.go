package annotations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Financial-Times/annotations-publisher/health"
	tid "github.com/Financial-Times/transactionid-utils-go"
)

const (
	userAgent = "PAC annotations-publisher"
)

var (
	// ErrInvalidAuthentication occurs when UPP responds with a 401
	ErrInvalidAuthentication = errors.New("publish authentication is invalid")
	ErrDraftNotFound         = errors.New("draft was not found")
)

// Publisher provides an interface to publish annotations to UPP
type Publisher interface {
	health.ExternalService
	Publish(uuid string, tid string, body map[string]interface{}) error
	GetDraft(ctx context.Context, uuid string) (interface{}, error)
	SaveDraft(ctx context.Context, uuid string, data interface{}) (interface{}, error)
}

type uppPublisher struct {
	client          *http.Client
	originSystemID  string
	draftsEndpoint  string
	publishEndpoint string
	publishAuth     string
	gtgEndpoint     string
}

// NewPublisher returns a new Publisher instance
func NewPublisher(originSystemID string, draftsEndpoint string, publishEndpoint string, publishAuth string, gtgEndpoint string) Publisher {
	return &uppPublisher{client: &http.Client{}, originSystemID: originSystemID, draftsEndpoint: draftsEndpoint, publishEndpoint: publishEndpoint, publishAuth: publishAuth, gtgEndpoint: gtgEndpoint}
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

	req.Header.Add("User-Agent", userAgent)
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

	req.Header.Add("User-Agent", userAgent)
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

func (a *uppPublisher) GetDraft(ctx context.Context, uuid string) (interface{}, error) {
	draftsUrl := fmt.Sprintf(a.draftsEndpoint, uuid)
	req, err := http.NewRequest("GET", draftsUrl, nil)
	if err != nil {
		return nil, err
	}

	txid, _ := tid.GetTransactionIDFromContext(ctx)

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Request-Id", txid)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrDraftNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Read from %v returned a %v status code", draftsUrl, resp.StatusCode)
	}

	body := make(map[string]interface{})
	ann := []interface{}{}
	err = json.NewDecoder(resp.Body).Decode(&ann)
	if err != nil {
		return nil, err
	}
	body["annotations"] = ann

	return body, nil
}

func (a *uppPublisher) SaveDraft(ctx context.Context, uuid string, data interface{}) (interface{}, error) {
	draftsUrl := fmt.Sprintf(a.draftsEndpoint, uuid)
	req, err := http.NewRequest("PUT", draftsUrl, nil)
	if err != nil {
		return nil, err
	}

	txid, _ := tid.GetTransactionIDFromContext(ctx)

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Request-Id", txid)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		body := make(map[string]interface{})
		ann := []interface{}{}
		err = json.NewDecoder(resp.Body).Decode(&ann)
		if err != nil {
			return nil, err
		}
		body["annotations"] = ann

		return body, nil
	}

	return nil, fmt.Errorf("Write to %v returned a %v status code", draftsUrl, resp.StatusCode)
}
