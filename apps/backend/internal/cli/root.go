package cli

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "scanner",
		Short: "OpenAD command line interface",
	}

	command.AddCommand(newScanCommand())
	command.AddCommand(newCompareCommand())
	command.AddCommand(newExportCommand())
	command.AddCommand(newHistoryCommand())

	return command
}

func Execute() error {
	return NewRootCommand().Execute()
}
