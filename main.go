package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Financial-Times/annotations-publisher/external"
	"github.com/Financial-Times/annotations-publisher/handler"
	"github.com/Financial-Times/annotations-publisher/health"
	"github.com/Financial-Times/annotations-publisher/service"
	"github.com/Financial-Times/api-endpoint"
	"github.com/Financial-Times/cm-annotations-ontology/validator"
	"github.com/Financial-Times/go-ft-http/fthttp"
	l "github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/service-status-go/gtg"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/Financial-Times/upp-go-sdk/pkg/server"
	"github.com/gorilla/mux"
	cli "github.com/jawher/mow.cli"
)

const (
	appDescription = "PAC Annotations Publisher"
	appCode        = "annotations-publisher"
)

func main() {
	app := cli.App(appCode, appDescription)

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "annotations-publisher",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})

	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "annotations-publisher",
		Desc:   "Application name",
		EnvVar: "APP_NAME",
	})

	port := app.Int(cli.IntOpt{
		Name:   "port",
		Value:  8080,
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})

	draftsEndpoint := app.String(cli.StringOpt{
		Name:   "draft-annotations-rw-endpoint",
		Desc:   "Endpoint for saving/reading draft annotations",
		Value:  "http://draft-annotations-api:8080/drafts/content/%v/annotations",
		EnvVar: "DRAFT_ANNOTATIONS_RW_ENDPOINT",
	})

	draftsGTGEndpoint := app.String(cli.StringOpt{
		Name:   "draft-annotations-rw-gtg-endpoint",
		Desc:   "GTG Endpoint for saving/reading draft annotations",
		Value:  "http://draft-annotations-api:8080/__gtg",
		EnvVar: "DRAFT_ANNOTATIONS_RW_GTG_ENDPOINT",
	})

	metadataNotifier := app.String(cli.StringOpt{
		Name:   "metadata-notifier-endpoint",
		Value:  "http://cms-metadata-notifier:8080/notify",
		Desc:   "Endpoint to publish annotations to UPP",
		EnvVar: "METADATA_NOTIFIER_ENDPOINT",
	})

	metadataNotifierGTGEndpoint := app.String(cli.StringOpt{
		Name:   "metadata-notifier-gtg-endpoint",
		Value:  "http://cms-metadata-notifier:8080/__gtg",
		Desc:   "GTG Endpoint for publishing annotations to UPP",
		EnvVar: "METADATA_NOTIFIER_GTG_ENDPOINT",
	})

	apiYml := app.String(cli.StringOpt{
		Name:   "api-yml",
		Value:  "api/api.yml",
		Desc:   "Location of the API Swagger YML file.",
		EnvVar: "API_YML",
	})

	httpTimeout := app.String(cli.StringOpt{
		Name:   "http-timeout",
		Value:  "8s",
		Desc:   "http client timeout in seconds",
		EnvVar: "HTTP_CLIENT_TIMEOUT",
	})

	logLevel := app.String(cli.StringOpt{
		Name:   "log-Level",
		Value:  "INFO",
		Desc:   "Logging level (DEBUG, INFO, WARN, ERROR)",
		EnvVar: "LOG_LEVEL",
	})

	logger := l.NewUPPLogger(*appName, *logLevel)
	logger.Infof("[Startup] %v is starting", *appSystemCode)

	app.Action = func() {
		logger.Infof("System code: %s, App Name: %s, Port: %d", *appSystemCode, *appName, *port)
		timeout, err := time.ParseDuration(*httpTimeout)
		if err != nil {
			logger.WithError(err).Fatal("Provided http timeout is not in the standard duration format.")
		}

		httpClient := fthttp.NewClient(timeout, "PAC", *appSystemCode)

		draftAnnotationsAPI := external.NewAnnotationsClient(*draftsEndpoint, *draftsGTGEndpoint, httpClient, logger)
		notifierAPI := external.NewPublisher(*metadataNotifier, *metadataNotifierGTGEndpoint, httpClient, logger)

		publisher := service.NewPublisher(logger, draftAnnotationsAPI, notifierAPI)

		//TODO: remove this after testing
		os.Setenv("JSON_SCHEMAS_PATH", "./schemas")
		os.Setenv("JSON_SCHEMA_NAME", "annotations-pac.json;annotations-sv.json;annotations-draft.json")

		v := validator.NewSchemaValidator(logger)
		jv := v.GetJSONValidator()
		sh := v.GetSchemaHandler()
		h := handler.NewHandler(logger, publisher, jv, sh)
		healthService := health.NewHealthService(*appSystemCode, *appName, appDescription, notifierAPI, draftAnnotationsAPI)

		serveEndpoints(*port, apiYml, h, healthService, timeout, logger)
	}

	err := app.Run(os.Args)
	if err != nil {
		logger.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

//	type Publisher interface {
//		health.ExternalService
//		Publish(ctx context.Context, uuid string, body map[string]interface{}) error
//		PublishFromStore(ctx context.Context, uuid string) error
//		SaveAndPublish(ctx context.Context, uuid string, hash string, body map[string]interface{}) error
//	}

type healthChecker interface {
	GTG() gtg.Status
	HealthCheckHandleFunc() func(w http.ResponseWriter, r *http.Request)
}

func serveEndpoints(port int, apiYml *string, h *handler.Handler, healthService healthChecker, timeout time.Duration, logger *l.UPPLogger) {
	srv := server.New(
		func(r *mux.Router) {
			r.HandleFunc("/drafts/content/{uuid}/annotations/publish", h.Publish).Methods(http.MethodPost)
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
