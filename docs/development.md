# Elasticsearch Connector Development

## Building the Connector

Prerequisites:

1. Install [Go](https://go.dev/doc/install)
2. Install [Docker](https://docs.docker.com/get-docker/)
3. Install [Hasura DDN Cli](https://hasura.io/docs/3.0/cli/installation)

### Steps

### 1. Clone the Repository

```sh
git clone https://github.com/hasura/ndc-elasticsearch.git
cd ndc-elasticsearch
```

### 2. Set Environment Variables

Set up the required environment variables for the Elasticsearch connector:
```sh
export ELASTICSEARCH_URL=<Your_Elasticsearch_Instance_URL>
export ELASTICSEARCH_USERNAME=<Your_Elasticsearch_Username>
export ELASTICSEARCH_PASSWORD=<Your_Elasticsearch_Password>
export ELASTICSEARCH_API_KEY=<Your_Elasticsearch_API_Key>
export ELASTICSEARCH_CA_CERT_PATH=<Path_To_Your_CA_Certificate>
export ELASTICSEARCH_INDEX_PATTERN=<Regex_Pattern_For_Indices>
```

Replace the placeholders with your actual Elasticsearch details.

### 3. Build the Connector

Compile the executable:
```go
go build
```

### 4. Initialize with Configuration 

Initialize with your database schema:
```sh
ndc-elasticsearch update
```

### 5. Running the Connector Locally

Execute the connector:
```sh
ndc-elasticsearch serve
```

Access at: http://localhost:8080

### 6. Verify

Check the schema:
```sh
curl http://localhost:8080/schema
```

Use the /query endpoint for queries.

## Docker Setup

Instructions for building and running the connector using Docker:

### Running the connector using Docker

Build the docker image using the provided `Dockerfile`:

```sh
docker build -t ndc-elasticsearch .
```

Run the Docker Container
```sh
docker run -p 8080:8080 -v <path_to_your_configuration.json>:/etc/connector/configuration.json -e "ELASTICSEARCH_URL:<Your_URL>" -e "ELASTICSEARCH_USERNAME:<Your_Username>" -e "ELASTICSEARCH_PASSWORD:<Your_Password>" -it ndc-elasticsearch
```

Replace placeholders with your Elasticsearch details.

### Development with v3-engine via Docker

Use `docker-compose.yaml` for setting up a local dev environment with Hasura `v3-engine` and other services:

```sh
docker compose up -d
open http://localhost:3000 # Graphiql
open http://localhost:4002 # Jaeger
open http://localhost:9090 # Prometheus
```

Update /etc/hosts Add required local mappings.

Load Sample Data:

Run the script below after starting Elasticsearch and Kibana to load sample data:

```sh
python resources/data/load_sample_data.py
```

Test with GraphiQL
```graphql
query MyQuery {
  app_dsKibanaSampleDataLogs {
    id
  }
}
```

Set x-hasura-role to admin in headers for testing.
```json
{"x-hasura-role": "admin"}
```