# PAC Annotations Publisher

Annotations Publisher is a micro-service that Publishes annotations from TagMe to UPP.

## Code
annotations-publisher

## Primary URL
https://pac-prod-glb.upp.ft.com/__annotations-publisher

## Service Tier
Platinum

## Lifecycle Stage
Production
## Host Platform
AWS

## Architecture
Annotations Publisher is part of the PAC clusters, it is deployed in both EU and US regions with two replicas per deployment. The service is used to publish draft annotations to UPP.

[PAC architecture diagram](https://app.lucidchart.com/publicSegments/view/22c1656b-6242-4da6-9dfb-f7225c20f38f/image.png)

## Contains Personal Data
No

## Contains Sensitive Data
No

## Dependencies
- cms-metadata-notifier
- generic-rw-aurora
- draft-annotations-api

## Failover Architecture Type
ActiveActive

## Failover Process Type
FullyAutomated

## Failback Process Type
FullyAutomated

## Failover Details
The service is deployed in both PAC clusters. The failover guide for the cluster is located [here](https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/pac-cluster).

## Data Recovery Process Type
NotApplicable

## Data Recovery Details
The service does not store data, so it does not require any data recovery steps.

## Release Process Type
PartiallyAutomated

## Rollback Process Type
Manual

## Release Details
Manual failover is needed when a new version of the service is deployed to production. Otherwise, an automated failover is going to take place when releasing.
For more details about the failover process see the [PAC failover guide](https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/pac-cluster).

## Key Management Process Type
Manual

## Key Management Details
The service uses sealed secrets to manage Kubernetes secrets.
The actions required to create/change sealed secrets are described [here](https://github.com/Financial-Times/upp-docs/tree/master/guides/sealed-secrets-guide/).

## Monitoring
Health Checks:
- [PAC Prod EU](https://pac-prod-eu.upp.ft.com/__health/__pods-health?service-name=annotations-publisher)
- [PAC Prod US](https://pac-prod-us.upp.ft.com/__health/__pods-health?service-name=annotations-publisher)

Splunk Alerts:
- [PAC Annotations Publish Failures](https://financialtimes.splunkcloud.com/en-US/app/financial_times_production/alert?s=%2FservicesNS%2Fnobody%2Ffinancial_times_production%2Fsaved%2Fsearches%2FPAC%2520Annotations%2520Failures)

## First Line Troubleshooting
Please refer to the [First Line Troubleshooting guide](https://github.com/Financial-Times/upp-docs/tree/master/guides/ops/first-line-troubleshooting).

## Second Line Troubleshooting
Please refer to the GitHub repository README for troubleshooting information.
