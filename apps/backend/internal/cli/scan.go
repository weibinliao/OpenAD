package cli

import (
	"encoding/json"

	"github.com/weibinliao/OpenAD/internal/scanservice"
	"github.com/spf13/cobra"
)

func newScanCommand() *cobra.Command {
	var depth int
	var includeInherited bool

	command := &cobra.Command{
		Use:   "scan <path>",
		Short: "Scan NTFS permissions recursively and print JSON results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := initDatabase(); err != nil {
				return err
			}

			service := scanservice.New()
			result, err := service.Run(scanservice.Request{
				Path:             args[0],
				MaxDepth:         depth,
				IncludeInherited: includeInherited,
			})
			if err != nil {
				return err
			}

			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")

			return encoder.Encode(result)
		},
	}

	command.Flags().IntVar(&depth, "depth", 3, "Maximum recursion depth (-1 for unlimited)")
	command.Flags().BoolVar(&includeInherited, "include-inherited", true, "Include inherited permissions in the output")

	return command
}
