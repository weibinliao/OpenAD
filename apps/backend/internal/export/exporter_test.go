//go:build ignore
// +build ignore

package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestNewExporter(t *testing.T) {
	assert.NotNil(t, NewExporter())
}

func TestExportToCSVWritesHeaderAndRows(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions.csv")

	err := exporter.ExportToCSV([]models.Permission{{
		Path:      `C:\Finance`,
		Trustee:   `DOMAIN\Alice`,
		Rights:    "Read",
		Type:      "Allow",
		Inherited: false,
		Source:    "Explicit",
	}}, outputPath)

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "OpenAD 权限报告")
	assert.Contains(t, string(contents), "Account Name,First Name,Last Name,E-Mail,Department,Division,Domain,Originating Group,Permissions,Group Inheritance Hierarchy,Path,Entry Type,Inherited,Source,Applies To,Account Type,Risk Level,Parent Delta")
	assert.Contains(t, string(contents), `Alice,,,,,,DOMAIN,Explicit,Read,Explicit,C:\Finance,Allow,No,Explicit,,,LOW,Explicit on Current Item`)
}

func TestExportToExcelWritesSheetValues(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions.xlsx")

	err := exporter.ExportToExcel([]models.Permission{{
		Path:      `C:\Finance`,
		Trustee:   `DOMAIN\Alice`,
		Rights:    "Read",
		Type:      "Allow",
		Inherited: true,
		Source:    "Inherited",
	}}, outputPath)

	require.NoError(t, err)
	workbook, err := excelize.OpenFile(outputPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = workbook.Close()
	})

	cellValue, err := workbook.GetCellValue("用户权限", "A5")
	require.NoError(t, err)
	assert.Equal(t, "Alice", cellValue)

	cellValue, err = workbook.GetCellValue("ACL 明细", "D5")
	require.NoError(t, err)
	assert.Equal(t, "是", cellValue)
}

func TestExportToHTMLWritesTable(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions.html")

	err := exporter.ExportToHTML([]models.Permission{
		{
			Path:      `C:\Audit`,
			Trustee:   `DOMAIN\Team`,
			Rights:    "Modify",
			Type:      "Allow",
			Inherited: false,
			Source:    "Explicit",
		},
		{
			Path:      `C:\Audit\Logs`,
			Trustee:   `DOMAIN\Reader`,
			Rights:    "Read",
			Type:      "Allow",
			Inherited: true,
			Source:    "Inherited",
		},
	}, outputPath)

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	body := string(contents)
	assert.Contains(t, body, "<table>")
	assert.Contains(t, body, "高频主体")
	assert.Contains(t, body, "权限条目")
	assert.Contains(t, body, `C:\Audit`)
	assert.Contains(t, body, "DOMAIN\\Team")
}

func TestExportToHTMLWritesTableValues(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions.html")

	err := exporter.ExportToHTML([]models.Permission{{
		Path:      `C:\Finance <Q4>`,
		Trustee:   `DOMAIN\Alice & Bob`,
		Rights:    "Read",
		Type:      "Allow",
		Inherited: false,
		Source:    "Explicit",
	}}, outputPath)

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	body := string(contents)
	assert.Contains(t, body, "<table>")
	assert.Contains(t, body, "<th>Account Name</th>")
	assert.Contains(t, body, "C:\\Finance &lt;Q4&gt;")
	assert.Contains(t, body, "DOMAIN\\Alice &amp; Bob")
	assert.True(t, strings.Contains(body, "<td>否</td>"))
}

func TestExportToHTMLWritesEmptyState(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions-empty.html")

	err := exporter.ExportToHTML(nil, outputPath)
	require.NoError(t, err)

	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "当前没有可导出的用户权限视图。")
}
