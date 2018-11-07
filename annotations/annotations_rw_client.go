package annotations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Financial-Times/annotations-publisher/health"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	log "github.com/sirupsen/logrus"
)

const (
	DocumentHashHeader         = "Document-Hash"
	PreviousDocumentHashHeader = "Previous-Document-Hash"
)

type AnnotationsClient interface {
	health.ExternalService
	GetAnnotations(ctx context.Context, uuid string) (AnnotationsBody, string, error)
	SaveAnnotations(ctx context.Context, uuid string, hash string, data AnnotationsBody) (AnnotationsBody, string, error)
}

type genericRWClient struct {
	client      *http.Client
	rwEndpoint  string
	gtgEndpoint string
}

func NewAnnotationsClient(endpoint string, client *http.Client) (AnnotationsClient, error) {
	v, err := url.Parse(fmt.Sprintf(endpoint, "dummy"))
	if err != nil {
		return nil, err
	}

	gtg, _ := url.Parse(status.GTGPath)
	gtgUrl := v.ResolveReference(gtg)

	return &genericRWClient{client: client, rwEndpoint: endpoint, gtgEndpoint: gtgUrl.String()}, nil
}

func (rw *genericRWClient) GTG() error {
	req, err := http.NewRequest("GET", rw.gtgEndpoint, nil)
	if err != nil {
		log.WithError(err).WithField("healthEndpoint", rw.gtgEndpoint).Error("Error in creating GTG request for generic-rw-aurora")
		return err
	}
	resp, err := rw.client.Do(req)
	if err != nil {
		log.WithError(err).WithField("healthEndpoint", rw.gtgEndpoint).Error("Error in GTG request for generic-rw-aurora")
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.WithField("healthEndpoint", rw.gtgEndpoint).
			WithField("status", resp.StatusCode).
			Error("GTG for generic-rw-aurora returned a non-200 HTTP status")

		return fmt.Errorf("GTG %v returned a %v status code for generic-rw-aurora", rw.gtgEndpoint, resp.StatusCode)
	}

	return nil
}

func (rw *genericRWClient) Endpoint() string {
	return rw.rwEndpoint
}

func (rw *genericRWClient) GetAnnotations(ctx context.Context, uuid string) (AnnotationsBody, string, error) {
	draftsUrl := fmt.Sprintf(rw.rwEndpoint, uuid)
	req, err := http.NewRequest("GET", draftsUrl, nil)
	if err != nil {
		return AnnotationsBody{}, "", err
	}

	req.Header.Set("Accept", "application/json")

	resp, err := rw.client.Do(req.WithContext(ctx))
	if err != nil {
		return AnnotationsBody{}, "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return AnnotationsBody{}, "", ErrDraftNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return AnnotationsBody{}, "", fmt.Errorf("read from %v returned a %v status code", draftsUrl, resp.StatusCode)
	}

	hash := resp.Header.Get(DocumentHashHeader)
	ann := AnnotationsBody{}
	err = json.NewDecoder(resp.Body).Decode(&ann)

	return ann, hash, err
}

func (rw *genericRWClient) SaveAnnotations(ctx context.Context, uuid string, hash string, data AnnotationsBody) (AnnotationsBody, string, error) {
	draftsUrl := fmt.Sprintf(rw.rwEndpoint, uuid)
	body, err := json.Marshal(data)
	if err != nil {
		return AnnotationsBody{}, "", err
	}
	req, err := http.NewRequest("PUT", draftsUrl, bytes.NewReader(body))
	if err != nil {
		return AnnotationsBody{}, "", err
	}

	req.Header.Set(PreviousDocumentHashHeader, hash)
	req.Header.Set("Accept", "application/json")

	resp, err := rw.client.Do(req.WithContext(ctx))
	if err != nil {
		return AnnotationsBody{}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var ann AnnotationsBody
		// deal with inconsistency between draft-annotations-api and generic-rw-aurora in their responses from PUT requests
		if resp.ContentLength == 0 {
			ann = data
		} else {
			ann = AnnotationsBody{}
			err = json.NewDecoder(resp.Body).Decode(&ann)
		}

		return ann, resp.Header.Get(DocumentHashHeader), err
	}

	return AnnotationsBody{}, "", fmt.Errorf("write to %v returned a %v status code", draftsUrl, resp.StatusCode)
}
