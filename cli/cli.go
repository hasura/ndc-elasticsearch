package cli

import (
	"context"
	"fmt"

	"github.com/hasura/ndc-sdk-go/connector"
)

type CLI struct {
	connector.ServeCLI
	Version  struct{} `cmd:"" help:"Print the version."`
	Update   struct{} `cmd:"" help:"Update the configurations."`
	Validate struct{} `cmd:"" help:"Validate the elasticsearch credentials."`
}

func (cli *CLI) Execute(ctx context.Context, command string) error {
	logger := connector.GetLogger(ctx)
	switch command {
	case "version":
		fmt.Println("v0.1.0")
		return nil
	case "update":
		err := updateConfiguration()
		if err != nil {
			return err
		}
		logger.Info("Configuration Updated Successfully.")
		return nil
	case "validate":
		return nil
	default:
		return fmt.Errorf("unknown command <%s>", command)
	}
}
