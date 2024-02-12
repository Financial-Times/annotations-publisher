package resources

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Financial-Times/annotations-publisher/annotations"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	log "github.com/sirupsen/logrus"
)

// Publish provides functionality to publish PAC annotations to UPP
func Publish(publisher annotations.Publisher, httpTimeOut time.Duration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		txid := tid.GetTransactionIDFromRequest(r)
		mlog := log.WithField(tid.TransactionIDKey, txid)
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
		log.WithFields(log.Fields{"transaction_id": txid, "uuid": uuid, "fromStore": fromStore}).Info("publish")

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			mlog.WithField("reason", err).Warn("error reading body")
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
			publishFromStore(ctx, publisher, uuid, w)
			return
		}

		var body map[string]interface{}
		err = json.Unmarshal(bodyBytes, &body)
		if err != nil {
			mlog.WithField("reason", err).Warn("failed to unmarshal publish body")
			writeMsg(w, http.StatusBadRequest, "Failed to process request json. Please provide a valid json request body")
			return
		}
		saveAndPublish(ctx, publisher, uuid, hash, w, body)
	}
}

func saveAndPublish(ctx context.Context, publisher annotations.Publisher, uuid string, hash string, w http.ResponseWriter, body map[string]interface{}) {
	txid, _ := tid.GetTransactionIDFromContext(ctx)
	mlog := log.WithField(tid.TransactionIDKey, txid)

	err := publisher.SaveAndPublish(ctx, uuid, hash, body)
	if err == annotations.ErrServiceTimeout {
		writeMsg(w, http.StatusGatewayTimeout, err.Error())
		return
	}
	if err == annotations.ErrDraftNotFound {
		writeMsg(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		mlog.WithField("reason", err).Error("failed to publish annotations to UPP")
		writeMsg(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeMsg(w, http.StatusAccepted, "Publish accepted")
}

func publishFromStore(ctx context.Context, publisher annotations.Publisher, uuid string, w http.ResponseWriter) {
	txid, _ := tid.GetTransactionIDFromContext(ctx)
	mlog := log.WithField(tid.TransactionIDKey, txid)

	err := publisher.PublishFromStore(ctx, uuid)
	if err == nil {
		writeMsg(w, http.StatusAccepted, "Publish accepted")
	} else if err == annotations.ErrServiceTimeout {
		writeMsg(w, http.StatusGatewayTimeout, err.Error())
	} else if err == annotations.ErrDraftNotFound {
		writeMsg(w, http.StatusNotFound, err.Error())
	} else {
		mlog.WithError(err).Error("Unable to publish annotations from store")
		writeMsg(w, http.StatusInternalServerError, "Unable to publish annotations from store")
	}
}

func writeMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := make(map[string]interface{})
	resp["message"] = strings.ToUpper(msg[:1]) + msg[1:]

	enc := json.NewEncoder(w)
	enc.Encode(&resp)
}
