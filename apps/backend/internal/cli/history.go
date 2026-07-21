package cli

import (
	"encoding/json"

	"github.com/weibinliao/OpenAD/internal/historyservice"
	"github.com/spf13/cobra"
)

func newHistoryCommand() *cobra.Command {
	var page int
	var pageSize int
	var status string
	var rootPath string

	command := &cobra.Command{
		Use:   "history",
		Short: "List scan history sessions from the local database",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := initDatabase(); err != nil {
				return err
			}

			service := historyservice.New()
			response, err := service.ListSessions(historyservice.SessionListFilter{
				Page:     page,
				PageSize: pageSize,
				Status:   status,
				RootPath: rootPath,
			})
			if err != nil {
				return err
			}

			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(response)
		},
	}

	command.Flags().IntVar(&page, "page", 1, "Page number")
	command.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	command.Flags().StringVar(&status, "status", "", "Filter by session status")
	command.Flags().StringVar(&rootPath, "root-path", "", "Filter by root path substring")

	command.AddCommand(newHistoryChangesCommand())
	return command
}

func newHistoryChangesCommand() *cobra.Command {
	var sessionID string
	var page int
	var pageSize int
	var changeType string
	var path string
	var trustee string

	command := &cobra.Command{
		Use:   "changes",
		Short: "List saved comparison changes for a scan session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := initDatabase(); err != nil {
				return err
			}

			service := historyservice.New()
			response, err := service.ListSessionChanges(sessionID, historyservice.ChangeListFilter{
				Page:       page,
				PageSize:   pageSize,
				ChangeType: changeType,
				Path:       path,
				Trustee:    trustee,
			})
			if err != nil {
				return err
			}

			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(response)
		},
	}

	command.Flags().StringVar(&sessionID, "session-id", "", "Scan session ID")
	command.Flags().IntVar(&page, "page", 1, "Page number")
	command.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	command.Flags().StringVar(&changeType, "change-type", "", "Filter by change type")
	command.Flags().StringVar(&path, "path", "", "Filter by path substring")
	command.Flags().StringVar(&trustee, "trustee", "", "Filter by trustee SID or account")
	_ = command.MarkFlagRequired("session-id")

	return command
}
