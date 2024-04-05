package main

import (
	"github.com/hasura/ndc-elasticsearch/cli"
	esConnector "github.com/hasura/ndc-elasticsearch/connector"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/connector"
)

func main() {
	var cli cli.CLI
	if err := connector.StartCustom[types.Configuration, types.State](
		&cli,
		&esConnector.Connector{},
		connector.WithMetricsPrefix("ndc-elasticsearch"),
		connector.WithDefaultServiceName("ndc-elasticsearch"),
	); err != nil {
		panic(err)
	}
}
