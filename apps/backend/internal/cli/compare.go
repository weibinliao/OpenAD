package cli

import (
	"encoding/json"
	"errors"

	"github.com/weibinliao/OpenAD/internal/comparison"
	"github.com/weibinliao/OpenAD/internal/comparisonservice"
	"github.com/weibinliao/OpenAD/internal/historyservice"
	"github.com/spf13/cobra"
)

func newCompareCommand() *cobra.Command {
	var baselineSession string
	var currentSession string
	var saveChanges bool

	command := &cobra.Command{
		Use:   "compare",
		Short: "Compare two scan sessions from the local database",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := initDatabase(); err != nil {
				return err
			}

			if saveChanges {
				service := comparisonservice.New()
				report, err := service.Compare(comparisonservice.Request{
					BaselineSessionID: baselineSession,
					CurrentSessionID:  currentSession,
				})
				if err != nil {
					if errors.Is(err, historyservice.ErrDatabaseUnavailable) {
						return errors.New("database is not initialized")
					}
					return err
				}

				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(report)
			}

			baseline, baselinePerms, err := loadSessionAndPermissions(baselineSession)
			if err != nil {
				return err
			}

			current, currentPerms, err := loadSessionAndPermissions(currentSession)
			if err != nil {
				return err
			}

			engine := comparison.NewComparisonEngine(baseline, current)
			report, err := engine.DetectChanges(baselinePerms, currentPerms)
			if err != nil {
				return err
			}

			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(report)
		},
	}

	command.Flags().StringVar(&baselineSession, "baseline-session", "", "Baseline scan session ID")
	command.Flags().StringVar(&currentSession, "current-session", "", "Current scan session ID")
	command.Flags().BoolVar(&saveChanges, "save-changes", true, "Persist comparison changes into current session")
	_ = command.MarkFlagRequired("baseline-session")
	_ = command.MarkFlagRequired("current-session")

	return command
}
