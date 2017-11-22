package annotations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Financial-Times/annotations-publisher/health"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"bytes"
)

type AnnotationsClient interface {
	health.ExternalService
	GetAnnotations(ctx context.Context, uuid string) (interface{}, error)
	SaveAnnotations(ctx context.Context, uuid string, data interface{}) (interface{}, error)
}

type genericRWClient struct {
	client          *http.Client
	rwEndpoint string
	gtgEndpoint     string
}

func NewAnnotationsClient(endpoint string) (AnnotationsClient, error) {
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

func (rw *genericRWClient) GetAnnotations(ctx context.Context, uuid string) (interface{}, error) {
	draftsUrl := fmt.Sprintf(rw.rwEndpoint, uuid)
	req, err := http.NewRequest("GET", draftsUrl, nil)
	if err != nil {
		return nil, err
	}

	txid, _ := tid.GetTransactionIDFromContext(ctx)

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Request-Id", txid)
	req.Header.Set("Accept", "application/json")

	resp, err := rw.client.Do(req)
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

func (rw *genericRWClient) SaveAnnotations(ctx context.Context, uuid string, data interface{}) (interface{}, error) {
	draftsUrl := fmt.Sprintf(rw.rwEndpoint, uuid)
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("PUT", draftsUrl, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	txid, _ := tid.GetTransactionIDFromContext(ctx)

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Request-Id", txid)
	req.Header.Set("Accept", "application/json")

	resp, err := rw.client.Do(req)
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
