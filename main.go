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

const appDescription = "PAC Annotations Publisher"

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

	writerEndpoint := app.String(cli.StringOpt{
		Name:   "published-annotations-rw-endpoint",
		Value:  "http://generic-rw-aurora:8080/published/content/%s/annotations",
		Desc:   "Endpoint for saving/reading published annotations",
		EnvVar: "PUBLISHED_ANNOTATIONS_RW_ENDPOINT",
	})

	annotationsEndpoint := app.String(cli.StringOpt{
		Name:   "annotations-publish-endpoint",
		Desc:   "Endpoint to publish annotations to UPP",
		EnvVar: "ANNOTATIONS_PUBLISH_ENDPOINT",
	})

	annotationsGTGEndpoint := app.String(cli.StringOpt{
		Name:   "annotations-publish-gtg-endpoint",
		Desc:   "GTG Endpoint for publishing annotations to UPP",
		EnvVar: "ANNOTATIONS_PUBLISH_GTG_ENDPOINT",
	})

	annotationsAuth := app.String(cli.StringOpt{
		Name:   "annotations-publish-auth",
		Desc:   "Basic auth to use for publishing annotations, in the format username:password",
		EnvVar: "ANNOTATIONS_PUBLISH_AUTH",
	})

	originSystemID := app.String(cli.StringOpt{
		Name:   "origin-system-id",
		Value:  "http://cmdb.ft.com/systems/pac",
		Desc:   "The system this publish originated from",
		EnvVar: "ORIGIN_SYSTEM_ID",
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

	log := logger.NewUPPInfoLogger(*appName)

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)
		timeout, err := time.ParseDuration(*httpTimeout)
		if err != nil {
			log.WithError(err).Fatal("Provided http timeout is not in the standard duration format.")
		}

		httpClient, err := fthttp.NewClient(
			fthttp.WithSysInfo("PAC", *appSystemCode),
			fthttp.WithTimeout(timeout),
		)
		if err != nil {
			log.WithError(err).Fatal("Failed to create new http client.")
		}

		draftAnnotationsRW, err := annotations.NewAnnotationsClient(*draftsEndpoint, httpClient, log)
		if err != nil {
			log.WithError(err).Fatal("Failed to create new draft annotations writer.")
		}

		publishedAnnotationsRW, err := annotations.NewAnnotationsClient(*writerEndpoint, httpClient, log)
		if err != nil {
			log.WithError(err).Fatal("Failed to create new published annotations writer.")
		}

		publisher := annotations.NewPublisher(*originSystemID, draftAnnotationsRW, publishedAnnotationsRW, *annotationsEndpoint, *annotationsAuth, *annotationsGTGEndpoint, httpClient, log)
		healthService := health.NewHealthService(*appSystemCode, *appName, appDescription, publisher, publishedAnnotationsRW, draftAnnotationsRW)

		serveEndpoints(*port, apiYml, publisher, healthService, timeout, log)
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func serveEndpoints(port string, apiYml *string, publisher annotations.Publisher, healthService *health.HealthService, timeout time.Duration, log *logger.UPPLogger) {
	r := vestigo.NewRouter()
	r.Post("/drafts/content/:uuid/annotations/publish", resources.Publish(publisher, timeout, log))

	var monitoringRouter http.Handler = r
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log, monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	r.Get("/__health", healthService.HealthCheckHandleFunc())
	r.Get(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))
	r.Get(status.BuildInfoPath, status.BuildInfoHandler)

	http.Handle("/", monitoringRouter)

	if apiYml != nil {
		apiEndpoint, err := api.NewAPIEndpointForFile(*apiYml)
		if err != nil {
			log.WithError(err).WithField("file", *apiYml).Warn("Failed to serve the API Endpoint for this service. Please validate the Swagger YML and the file location")
		} else {
			r.Get(api.DefaultPath, apiEndpoint.ServeHTTP)
		}
	}

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Unable to start: %v", err)
	}
}
