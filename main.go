package main

import (
	"os"
	"time"

	"github.com/Financial-Times/annotations-publisher/draft"
	"github.com/Financial-Times/annotations-publisher/handler"
	"github.com/Financial-Times/annotations-publisher/health"
	"github.com/Financial-Times/annotations-publisher/notifier"
	"github.com/Financial-Times/annotations-publisher/server"
	"github.com/Financial-Times/annotations-publisher/service"
	"github.com/Financial-Times/cm-annotations-ontology/validator"
	"github.com/Financial-Times/go-ft-http/fthttp"
	l "github.com/Financial-Times/go-logger/v2"
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

		draftAnnotationsAPI := draft.NewAPI(*draftsEndpoint, *draftsGTGEndpoint, httpClient, logger)
		notifierAPI := notifier.NewAPI(*metadataNotifier, *metadataNotifierGTGEndpoint, httpClient, logger)

		publisher := service.NewPublisher(logger, draftAnnotationsAPI, notifierAPI)

		//TODO: remove this after testing
		os.Setenv("JSON_SCHEMAS_PATH", "./schemas")
		os.Setenv("JSON_SCHEMA_NAME", "annotations-pac.json;annotations-sv.json;annotations-draft.json")

		v := validator.NewSchemaValidator(logger)
		jv := v.GetJSONValidator()
		sh := v.GetSchemaHandler()

		h := handler.NewHandler(logger, publisher, jv, sh)

		healthService := health.NewHealthService(*appSystemCode, *appName, appDescription, notifierAPI, draftAnnotationsAPI)
		srv := server.New(port, apiYml, h, healthService, logger)
		srv.Start()
	}

	err := app.Run(os.Args)
	if err != nil {
		logger.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}
