package cli

import (
	"context"
	"fmt"

	"github.com/hasura/ndc-sdk-go/connector"
)

type Arguments struct {
	// The path to the configuration. Defaults to the current directory.
	Configuration string `help:"Configuration directory." env:"HASURA_CONFIGURATION_DIRECTORY"`
}

type CLI struct {
	connector.ServeCLI
	Version    struct{}  `cmd:"" help:"Print the version."`
	Initialize Arguments `cmd:"" help:"Initialize configuration directory."`
	Update     Arguments `cmd:"" help:"Update configuration directory."`
	Validate   Arguments `cmd:"" help:"Validate configuration directory."`
}

func (cli *CLI) Execute(ctx context.Context, command string) error {
	logger := connector.GetLogger(ctx)
	switch command {
	case "version":
		logger.InfoContext(ctx, "v0.1.0")
		return nil
	case "initialize":
		err := initialize(cli.Initialize.Configuration)
		if err != nil {
			return err
		}
		logger.InfoContext(ctx, "Configuration Initialized Successfully.")
		return nil
	case "update":
		err := updateConfiguration(ctx, cli.Update.Configuration)
		if err != nil {
			return err
		}
		logger.InfoContext(ctx, "Configuration Updated Successfully.")
		return nil
	case "validate":
		err := validate(cli.Validate.Configuration)
		if err != nil {
			return err
		}
		logger.InfoContext(ctx, "Configuration Validated Successfully.")
		return nil
	default:
		return fmt.Errorf("unknown command <%s>", command)
	}
}
