package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	publisher publisher
	jv        JSONValidator
	sh        SchemaHandler
}

type publisher interface {
	PublishFromStore(ctx context.Context, uuid string) error
	SaveAndPublish(ctx context.Context, uuid string, hash string, body map[string]interface{}) error
}

func NewHandler(l *logger.UPPLogger, p publisher, jv JSONValidator, sh SchemaHandler) *Handler {
	return &Handler{
		logger:    l,
		publisher: p,
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
		err = h.publisher.PublishFromStore(ctx, uuid)
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

	err = h.publisher.SaveAndPublish(ctx, uuid, hash, body)
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

func writeMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := make(map[string]interface{})
	resp["message"] = strings.ToUpper(msg[:1]) + msg[1:]

	enc := json.NewEncoder(w)
	_ = enc.Encode(&resp)
}
