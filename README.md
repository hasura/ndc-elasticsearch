# ndc-elasticsearch Connector

## Get started

### DDN CLI

DDN CLI automatically starts the connector with `ddn dev` command that uses Docker Compose internally. 
Docker Compose can watch changes in the source code and rebuild the connector. However, this connector limits the changes in the root folder only to avoid rebuild noises.
After editing files in child folders, save any `*.go` file in the root folder to trigger the build.

### Native Go

Start the connector server at http://localhost:8080

```go
go run . serve
```

## Code generation

### Convenience script (Linux & MacOS only)

You can run the convenience script with `make` or with the bash file directly. 
The script automatically downloads the tool and runs the `generate` command.

```bash
make generate
```

```bash
sh ./scripts/generate.sh
```

### Manually download

Download the `hasura-ndc-go` tool at the [release page](https://github.com/hasura/ndc-sdk-go/releases/tag/v1.0.0) page.
Navigate to the root project folder and run `generate` whenever there are new changes from NDC functions and types.

```sh
hasura-ndc-go generate
```

See [NDC Go SDK](https://github.com/hasura/ndc-sdk-go) for more information and [the generation tool](https://github.com/hasura/ndc-sdk-go/tree/main/cmd/ndc-go-sdk) for command documentation.
