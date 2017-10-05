package annotations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Writer saves annotations via the configured PUT endpoint
type Writer interface {
	Write(uuid string, tid string, body map[string]interface{}) (map[string]interface{}, error)
	GTG() error
	Endpoint() string
}

type pacWriter struct {
	client          *http.Client
	saveEndpoint    string
	saveGTGEndpoint string
}

// NewWriter returns a new PAC annotations writer
func NewWriter(client *http.Client, saveEndpoint string, saveGTGEndpoint string) Writer {
	return &pacWriter{client: client, saveEndpoint: saveEndpoint, saveGTGEndpoint: saveGTGEndpoint}
}

func (p *pacWriter) Write(uuid string, tid string, body map[string]interface{}) (map[string]interface{}, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf(p.saveEndpoint, uuid)
	req, err := http.NewRequest("PUT", uri, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", "PAC annotations-publisher")
	req.Header.Add("X-Request-Id", tid)
	req.Header.Add("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Save to %v returned a %v status code", uri, resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	respBody := make(map[string]interface{})
	err = dec.Decode(&respBody)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

func (p *pacWriter) GTG() error {
	req, err := http.NewRequest("GET", p.saveGTGEndpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", "PAC annotations-publisher")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GTG %v returned a %v status code", p.saveGTGEndpoint, resp.StatusCode)
	}

	return nil
}

func (p *pacWriter) Endpoint() string {
	return p.saveEndpoint
}
