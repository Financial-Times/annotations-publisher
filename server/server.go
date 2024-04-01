package server

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Financial-Times/api-endpoint"
	l "github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/service-status-go/gtg"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/Financial-Times/upp-go-sdk/pkg/server"
	"github.com/gorilla/mux"
)

type startCloser interface {
	Start() error
	Close() error
}
type healthChecker interface {
	GTG() gtg.Status
	HealthCheckHandleFunc() func(w http.ResponseWriter, r *http.Request)
}

type handler interface {
	Publish(w http.ResponseWriter, r *http.Request)
	Validate(w http.ResponseWriter, r *http.Request)
	ListSchemas(w http.ResponseWriter, r *http.Request)
	GetSchema(w http.ResponseWriter, r *http.Request)
}

type Server struct {
	port          *int
	apiYml        *string
	h             handler
	healthService healthChecker
	logger        *l.UPPLogger
}

func New(port *int, apiYml *string, h handler, hs healthChecker, logger *l.UPPLogger) *Server {
	return &Server{
		port:          port,
		apiYml:        apiYml,
		h:             h,
		healthService: hs,
		logger:        logger,
	}
}
func (s *Server) Start() {
	shutdown := s.listenForShutdownSignal()
	router := s.registerEndpoints()
	srv := s.startServer(router)
	s.waitForShutdownSignal(srv, shutdown)
}

func (s *Server) registerEndpoints() func(r *mux.Router) {
	r := func(r *mux.Router) {
		r.HandleFunc("/drafts/content/{uuid}/annotations/publish", s.h.Publish).Methods(http.MethodPost)
		r.HandleFunc("/validate", s.h.Validate).Methods(http.MethodPost)
		r.HandleFunc("/schemas", s.h.ListSchemas).Methods(http.MethodGet)
		r.HandleFunc("/schemas/{schemaName}", s.h.GetSchema).Methods(http.MethodGet)

		r.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)
		r.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(s.healthService.GTG))

		if s.apiYml != nil {
			apiEndpoint, err := api.NewAPIEndpointForFile(*s.apiYml)
			if err != nil {
				s.logger.WithError(err).WithField("file", *s.apiYml).Warn("Failed to serve the API Endpoint for this service. Please validate the Swagger YML and the file location")
			} else {
				r.Handle(api.DefaultPath, apiEndpoint)
			}
		}
	}
	return r
}

func (s *Server) startServer(router func(r *mux.Router)) *server.Server {
	srv := server.New(
		router,
		server.WithTIDAwareRequestLogging(s.logger),
		server.WithHealthCheckHander(s.healthService.HealthCheckHandleFunc()),
		server.WithCustomAppPort(*s.port),
	)

	go func() {
		if err := srv.Start(); err != nil {
			s.logger.Infof("HTTP server closing with message: %v", err)
		}
	}()
	return srv
}

func (s *Server) listenForShutdownSignal() chan bool {
	ch := make(chan os.Signal, 1)
	shutdown := make(chan bool, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		shutdown <- true
	}()
	return shutdown
}

func (s *Server) waitForShutdownSignal(srv startCloser, shutdown chan bool) {
	<-shutdown
	s.logger.Info("HTTP server shutting down")
	if err := srv.Close(); err != nil {
		s.logger.WithError(err).Error("failed to close the server")
	}
}
