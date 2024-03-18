package main

import (
	"net/http"
	"os"
	"time"

	"github.com/Financial-Times/annotations-publisher/annotations"
	"github.com/Financial-Times/annotations-publisher/health"
	"github.com/Financial-Times/annotations-publisher/resources"
	"github.com/Financial-Times/api-endpoint"
	"github.com/Financial-Times/go-ft-http/fthttp"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/husobee/vestigo"
	cli "github.com/jawher/mow.cli"
	"github.com/rcrowley/go-metrics"
)

const (
	appDescription = "PAC Annotations Publisher"
)

type jsonValidator interface {
	Validate(interface{}) error
}

type schemaHandler interface {
	ListSchemas(w http.ResponseWriter, r *http.Request)
	GetSchema(w http.ResponseWriter, r *http.Request)
}

func main() {
	app := cli.App("annotations-publisher", appDescription)

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

	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})

	draftsEndpoint := app.String(cli.StringOpt{
		Name:   "draft-annotations-rw-endpoint",
		Desc:   "Endpoint for saving/reading draft annotations",
		Value:  "http://draft-annotations-api:8080/drafts/content/%v/annotations",
		EnvVar: "DRAFT_ANNOTATIONS_RW_ENDPOINT",
	})

	annotationsEndpoint := app.String(cli.StringOpt{
		Name:   "annotations-publish-endpoint",
		Value:  "http://cms-metadata-notifier:8080/notify",
		Desc:   "Endpoint to publish annotations to UPP",
		EnvVar: "ANNOTATIONS_PUBLISH_ENDPOINT",
	})

	annotationsGTGEndpoint := app.String(cli.StringOpt{
		Name:   "annotations-publish-gtg-endpoint",
		Value:  "http://cms-metadata-notifier:8080/__gtg",
		Desc:   "GTG Endpoint for publishing annotations to UPP",
		EnvVar: "ANNOTATIONS_PUBLISH_GTG_ENDPOINT",
	})

	apiYml := app.String(cli.StringOpt{
		Name:   "api-yml",
		Value:  "./api.yml",
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
		Name:   "logLevel",
		Value:  "INFO",
		Desc:   "Logging level (DEBUG, INFO, WARN, ERROR)",
		EnvVar: "LOG_LEVEL",
	})

	enableValidation := app.Bool(cli.BoolOpt{
		Name:   "enableValidation",
		Value:  true,
		Desc:   "An option to allow or disallow validating annotations",
		EnvVar: "ENABLE_VALIDATION",
	})

	logger := logger.NewUPPLogger(*appName, *logLevel)
	logger.Infof("[Startup] %v is starting", *appSystemCode)

	app.Action = func() {
		logger.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)
		timeout, err := time.ParseDuration(*httpTimeout)
		if err != nil {
			logger.WithError(err).Fatal("Provided http timeout is not in the standard duration format.")
		}

		httpClient := fthttp.NewClient(timeout, "PAC", *appSystemCode)

		draftAnnotationsRW, err := annotations.NewAnnotationsClient(*draftsEndpoint, httpClient, logger)
		if err != nil {
			logger.WithError(err).Fatal("Failed to create new draft annotations writer.")
		}

		//var sh schemaHandler
		//var jv jsonValidator
		//
		if *enableValidation {
			//	v := validator.NewSchemaValidator(logger)
			//	sh = v.GetSchemaHandler()
			//	jv = v.GetJSONValidator()
		}

		publisher := annotations.NewPublisher(draftAnnotationsRW, *annotationsEndpoint, *annotationsGTGEndpoint, httpClient, logger)
		healthService := health.NewHealthService(*appSystemCode, *appName, appDescription, publisher, draftAnnotationsRW)

		serveEndpoints(*port, apiYml, publisher, healthService, timeout, logger)
	}

	err := app.Run(os.Args)
	if err != nil {
		logger.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func serveEndpoints(port string, apiYml *string, publisher annotations.Publisher, healthService *health.HealthService, timeout time.Duration, logger *logger.UPPLogger) {
	r := vestigo.NewRouter()
	r.Post("/drafts/content/:uuid/annotations/publish", resources.Publish(publisher, timeout, logger))

	var monitoringRouter http.Handler = r
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(logger.Logger, monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	r.Get("/__health", healthService.HealthCheckHandleFunc())
	r.Get(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))
	r.Get(status.BuildInfoPath, status.BuildInfoHandler)

	http.Handle("/", monitoringRouter)

	if apiYml != nil {
		apiEndpoint, err := api.NewAPIEndpointForFile(*apiYml)
		if err != nil {
			logger.WithError(err).WithField("file", *apiYml).Warn("Failed to serve the API Endpoint for this service. Please validate the Swagger YML and the file location")
		} else {
			r.Get(api.DefaultPath, apiEndpoint.ServeHTTP)
		}
	}

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logger.Fatalf("Unable to start: %v", err)
	}
}
