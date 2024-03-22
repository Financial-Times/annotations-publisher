package service

import (
	"context"
	"net"

	"github.com/Financial-Times/annotations-publisher/draft"
	"github.com/Financial-Times/annotations-publisher/notifier"
	"github.com/Financial-Times/go-logger/v2"
	tid "github.com/Financial-Times/transactionid-utils-go"
)

type Service struct {
	l           *logger.UPPLogger
	draftAPI    *draft.API
	notifierAPI *notifier.API
}

func NewPublisher(l *logger.UPPLogger, draftAPI *draft.API, notifierAPI *notifier.API) *Service {
	return &Service{l: l, draftAPI: draftAPI, notifierAPI: notifierAPI}
}

func (s *Service) SaveAndPublish(ctx context.Context, uuid string, hash string, body map[string]interface{}) error {
	txid, _ := tid.GetTransactionIDFromContext(ctx)
	_, _, err := s.draftAPI.SaveAnnotations(ctx, uuid, hash, body)

	if err != nil {
		if isTimeoutErr(err) {
			s.l.WithTransactionID(txid).WithError(err).Error("write to draft annotations timed out")
			return notifier.ErrServiceTimeout
		}

		s.l.WithError(err).Error("write to draft annotations failed")
		return err
	}
	return s.PublishFromStore(ctx, uuid)
}

func (s *Service) PublishFromStore(ctx context.Context, uuid string) error {
	txid, _ := tid.GetTransactionIDFromContext(ctx)

	var draft map[string]interface{}
	var hash string
	var published map[string]interface{}
	var err error

	if draft, hash, err = s.draftAPI.GetAnnotations(ctx, uuid); err == nil {
		published, _, err = s.draftAPI.SaveAnnotations(ctx, uuid, hash, draft)
	}

	if err != nil {
		if isTimeoutErr(err) {
			s.l.WithTransactionID(txid).WithError(err).Error("r/w to draft annotations timed out ")
			return notifier.ErrServiceTimeout
		}
		s.l.WithError(err).Error("r/w to draft annotations failed")
		return err
	}
	return s.notifierAPI.Publish(ctx, uuid, published)
}

func isTimeoutErr(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
