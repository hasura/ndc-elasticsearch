package cli

import (
	"context"
	"fmt"

	"github.com/hasura/ndc-sdk-go/connector"
)

type Arguments struct {
	// The path to the configuration. Defaults to the current directory.
	Configuration string `help:"Configuration directory." env:"HASURA_PLUGIN_CONNECTOR_CONTEXT_PATH"`
}

type UpgradeArgs struct {
	// dirFrom is the path to the old configuration directory.
	DirFrom string `help:"Directory to upgrade from." required:"true"`
	// dirTo is the path to the new configuration directory.
	DirTo string `help:"Directory to upgrade to." required:"true"`
}

type CLI struct {
	connector.ServeCLI
	Version    struct{}    `cmd:"" help:"Print the version."`
	Initialize Arguments   `cmd:"" help:"Initialize the configuration directory."`
	Update     Arguments   `cmd:"" help:"Update the configuration directory."`
	Validate   Arguments   `cmd:"" help:"Validate the configuration directory."`
	Upgrade    UpgradeArgs `cmd:"" help:"Upgrade the configuration directory to be compatible with the latest connector version."`
}

// Execute executes the CLI command based on the provided command string.
func (cli *CLI) Execute(ctx context.Context, command string) error {
	logger := connector.GetLogger(ctx)

	switch command {
	case "version":
		logger.InfoContext(ctx, "v0.2.0")
	case "initialize":
		if err := initializeConfig(cli.Initialize.Configuration); err != nil {
			return fmt.Errorf("failed to initialize configuration: %w", err)
		}
		logger.InfoContext(ctx, "Configuration initialized successfully.")
	case "update":
		if err := updateConfig(ctx, cli.Update.Configuration); err != nil {
			return fmt.Errorf("failed to update configuration: %w", err)
		}
		logger.InfoContext(ctx, "Configuration updated successfully.")
	case "validate":
		if err := validateConfig(cli.Validate.Configuration); err != nil {
			return fmt.Errorf("failed to validate configuration: %w", err)
		}
		logger.InfoContext(ctx, "Configuration validated successfully.")
	case "upgrade":
		upgraded, err := upgradeConfig(cli.Upgrade.DirFrom, cli.Upgrade.DirTo)
		if err != nil {
			return fmt.Errorf("failed to upgrade configuration: %w", err)
		}
		if !upgraded {
			logger.InfoContext(ctx, "Configuration already up-to-date.")
			return nil
		}
		logger.InfoContext(ctx, "Configuration upgraded successfully.")
	default:
		return fmt.Errorf("unknown command <%s>", command)
	}

	return nil
}
