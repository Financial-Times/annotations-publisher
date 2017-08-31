package resources

import (
	"encoding/json"
	"net/http"

	"github.com/Financial-Times/annotations-publisher/annotations"
	"github.com/husobee/vestigo"
	log "github.com/sirupsen/logrus"
)

func Publish(publisher annotations.Publisher) func(w http.ResponseWriter, r *http.Request) {
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
			log.WithError(err).Error("Failed to decode publish body")
			writeMsg(w, http.StatusBadRequest, "Failed to process request json. Please provide a valid json request body")
			return
		}

		err = publisher.Publish(uuid, body)
		if err != nil {
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
