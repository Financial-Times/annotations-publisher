package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Financial-Times/annotations-publisher/external"
	"github.com/Financial-Times/go-logger/v2"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type JSONValidator interface {
	Validate(interface{}) error
}
type SchemaHandler interface {
	ListSchemas(w http.ResponseWriter, r *http.Request)
	GetSchema(w http.ResponseWriter, r *http.Request)
}
type Handler struct {
	logger    *logger.UPPLogger
	publisher *external.UppPublisher
	draftAPI  *external.RWClient
	jv        JSONValidator
	sh        SchemaHandler
}

func NewHandler(l *logger.UPPLogger, p *external.UppPublisher, draftAPI *external.RWClient, jv JSONValidator, sh SchemaHandler) *Handler {
	return &Handler{
		logger:    l,
		publisher: p,
		draftAPI:  draftAPI,
		jv:        jv,
		sh:        sh,
	}
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	txid := tid.GetTransactionIDFromRequest(r)
	ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), txid), 10*time.Second)
	defer cancel()

	origin := r.Header.Get(external.OriginSystemIDHeader)
	if origin == "" {
		writeMsg(w, http.StatusBadRequest, "Invalid request: X-Origin-System-Id header missing")
		return
	}
	ctx = context.WithValue(ctx, external.CtxOriginSystemIDKey(external.OriginSystemIDHeader), origin)

	vars := mux.Vars(r)
	uuid := vars["uuid"]
	if uuid == "" {
		writeMsg(w, http.StatusBadRequest, "Please specify a valid uuid in the request")
		return
	}

	fromStore, _ := strconv.ParseBool(r.URL.Query().Get("fromStore"))
	hash := r.Header.Get(external.PreviousDocumentHashHeader)
	h.logger.WithFields(log.Fields{"transaction_id": txid, "uuid": uuid, "fromStore": fromStore}).Info("publish")

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.WithTransactionID(txid).WithField("reason", err).Warn("error reading body")
		writeMsg(w, http.StatusBadRequest, "Failed to read request body. Please provide a valid json request body")
		return
	}

	if fromStore && len(bodyBytes) > 0 {
		writeMsg(w, http.StatusBadRequest, "A request body cannot be provided when fromStore=true")
		return
	}
	if !fromStore && len(bodyBytes) == 0 {
		writeMsg(w, http.StatusBadRequest, "Please provide a valid json request body")
		return
	}
	if fromStore {
		err = h.publishFromStore(ctx, uuid)
		if err == nil {
			writeMsg(w, http.StatusAccepted, "Publish accepted")
		} else if errors.Is(err, external.ErrServiceTimeout) {
			writeMsg(w, http.StatusGatewayTimeout, err.Error())
		} else if errors.Is(err, external.ErrDraftNotFound) {
			writeMsg(w, http.StatusNotFound, err.Error())
		} else {
			h.logger.WithTransactionID(txid).WithError(err).Error("Unable to publish annotations from store")
			writeMsg(w, http.StatusInternalServerError, "Unable to publish annotations from store")
		}
		return
	}

	var body map[string]interface{}
	err = json.Unmarshal(bodyBytes, &body)
	if err != nil {
		h.logger.WithTransactionID(txid).WithField("reason", err).Warn("failed to unmarshal publish body")
		writeMsg(w, http.StatusBadRequest, "Failed to process request json. Please provide a valid json request body")
		return
	}

	err = h.jv.Validate(body)
	if err != nil {
		h.logger.WithTransactionID(txid).WithField("reason", err).Warn("failed to validate schema")
		writeMsg(w, http.StatusBadRequest, "Failed to validate json schema. Please provide a valid json request body")
		return
	}

	err = h.saveAndPublish(ctx, uuid, hash, body)
	if errors.Is(err, external.ErrServiceTimeout) {
		writeMsg(w, http.StatusGatewayTimeout, err.Error())
		return
	}
	if errors.Is(err, external.ErrDraftNotFound) {
		writeMsg(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		h.logger.WithTransactionID(txid).WithField("reason", err).Error("failed to publish annotations to UPP")
		writeMsg(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeMsg(w, http.StatusAccepted, "Publish accepted")
}

func (h *Handler) saveAndPublish(ctx context.Context, uuid string, hash string, body map[string]interface{}) error {
	txid, _ := tid.GetTransactionIDFromContext(ctx)
	_, _, err := h.draftAPI.SaveAnnotations(ctx, uuid, hash, body)

	if err != nil {
		if isTimeoutErr(err) {
			h.logger.WithTransactionID(txid).WithError(err).Error("write to draft annotations timed out")
			return external.ErrServiceTimeout
		}

		h.logger.WithError(err).Error("write to draft annotations failed")
		return err
	}
	return h.publishFromStore(ctx, uuid)
}

func (h *Handler) publishFromStore(ctx context.Context, uuid string) error {
	txid, _ := tid.GetTransactionIDFromContext(ctx)

	var draft map[string]interface{}
	var hash string
	var published map[string]interface{}
	var err error

	if draft, hash, err = h.draftAPI.GetAnnotations(ctx, uuid); err == nil {
		published, _, err = h.draftAPI.SaveAnnotations(ctx, uuid, hash, draft)
	}

	if err != nil {
		if isTimeoutErr(err) {
			h.logger.WithTransactionID(txid).WithError(err).Error("r/w to draft annotations timed out ")
			return external.ErrServiceTimeout
		}
		h.logger.WithError(err).Error("r/w to draft annotations failed")
		return err

	}
	return h.publisher.Publish(ctx, uuid, published)
}

func writeMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := make(map[string]interface{})
	resp["message"] = strings.ToUpper(msg[:1]) + msg[1:]

	enc := json.NewEncoder(w)
	_ = enc.Encode(&resp)
}
func isTimeoutErr(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
