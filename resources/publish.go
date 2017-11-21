package resources

import (
	"context"
	"encoding/json"
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

		uuid := vestigo.Param(r, "uuid")
		if uuid == "" {
			writeMsg(w, http.StatusBadRequest, "Please specify a valid uuid in the request")
			return
		}

		fromStore, _ := strconv.ParseBool(r.URL.Query().Get("fromStore"))
		log.WithFields(log.Fields{"transaction_id": txid, "uuid": uuid, "fromStore": fromStore}).Info("publish")

		if fromStore {
			publishFromStore(ctx, publisher, uuid, w)
		} else {
			saveAndPublish(ctx, publisher, uuid, w, r)
		}
	}
}

func saveAndPublish(ctx context.Context, publisher annotations.Publisher, uuid string, w http.ResponseWriter, r *http.Request) {
	body := make(map[string]interface{})

	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&body)
	if err != nil {
		log.WithField("reason", err).Warn("Failed to decode publish body")
		writeMsg(w, http.StatusBadRequest, "Failed to process request json. Please provide a valid json request body")
		return
	}

	txid, _ := tid.GetTransactionIDFromContext(ctx)
	err = publisher.Publish(uuid, txid, body)
	if err == annotations.ErrInvalidAuthentication { // the service config needs to be updated for this to work
		writeMsg(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		log.WithField("reason", err).Warn("Failed to publish annotations to UPP")
		writeMsg(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeMsg(w, http.StatusAccepted, "Publish accepted")
}

func publishFromStore(ctx context.Context, publisher annotations.Publisher, uuid string, w http.ResponseWriter) {
	draft, err := publisher.GetDraft(ctx, uuid)
	if err == annotations.ErrDraftNotFound {
		writeMsg(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		log.WithError(err).Error("unable to read draft annotations")
		writeMsg(w, http.StatusInternalServerError, "Unable to read draft annotations")
		return
	}

	log.Info(draft)
	writeMsg(w, http.StatusNotImplemented, "Not implemented")
}

func writeMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := make(map[string]interface{})
	resp["message"] = strings.ToUpper(msg[:1]) + msg[1:]

	enc := json.NewEncoder(w)
	enc.Encode(&resp)
}
