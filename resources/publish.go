package resources

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Financial-Times/annotations-publisher/annotations"
	"github.com/Financial-Times/go-logger/v2"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	log "github.com/sirupsen/logrus"
)

type JSONValidator interface {
	Validate(interface{}) error
}

// Publish provides functionality to publish PAC annotations to UPP
func Publish(publisher annotations.Publisher, jv JSONValidator, httpTimeOut time.Duration, logger *logger.UPPLogger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		txid := tid.GetTransactionIDFromRequest(r)
		ctx, cancel := context.WithTimeout(tid.TransactionAwareContext(context.Background(), txid), httpTimeOut)
		defer cancel()

		origin := r.Header.Get(annotations.OriginSystemIDHeader)
		if origin == "" {
			writeMsg(w, http.StatusBadRequest, "Invalid request: X-Origin-System-Id header missing")
			return
		}
		ctx = context.WithValue(ctx, annotations.CtxOriginSystemIDKey(annotations.OriginSystemIDHeader), origin)

		uuid := vestigo.Param(r, "uuid")
		if uuid == "" {
			writeMsg(w, http.StatusBadRequest, "Please specify a valid uuid in the request")
			return
		}

		fromStore, _ := strconv.ParseBool(r.URL.Query().Get("fromStore"))
		hash := r.Header.Get(annotations.PreviousDocumentHashHeader)
		logger.WithFields(log.Fields{"transaction_id": txid, "uuid": uuid, "fromStore": fromStore}).Info("publish")

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			logger.WithTransactionID(txid).WithField("reason", err).Warn("error reading body")
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
			publishFromStore(ctx, logger, publisher, uuid, w)
			return
		}

		var body map[string]interface{}
		err = json.Unmarshal(bodyBytes, &body)
		if err != nil {
			logger.WithTransactionID(txid).WithField("reason", err).Warn("failed to unmarshal publish body")
			writeMsg(w, http.StatusBadRequest, "Failed to process request json. Please provide a valid json request body")
			return
		}

		err = jv.Validate(body)
		if err != nil {
			logger.WithTransactionID(txid).WithField("reason", err).Warn("failed to validate schema")
			writeMsg(w, http.StatusBadRequest, "Failed to validate json schema. Please provide a valid json request body")
			return
		}
		saveAndPublish(ctx, logger, publisher, uuid, hash, w, body)
	}
}

func saveAndPublish(ctx context.Context, logger *logger.UPPLogger, publisher annotations.Publisher, uuid string, hash string, w http.ResponseWriter, body map[string]interface{}) {
	txid, _ := tid.GetTransactionIDFromContext(ctx)

	err := publisher.SaveAndPublish(ctx, uuid, hash, body)
	if errors.Is(err, annotations.ErrServiceTimeout) {
		writeMsg(w, http.StatusGatewayTimeout, err.Error())
		return
	}
	if errors.Is(err, annotations.ErrDraftNotFound) {
		writeMsg(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		logger.WithTransactionID(txid).WithField("reason", err).Error("failed to publish annotations to UPP")
		writeMsg(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeMsg(w, http.StatusAccepted, "Publish accepted")
}

func publishFromStore(ctx context.Context, logger *logger.UPPLogger, publisher annotations.Publisher, uuid string, w http.ResponseWriter) {
	txid, _ := tid.GetTransactionIDFromContext(ctx)

	err := publisher.PublishFromStore(ctx, uuid)
	if err == nil {
		writeMsg(w, http.StatusAccepted, "Publish accepted")
	} else if errors.Is(err, annotations.ErrServiceTimeout) {
		writeMsg(w, http.StatusGatewayTimeout, err.Error())
	} else if errors.Is(err, annotations.ErrDraftNotFound) {
		writeMsg(w, http.StatusNotFound, err.Error())
	} else {
		logger.WithTransactionID(txid).WithError(err).Error("Unable to publish annotations from store")
		writeMsg(w, http.StatusInternalServerError, "Unable to publish annotations from store")
	}
}

func writeMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := make(map[string]interface{})
	resp["message"] = strings.ToUpper(msg[:1]) + msg[1:]

	enc := json.NewEncoder(w)
	_ = enc.Encode(&resp)
}
