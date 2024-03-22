package external

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/Financial-Times/go-logger/v2"
	tid "github.com/Financial-Times/transactionid-utils-go"
)

type CtxOriginSystemIDKey string

const OriginSystemIDHeader string = "X-Origin-System-Id"

var (
	ErrDraftNotFound  = errors.New("draft was not found")
	ErrServiceTimeout = errors.New("downstream service timed out")
)

// Publisher provides an interface to publish annotations to UPP
//type Publisher interface {
//	health.ExternalService
//	Publish(ctx context.Context, uuid string, body map[string]interface{}) error
//	PublishFromStore(ctx context.Context, uuid string) error
//	SaveAndPublish(ctx context.Context, uuid string, hash string, body map[string]interface{}) error
//}

type UppPublisher struct {
	client          *http.Client
	publishEndpoint string
	gtgEndpoint     string
	logger          *logger.UPPLogger
}

// NewPublisher returns a new Publisher instance
func NewPublisher(publishEndpoint string, gtgEndpoint string, client *http.Client, logger *logger.UPPLogger) *UppPublisher {
	logger.WithField("endpoint", publishEndpoint).Info("publish endpoint")

	return &UppPublisher{
		client:          client,
		publishEndpoint: publishEndpoint,
		gtgEndpoint:     gtgEndpoint,
		logger:          logger,
	}
}

// Publish sends the annotations to UPP via the configured publishEndpoint. Requests contain X-Origin-System-Id and X-Request-Id and a User-Agent as provided.
func (a *UppPublisher) Publish(ctx context.Context, uuid string, body map[string]interface{}) error {
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

	req.Header.Add("X-Origin-System-Id", ctx.Value(CtxOriginSystemIDKey(OriginSystemIDHeader)).(string))
	req.Header.Add("Content-Type", "application/json")

	resp, err := a.client.Do(req.WithContext(ctx))
	if err != nil {
		if isTimeoutErr(err) {
			a.logger.WithTransactionID(txid).WithError(err).Error("annotations publish to upp timed out")
			return ErrServiceTimeout
		}
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("publish to %v returned a %v status code", a.publishEndpoint, resp.StatusCode)
	}

	return nil
}

// GTG performs a health check against the UPP cms-metadata-notifier service
func (a *UppPublisher) GTG() error {
	req, err := http.NewRequest("GET", a.gtgEndpoint, nil)
	if err != nil {
		a.logger.WithError(err).WithField("healthEndpoint", a.gtgEndpoint).Error("Error in creating GTG request for UPP cms-metadata-notifier service")
		return err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		a.logger.WithError(err).WithField("healthEndpoint", a.gtgEndpoint).Error("Error in GTG request for UPP cms-metadata-notifier service")
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.WithField("healthEndpoint", a.gtgEndpoint).
			WithField("status", resp.StatusCode).
			Error("GTG for UPP cms-metadata-notifier service returned a non-200 HTTP status")
		return fmt.Errorf("GTG %v returned a %v status code for UPP cms-metadata-notifier service", a.gtgEndpoint, resp.StatusCode)
	}

	return nil
}

// Endpoint returns the configured publish endpoint
func (a *UppPublisher) Endpoint() string {
	return a.publishEndpoint
}

//func (a *UppPublisher) PublishFromStore(ctx context.Context, published map[string]interface{}, uuid string) error {
//	txid, _ := tid.GetTransactionIDFromContext(ctx)
//
//	var draft map[string]interface{}
//	var hash string
//	var published map[string]interface{}
//	var err error
//
//	if draft, hash, err = a.draftAnnotationsClient.GetAnnotations(ctx, uuid); err == nil {
//		published, _, err = a.draftAnnotationsClient.SaveAnnotations(ctx, uuid, hash, draft)
//	}
//
//	if err != nil {
//		if isTimeoutErr(err) {
//			a.logger.WithTransactionID(txid).WithError(err).Error("r/w to draft annotations timed out ")
//			return ErrServiceTimeout
//		}
//		a.logger.WithError(err).Error("r/w to draft annotations failed")
//		return err
//	}
//
//	err = a.Publish(ctx, uuid, published)
//
//	return err
//}

//func (a *UppPublisher) SaveAndPublish(ctx context.Context, uuid string, hash string, body map[string]interface{}) error {
//	txid, _ := tid.GetTransactionIDFromContext(ctx)
//	_, _, err := a.draftAnnotationsClient.SaveAnnotations(ctx, uuid, hash, body)
//
//	if err != nil {
//		if isTimeoutErr(err) {
//			a.logger.WithTransactionID(txid).WithError(err).Error("write to draft annotations timed out")
//			return ErrServiceTimeout
//		}
//
//		a.logger.WithError(err).Error("write to draft annotations failed")
//		return err
//	}
//	return a.PublishFromStore(ctx, uuid)
//}

func isTimeoutErr(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
