package resources

import (
	"encoding/json"
	"net/http"

	"github.com/Financial-Times/annotations-publisher/annotations"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	log "github.com/sirupsen/logrus"
)

// Publish provides functionality to publish PAC annotations to UPP
func Publish(writer annotations.Writer, publisher annotations.Publisher) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := vestigo.Param(r, "uuid")
		if uuid == "" {
			writeMsg(w, http.StatusBadRequest, "Please specify a valid uuid in the request")
			return
		}

		body := make(map[string]interface{})

		dec := json.NewDecoder(r.Body)
		err := dec.Decode(&body)
		if err != nil {
			log.WithField("reason", err).Warn("Failed to decode publish body")
			writeMsg(w, http.StatusBadRequest, "Failed to process request json. Please provide a valid json request body")
			return
		}

		txid := tid.GetTransactionIDFromRequest(r)
		savedBody, err := writer.Write(uuid, txid, body)
		if err != nil {
			writeMsg(w, http.StatusInternalServerError, err.Error())
			return
		}

		err = publisher.Publish(uuid, txid, savedBody)
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
}

func writeMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := make(map[string]interface{})
	resp["message"] = msg

	enc := json.NewEncoder(w)
	enc.Encode(&resp)
}
