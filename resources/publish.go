package resources

import (
	"net/http"

	"github.com/Financial-Times/annotations-publisher/annotations"
)

func Publish(publisher annotations.Publisher) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

	}
}
