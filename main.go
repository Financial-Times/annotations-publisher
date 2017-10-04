package main

import (
	"net/http"
	"os"

	"github.com/Financial-Times/annotations-publisher/annotations"
	"github.com/Financial-Times/annotations-publisher/health"
	"github.com/Financial-Times/annotations-publisher/resources"
	api "github.com/Financial-Times/api-endpoint"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/husobee/vestigo"
	cli "github.com/jawher/mow.cli"
	metrics "github.com/rcrowley/go-metrics"
	log "github.com/sirupsen/logrus"
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

	saveEndpoint := app.String(cli.StringOpt{
		Name:   "annotations-save-endpoint",
		Desc:   "Endpoint to save annotations to PAC",
		EnvVar: "ANNOTATIONS_SAVE_ENDPOINT",
	})

	saveGTGEndpoint := app.String(cli.StringOpt{
		Name:   "annotations-save-gtg-endpoint",
		Desc:   "GTG Endpoint for the service which saves PAC annotations (usually draft-annotations-api)",
		EnvVar: "ANNOTATIONS_SAVE_GTG_ENDPOINT",
	})

	log.SetLevel(log.InfoLevel)
	log.Infof("[Startup] %v is starting", *appSystemCode)

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)

		client := &http.Client{}
		publisher := annotations.NewPublisher(client, *originSystemID, *annotationsEndpoint, *annotationsAuth, *annotationsGTGEndpoint)
		writer := annotations.NewWriter(client, *saveEndpoint, *saveGTGEndpoint)

		healthService := health.NewHealthService(*appSystemCode, *appName, appDescription, publisher, writer)

		serveEndpoints(*port, apiYml, writer, publisher, healthService)
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func serveEndpoints(port string, apiYml *string, writer annotations.Writer, publisher annotations.Publisher, healthService *health.HealthService) {
	r := vestigo.NewRouter()
	r.Post("/drafts/content/:uuid/annotations/publish", resources.Publish(writer, publisher))

	var monitoringRouter http.Handler = r
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), monitoringRouter)
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
