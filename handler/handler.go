package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Financial-Times/annotations-publisher/draft"
	"github.com/Financial-Times/annotations-publisher/notifier"
	"github.com/Financial-Times/go-logger/v2"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type jsonValidator interface {
	Validate(interface{}) error
}
type schemaHandler interface {
	ListSchemas(w http.ResponseWriter, r *http.Request)
	GetSchema(w http.ResponseWriter, r *http.Request)
}
type publisher interface {
	PublishFromStore(ctx context.Context, uuid string) error
	SaveAndPublish(ctx context.Context, uuid string, hash string, body map[string]interface{}) error
}

type Handler struct {
	logger    *logger.UPPLogger
	publisher publisher
	jv        jsonValidator
	sh        schemaHandler
}

func NewHandler(l *logger.UPPLogger, p publisher, jv jsonValidator, sh schemaHandler) *Handler {
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

	origin := r.Header.Get(notifier.OriginSystemIDHeader)
	if origin == "" {
		respondWithMessage(w, http.StatusBadRequest, "Invalid request: X-Origin-System-Id header missing")
		return
	}
	ctx = context.WithValue(ctx, notifier.CtxOriginSystemIDKey(notifier.OriginSystemIDHeader), origin)

	vars := mux.Vars(r)
	uuid := vars["uuid"]
	if uuid == "" {
		respondWithMessage(w, http.StatusBadRequest, "Please specify a valid uuid in the request")
		return
	}

	fromStore, _ := strconv.ParseBool(r.URL.Query().Get("fromStore"))
	hash := r.Header.Get(draft.PreviousDocumentHashHeader)
	h.logger.WithFields(log.Fields{"transaction_id": txid, "uuid": uuid, "fromStore": fromStore}).Info("publish")

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.WithTransactionID(txid).WithField("reason", err).Warn("error reading body")
		respondWithMessage(w, http.StatusBadRequest, "Failed to read request body. Please provide a valid json request body")
		return
	}

	if fromStore && len(bodyBytes) > 0 {
		respondWithMessage(w, http.StatusBadRequest, "A request body cannot be provided when fromStore=true")
		return
	}
	if !fromStore && len(bodyBytes) == 0 {
		respondWithMessage(w, http.StatusBadRequest, "Please provide a valid json request body")
		return
	}
	if fromStore {
		err = h.publisher.PublishFromStore(ctx, uuid)
		if err == nil {
			respondWithMessage(w, http.StatusAccepted, "Publish accepted")
		} else if errors.Is(err, notifier.ErrServiceTimeout) {
			respondWithMessage(w, http.StatusGatewayTimeout, err.Error())
		} else if errors.Is(err, notifier.ErrDraftNotFound) {
			respondWithMessage(w, http.StatusNotFound, err.Error())
		} else {
			h.logger.WithTransactionID(txid).WithError(err).Error("Unable to publish annotations from store")
			respondWithMessage(w, http.StatusInternalServerError, "Unable to publish annotations from store")
		}
		return
	}

	var body map[string]interface{}
	err = json.Unmarshal(bodyBytes, &body)
	if err != nil {
		h.logger.WithTransactionID(txid).WithField("reason", err).Warn("failed to unmarshal publish body")
		respondWithMessage(w, http.StatusBadRequest, "Failed to process request json. Please provide a valid json request body")
		return
	}

	err = h.jv.Validate(body)
	if err != nil {
		h.logger.WithTransactionID(txid).WithField("reason", err).Warn("failed to validate schema")
		respondWithMessage(w, http.StatusBadRequest, "Failed to validate json schema. Please provide a valid json request body")
		return
	}

	err = h.publisher.SaveAndPublish(ctx, uuid, hash, body)
	if errors.Is(err, notifier.ErrServiceTimeout) {
		respondWithMessage(w, http.StatusGatewayTimeout, err.Error())
		return
	}
	if errors.Is(err, notifier.ErrDraftNotFound) {
		respondWithMessage(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		h.logger.WithTransactionID(txid).WithField("reason", err).Error("failed to publish annotations to UPP")
		respondWithMessage(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	respondWithMessage(w, http.StatusAccepted, "Publish accepted")
}

func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	txid := tid.GetTransactionIDFromRequest(r)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		msg := "Failed to read request body"
		h.logger.WithTransactionID(txid).WithError(err).Error(msg)
		respondWithMessage(w, http.StatusBadRequest, msg)
		return
	}

	var body map[string]interface{}
	err = json.Unmarshal(b, &body)
	if err != nil {
		msg := fmt.Sprintf("Failed to unmarshal request body: %v", err)
		log.WithError(err).Error(msg)
		respondWithMessage(w, http.StatusBadRequest, msg)
		return
	}

	err = h.jv.Validate(body)
	if err != nil {
		msg := fmt.Sprintf("Failed to validate request body: %v", err)
		log.WithError(err).Error(msg)
		respondWithMessage(w, http.StatusBadRequest, msg)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func respondWithMessage(w http.ResponseWriter, statusCode int, messageTxt string) {
	type messageResponse struct {
		Message string `json:"message"`
	}

	mess := &messageResponse{Message: messageTxt}

	resp, _ := json.Marshal(mess)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(resp)
}

func (h *Handler) ListSchemas(w http.ResponseWriter, r *http.Request) {
	h.sh.ListSchemas(w, r)
}

func (h *Handler) GetSchema(w http.ResponseWriter, r *http.Request) {
	h.sh.GetSchema(w, r)
}
