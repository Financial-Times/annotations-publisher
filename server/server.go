package server

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Financial-Times/annotations-publisher/handler"
	"github.com/Financial-Times/api-endpoint"
	l "github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/service-status-go/gtg"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/Financial-Times/upp-go-sdk/pkg/server"
	"github.com/gorilla/mux"
)

type healthChecker interface {
	GTG() gtg.Status
	HealthCheckHandleFunc() func(w http.ResponseWriter, r *http.Request)
}

func Start(port int, apiYml *string, h *handler.Handler, healthService healthChecker, logger *l.UPPLogger) {
	srv := server.New(
		func(r *mux.Router) {
			r.HandleFunc("/drafts/content/{uuid}/annotations/publish", h.Publish).Methods(http.MethodPost)
			r.HandleFunc("/validate", h.Validate).Methods(http.MethodPost)
			r.HandleFunc("/schemas", h.ListSchemas).Methods(http.MethodGet)
			r.HandleFunc("/schemas/{schemaName}", h.GetSchema).Methods(http.MethodGet)

			r.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)
			r.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))

			if apiYml != nil {
				apiEndpoint, err := api.NewAPIEndpointForFile(*apiYml)
				if err != nil {
					logger.WithError(err).WithField("file", *apiYml).Warn("Failed to serve the API Endpoint for this service. Please validate the Swagger YML and the file location")
				} else {
					r.Handle(api.DefaultPath, apiEndpoint)
				}
			}
		},
		server.WithTIDAwareRequestLogging(logger),
		server.WithHealthCheckHander(healthService.HealthCheckHandleFunc()),
		server.WithCustomAppPort(port),
	)

	go func() {
		if err := srv.Start(); err != nil {
			logger.Infof("HTTP server closing with message: %v", err)
		}
	}()

	defer func() {
		logger.Info("HTTP server shutting down")
		if err := srv.Close(); err != nil {
			logger.WithError(err).Error("failed to close the server")
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
