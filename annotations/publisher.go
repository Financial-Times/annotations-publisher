package annotations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Financial-Times/annotations-publisher/health"
	tid "github.com/Financial-Times/transactionid-utils-go"
	log "github.com/sirupsen/logrus"
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
	Publish(ctx context.Context, uuid string, body map[string]interface{}) error
	PublishFromStore(ctx context.Context, uuid string) error
	SaveAndPublish(ctx context.Context, uuid string, hash string, body AnnotationsBody) error
}

type uppPublisher struct {
	client                     *http.Client
	originSystemID             string
	draftAnnotationsClient     AnnotationsClient
	publishedAnnotationsClient AnnotationsClient
	publishEndpoint            string
	publishAuth                string
	gtgEndpoint                string
}

// NewPublisher returns a new Publisher instance
func NewPublisher(originSystemID string, draftAnnotationsClient AnnotationsClient, publishedAnnotationsClient AnnotationsClient, publishEndpoint string, publishAuth string, gtgEndpoint string, timeout time.Duration) Publisher {
	log.WithField("endpoint", draftAnnotationsClient.Endpoint()).Info("draft annotations r/w endpoint")
	log.WithField("endpoint", publishedAnnotationsClient.Endpoint()).Info("published annotations r/w endpoint")
	log.WithField("endpoint", publishEndpoint).Info("publish endpoint")
	return &uppPublisher{client: &http.Client{Timeout: timeout}, originSystemID: originSystemID, draftAnnotationsClient: draftAnnotationsClient, publishedAnnotationsClient: publishedAnnotationsClient, publishEndpoint: publishEndpoint, publishAuth: publishAuth, gtgEndpoint: gtgEndpoint}
}

// Publish sends the annotations to UPP via the configured publishEndpoint. Requests contain X-Origin-System-Id and X-Request-Id and a User-Agent as provided.
func (a *uppPublisher) Publish(ctx context.Context, uuid string, body map[string]interface{}) error {
	txid, _ := tid.GetTransactionIDFromContext(ctx)
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
	req.Header.Add("X-Request-Id", txid)
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

func (a *uppPublisher) PublishFromStore(ctx context.Context, uuid string) error {
	txid, _ := tid.GetTransactionIDFromContext(ctx)
	mlog := log.WithField("transaction_id", txid)

	var draft AnnotationsBody
	var hash string
	var published AnnotationsBody
	var err error

	if draft, hash, err = a.draftAnnotationsClient.GetAnnotations(ctx, uuid); err == nil {
		published, hash, err = a.draftAnnotationsClient.SaveAnnotations(ctx, uuid, hash, draft)
	}

	if err != nil {
		mlog.WithError(err).Error("r/w to draft annotations failed")
		return err
	}

	_, _, err = a.publishedAnnotationsClient.SaveAnnotations(ctx, uuid, hash, published)
	if err != nil {
		mlog.WithError(err).Error("r/w to published annotations failed")
		return err
	}

	uppPublishBody := map[string]interface{}{
		"annotations": published.Annotations,
	}
	err = a.Publish(ctx, uuid, uppPublishBody)

	return err
}

func (a *uppPublisher) SaveAndPublish(ctx context.Context, uuid string, hash string, body AnnotationsBody) error {
	txid, _ := tid.GetTransactionIDFromContext(ctx)
	mlog := log.WithField("transaction_id", txid)
	_, _, err := a.draftAnnotationsClient.SaveAnnotations(ctx, uuid, hash, body)

	if err != nil {
		mlog.WithError(err).Error("write to draft annotations failed")
		return err
	}
	return  a.PublishFromStore(ctx, uuid)
}
