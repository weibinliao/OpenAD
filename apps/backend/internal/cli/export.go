package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/weibinliao/OpenAD/internal/export"
	"github.com/spf13/cobra"
)

func newExportCommand() *cobra.Command {
	var sessionID string
	var format string
	var output string

	command := &cobra.Command{
		Use:   "export",
		Short: "Export a scan session from the local database",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := initDatabase(); err != nil {
				return err
			}

			_, permissions, err := loadSessionAndPermissions(sessionID)
			if err != nil {
				return err
			}

			if strings.TrimSpace(output) == "" {
				return fmt.Errorf("--output is required")
			}

			if err := ensureParentDir(output); err != nil {
				return err
			}

			exporter := export.NewExporter()
			switch strings.ToLower(strings.TrimSpace(format)) {
			case "csv":
				err = exporter.ExportToCSV(permissions, output, export.Options{})
			case "excel":
				err = exporter.ExportToExcel(permissions, output, export.Options{})
			case "html":
				err = exporter.ExportToHTML(permissions, output, export.Options{})
			default:
				return fmt.Errorf("unsupported format %q: expected csv, excel, or html", format)
			}
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "exported %d permissions to %s\n", len(permissions), output)
			return nil
		},
	}

	command.Flags().StringVar(&sessionID, "session-id", "", "Scan session ID to export")
	command.Flags().StringVar(&format, "format", "csv", "Export format: csv, excel, or html")
	command.Flags().StringVar(&output, "output", "", "Output file path")
	_ = command.MarkFlagRequired("session-id")
	_ = command.MarkFlagRequired("output")

	return command
}

func ensureParentDir(filePath string) error {
	parentDir := filepath.Dir(filePath)
	if parentDir == "." || parentDir == "" {
		return nil
	}

	return ensureDirectory(parentDir)
}

func ensureDirectory(dir string) error {
	if dir == "" {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}
