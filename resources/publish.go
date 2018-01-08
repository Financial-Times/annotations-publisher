package resources

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/Financial-Times/annotations-publisher/annotations"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	log "github.com/sirupsen/logrus"
)

// Publish provides functionality to publish PAC annotations to UPP
func Publish(publisher annotations.Publisher) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		txid := tid.GetTransactionIDFromRequest(r)
		ctx := tid.TransactionAwareContext(r.Context(), txid)
		mlog := log.WithField(tid.TransactionIDKey, txid)

		uuid := vestigo.Param(r, "uuid")
		if uuid == "" {
			writeMsg(w, http.StatusBadRequest, "Please specify a valid uuid in the request")
			return
		}

		fromStore, _ := strconv.ParseBool(r.URL.Query().Get("fromStore"))
		hash := r.Header.Get(annotations.PreviousDocumentHashHeader)
		log.WithFields(log.Fields{"transaction_id": txid, "uuid": uuid, "fromStore": fromStore}).Info("publish")

		var body annotations.AnnotationsBody

		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			mlog.WithField("reason", err).Warn("Error reading body")
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
		} else {
			json.Unmarshal(bodyBytes, &body)
			if err != nil || len(body.Annotations) == 0 {
				mlog.WithField("reason", err).Warn("Failed to unmarshall publish body")
				writeMsg(w, http.StatusBadRequest, "Failed to process request json. Please provide a valid json request body")
				return
			}
			saveAndPublish(ctx, publisher, uuid, hash, w, body)
		}
	}
}

func saveAndPublish(ctx context.Context, publisher annotations.Publisher, uuid string, hash string, w http.ResponseWriter, body annotations.AnnotationsBody) {
	txid, _ := tid.GetTransactionIDFromContext(ctx)
	mlog := log.WithField(tid.TransactionIDKey, txid)

	err := publisher.SaveAndPublish(ctx, uuid, hash, body)
	if err == annotations.ErrInvalidAuthentication { // the service config needs to be updated for this to work
		writeMsg(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err == annotations.ErrDraftNotFound {
		writeMsg(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		mlog.WithField("reason", err).Error("Failed to publish annotations to UPP")
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
	} else if err == annotations.ErrDraftNotFound {
		writeMsg(w, http.StatusNotFound, err.Error())
	} else {
		mlog.WithError(err).Error("unable to publish annotations from store")
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
