package external

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Financial-Times/go-logger/v2"
)

const (
	DocumentHashHeader         = "Document-Hash"
	PreviousDocumentHashHeader = "Previous-Document-Hash"
)

var ErrMissingOriginHeader = errors.New("X-Origin-System-Id header not found in context")

type RWClient struct {
	client      *http.Client
	rwEndpoint  string
	gtgEndpoint string
	logger      *logger.UPPLogger
}

func NewAnnotationsClient(endpoint string, gtgEndpoint string, client *http.Client, logger *logger.UPPLogger) *RWClient {
	r := &RWClient{client: client, rwEndpoint: endpoint, gtgEndpoint: gtgEndpoint, logger: logger}
	logger.WithField("endpoint", r.Endpoint()).Info("draft annotations r/w endpoint")

	return r
}

func (rw *RWClient) GTG() error {
	req, err := http.NewRequest("GET", rw.gtgEndpoint, nil)
	if err != nil {
		rw.logger.WithError(err).WithField("healthEndpoint", rw.gtgEndpoint).Error("Error in creating GTG request for generic-rw-aurora")
		return err
	}
	resp, err := rw.client.Do(req)
	if err != nil {
		rw.logger.WithError(err).WithField("healthEndpoint", rw.gtgEndpoint).Error("Error in GTG request for generic-rw-aurora")
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rw.logger.WithField("healthEndpoint", rw.gtgEndpoint).
			WithField("status", resp.StatusCode).
			Error("GTG for generic-rw-aurora returned a non-200 HTTP status")

		return fmt.Errorf("GTG %v returned a %v status code for generic-rw-aurora", rw.gtgEndpoint, resp.StatusCode)
	}

	return nil
}

func (rw *RWClient) Endpoint() string {
	return rw.rwEndpoint
}

func (rw *RWClient) GetAnnotations(ctx context.Context, uuid string) (map[string]interface{}, string, error) {
	draftsURL := fmt.Sprintf(rw.rwEndpoint, uuid)
	req, err := http.NewRequest("GET", draftsURL, nil)
	if err != nil {
		return map[string]interface{}{}, "", err
	}

	q := req.URL.Query()
	//we send this parameter to draft-annotations-api to get isClassifiedBy predicate as hasBrand
	q.Add("sendHasBrand", strconv.FormatBool(true))
	req.URL.RawQuery = q.Encode()

	headerToAssert := ctx.Value(CtxOriginSystemIDKey(OriginSystemIDHeader))
	originHeader, ok := headerToAssert.(string)
	if !ok {
		return map[string]interface{}{}, "", ErrMissingOriginHeader
	}
	req.Header.Set("X-Origin-System-Id", originHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := rw.client.Do(req.WithContext(ctx))
	if err != nil {
		return map[string]interface{}{}, "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return map[string]interface{}{}, "", ErrDraftNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{}, "", fmt.Errorf("read from %v returned a %v status code", draftsURL, resp.StatusCode)
	}

	hash := resp.Header.Get(DocumentHashHeader)
	var ann map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&ann)

	return ann, hash, err
}

func (rw *RWClient) SaveAnnotations(ctx context.Context, uuid string, hash string, data map[string]interface{}) (map[string]interface{}, string, error) {
	draftsURL := fmt.Sprintf(rw.rwEndpoint, uuid)
	body, err := json.Marshal(data)
	if err != nil {
		return map[string]interface{}{}, "", err
	}
	req, err := http.NewRequest("PUT", draftsURL, bytes.NewReader(body))
	if err != nil {
		return map[string]interface{}{}, "", err
	}

	headerToAssert := ctx.Value(CtxOriginSystemIDKey(OriginSystemIDHeader))
	originHeader, ok := headerToAssert.(string)
	if !ok {
		return map[string]interface{}{}, "", ErrMissingOriginHeader
	}
	req.Header.Set("X-Origin-System-Id", originHeader)
	req.Header.Set(PreviousDocumentHashHeader, hash)
	req.Header.Set("Accept", "application/json")

	resp, err := rw.client.Do(req.WithContext(ctx))
	if err != nil {
		return map[string]interface{}{}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var ann map[string]interface{}
		// deal with inconsistency between draft-annotations-api and generic-rw-aurora in their responses from PUT requests
		if resp.ContentLength == 0 {
			ann = data
		} else {
			err = json.NewDecoder(resp.Body).Decode(&ann)
		}

		return ann, resp.Header.Get(DocumentHashHeader), err
	}

	return map[string]interface{}{}, "", fmt.Errorf("write to %v returned a %v status code", draftsURL, resp.StatusCode)
}
