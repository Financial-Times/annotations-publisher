# Annotations Publisher
[![Circle CI](https://circleci.com/gh/Financial-Times/annotations-publisher/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/annotations-publisher/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/annotations-publisher)](https://goreportcard.com/report/github.com/Financial-Times/annotations-publisher) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/annotations-publisher/badge.svg)](https://coveralls.io/github/Financial-Times/annotations-publisher)

## Introduction

The Annotations Publisher is a microservice that Publishes annotations from TagMe to UPP.

## Installation

Download the source code, dependencies and test dependencies:

```
go get -u github.com/kardianos/govendor
mkdir $GOPATH/src/github.com/Financial-Times/annotations-publisher
cd $GOPATH/src/github.com/Financial-Times
git clone https://github.com/Financial-Times/annotations-publisher.git
cd annotations-publisher && govendor sync
go build .
```

## Running locally

1. Run the tests and install the binary:

```
govendor sync
govendor test -v -race +local
go install
```

2. Run the binary (using the `help` flag to see the available optional arguments):

```
$GOPATH/bin/annotations-publisher [--help]

PAC Annotations Publisher

Options:
  --app-system-code="annotations-publisher"             System Code of the application ($APP_SYSTEM_CODE)
  --app-name="annotations-publisher"                    Application name ($APP_NAME)
  --port="8080"                                         Port to listen on ($APP_PORT)
  --draft-annotations-rw-endpoint=""                    Endpoint for saving/reading draft annotations ($DRAFT_ANNOTATIONS_RW_ENDPOINT)
  --annotations-publish-endpoint=""                     Endpoint to publish annotations to UPP ($ANNOTATIONS_PUBLISH_ENDPOINT)
  --annotations-publish-gtg-endpoint=""                 GTG Endpoint for publishing annotations to UPP ($ANNOTATIONS_PUBLISH_GTG_ENDPOINT)
  --annotations-publish-auth=""                         Basic auth to use for publishing annotations, in the format username:password ($ANNOTATIONS_PUBLISH_AUTH)
  --origin-system-id="http://cmdb.ft.com/systems/pac"   The system this publish originated from ($ORIGIN_SYSTEM_ID)
  --api-yml="./api.yml"                                 Location of the API Swagger YML file. ($API_YML)
  --http-timeout="8s"                                     http client timeout in seconds
```

3. Test:

```
curl http://localhost:8080/__health | jq
```

## Build and deployment

* Built by Docker Hub on merge to master: [coco/annotations-publisher](https://hub.docker.com/r/coco/annotations-publisher/)
* CI provided by CircleCI: [annotations-publisher](https://circleci.com/gh/Financial-Times/annotations-publisher)

## Service endpoints

For a full description of API endpoints for the service, please see the [Open API specification](./api/api.yml).

### POST
####Publish with Body####

This endpoint saves the provided annotations to the PAC database (within the draft and published annotations collections), and will subsequently publish the annotations to UPP.
N.B.: Currently, if the hash value is empty, the request will succeed anyway. This may change in the future.

curl http://localhost:8080/draft/content/b7b871f6-8a89-11e4-8e24-00144feabdc0/annotations/publish -XPOST -H "Previous-Document-Hash:hashvalue" --data
'{
```
{
      "annotations":[
      {
        "predicate": "http://www.ft.com/ontology/annotation/hasContributor",
        "id": "http://www.ft.com/thing/5bd49568-6d7c-3c10-a5b0-2f3fd5974a6b",
      },
      {
        "predicate": "http://www.ft.com/ontology/annotation/about",
        "id": "http://www.ft.com/thing/d7de27f8-1633-3fcc-b308-c95a2ad7d1cd",
      },
      {
        "predicate": "http://www.ft.com/ontology/annotation/hasDisplayTag",
        "id": "http://www.ft.com/thing/d7de27f8-1633-3fcc-b308-c95a2ad7d1cd",
      }
    ]
}
```
}'


####Publish from Store####

```
curl http://localhost:8080/draft/content/b7b871f6-8a89-11e4-8e24-00144feabdc0/annotations/publish?fromStore=true -XPOST
```

By removing the request body, and setting the query parameter fromStore=true, a POST to the same endpoint will first retrieve the latest annotations from the PAC database, persist them to the PAC database published collection, and then publish to UPP.

## Healthchecks

Admin endpoints are:

`/__gtg`
`/__health`
`/__build-info`

At the moment the `/__health` endpoint checks the availability of the UPP Publishing cluster's `publish` category, and the `/__gtg` performs no checks (effectively a ping of the service).

### Logging

* The application uses [logrus](https://github.com/sirupsen/logrus); the log file is initialised in [main.go](main.go).
* Logging requires an `env` app parameter, for all environments other than `local` logs are written to file.
* When running locally, logs are written to console. If you want to log locally to file, you need to pass in an env parameter that is != `local`.
* NOTE: `/__build-info` and `/__gtg` endpoints are not logged as they are called every second from varnish/vulcand and this information is not needed in logs/splunk.

