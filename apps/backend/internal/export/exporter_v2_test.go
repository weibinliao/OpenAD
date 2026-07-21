package export

import (
	"encoding/csv"
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
		Path:        `C:\Finance`,
		Trustee:     `DOMAIN\Alice`,
		Rights:      "Read",
		Type:        "Allow",
		Inherited:   false,
		Source:      "Explicit",
		AccountName: "Alice",
	}}, outputPath, Options{})

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	body := string(contents)
	assert.Contains(t, body, "OpenAD 权限报告")
	assert.Contains(t, body, "Account Name,First Name,Last Name,E-Mail,Department,Division,Domain,Originating Group,Permissions,Group Inheritance Hierarchy")
	assert.Contains(t, body, `Alice,,,,,,DOMAIN,DOMAIN\Alice,允许: 读取,`)
}

func TestExportToCSVAggregatesPermissionsPerPathAndIdentity(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions-aggregated.csv")

	err := exporter.ExportToCSV([]models.Permission{
		{
			Path:             `C:\Finance`,
			Trustee:          `DOMAIN\Alice`,
			Rights:           "Read",
			Type:             "Allow",
			Inherited:        false,
			Source:           "Explicit",
			AccountName:      "Alice",
			AppliesTo:        "This Folder Only",
			AccountType:      "User",
			RiskLevel:        "low",
			ParentDelta:      "Explicit on Current Item",
			OriginatingGroup: `DOMAIN\Alice`,
		},
		{
			Path:             `C:\Finance`,
			Trustee:          `DOMAIN\Alice`,
			Rights:           "Write",
			Type:             "Allow",
			Inherited:        true,
			Source:           "Inherited",
			AccountName:      "Alice",
			AppliesTo:        "This Folder and Files",
			AccountType:      "User",
			RiskLevel:        "high",
			ParentDelta:      "Inherited from Parent",
			OriginatingGroup: `DOMAIN\Finance-Team`,
		},
	}, outputPath, Options{})

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	body := string(contents)
	assert.Equal(t, 1, strings.Count(body, "Alice,,,,,,DOMAIN,"))
	assert.Contains(t, body, "DOMAIN\\Alice / DOMAIN\\Finance-Team")
	assert.Contains(t, body, "允许: 读取, 仅此文件夹 / 允许: 写入, 此文件夹和文件")
}

func TestExportToCSVUsesProvidedSemanticUserRows(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions-contract.csv")

	err := exporter.ExportToCSV([]models.Permission{{
		Path:             `C:\Finance`,
		Trustee:          `DOMAIN\Alice`,
		TrusteeSID:       `S-1-5-21-1000`,
		Rights:           "Read",
		Type:             "Allow",
		Inherited:        false,
		Source:           "Explicit",
		AccountName:      "Alice",
		OriginatingGroup: `DOMAIN\Alice`,
	}}, outputPath, Options{
		UserRows: []UserRow{{
			Path:                      `C:\Finance`,
			Trustee:                   `DOMAIN\Alice`,
			TrusteeSID:                `S-1-5-21-1000`,
			AccountName:               "alice",
			FirstName:                 "Alice",
			LastName:                  "Ng",
			Email:                     "alice@example.com",
			Department:                "Finance",
			Division:                  "Operations",
			Domain:                    "DOMAIN",
			OriginatingGroup:          `DOMAIN\Frontend-Contract`,
			Permissions:               "Allow: Read, This Folder Only",
			GroupInheritanceHierarchy: `DOMAIN\Frontend-Contract`,
			PermissionCount:           1,
			RiskLevel:                 "low",
			AppliesToSummary:          "This Folder Only",
			InheritanceSummary:        "Explicit",
			RowCount:                  1,
			MemberKeys:                []string{"finance-read"},
		}},
	})

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	rows := readCSVRows(t, contents)
	require.Len(t, rows, 3)
	assert.Equal(t, []string{"alice", "Alice", "Ng", "alice@example.com", "Finance", "Operations", "DOMAIN", `DOMAIN\Frontend-Contract`, "Allow: Read, This Folder Only", `DOMAIN\Frontend-Contract`}, rows[2])
	assert.NotContains(t, strings.Join(rows[2], ","), "允许: 读取")
}

func TestExportToCSVDeduplicatesPresentationRows(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions-dedup.csv")

	err := exporter.ExportToCSV(nil, outputPath, Options{
		UserRows: []UserRow{
			{
				Path:                      `C:\Finance`,
				AccountName:               "alice",
				FirstName:                 "Alice",
				LastName:                  "Ng",
				Email:                     "alice@example.com",
				Department:                "Finance",
				Division:                  "Operations",
				Domain:                    "DOMAIN",
				OriginatingGroup:          `DOMAIN\Finance-Team`,
				Permissions:               "Allow: Read, This Folder Only",
				GroupInheritanceHierarchy: `DOMAIN\Finance-Team`,
			},
			{
				Path:                      `C:\Finance\Archive`,
				AccountName:               "alice",
				FirstName:                 "Alice",
				LastName:                  "Ng",
				Email:                     "alice@example.com",
				Department:                "Finance",
				Division:                  "Operations",
				Domain:                    "DOMAIN",
				OriginatingGroup:          `DOMAIN\Finance-Team`,
				Permissions:               "Allow: Read, This Folder Only",
				GroupInheritanceHierarchy: `DOMAIN\Finance-Team`,
			},
		},
	})

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	rows := readCSVRows(t, contents)
	require.Len(t, rows, 3)
	assert.Equal(t, []string{"alice", "Alice", "Ng", "alice@example.com", "Finance", "Operations", "DOMAIN", `DOMAIN\Finance-Team`, "Allow: Read, This Folder Only", `DOMAIN\Finance-Team`}, rows[2])
}

func readCSVRows(t *testing.T, contents []byte) [][]string {
	t.Helper()

	reader := csv.NewReader(strings.NewReader(string(contents)))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	require.NoError(t, err)

	return rows
}

func TestExportToCSVSupportsScanResultsMode(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "scan-results.csv")

	err := exporter.ExportToCSV([]models.Permission{{
		Path:             `C:\Finance`,
		Trustee:          `DOMAIN\Alice`,
		TrusteeSID:       `S-1-5-21-1000`,
		Rights:           "Read and Execute",
		Type:             "Allow",
		Inherited:        true,
		Source:           "Inherited from share root",
		AppliesTo:        "This Folder and Files",
		AccountName:      "alice",
		FirstName:        "Alice",
		LastName:         "Ng",
		Email:            "alice@example.com",
		Department:       "Finance",
		Division:         "Operations",
		Domain:           "DOMAIN",
		OriginatingGroup: `DOMAIN\Finance-Team`,
	}}, outputPath, Options{Mode: "scan-results", Title: "Finance Scan Results"})

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	body := string(contents)

	assert.Contains(t, body, "Finance Scan Results")
	assert.Contains(t, body, "Path,Account Name,First Name,Last Name,E-Mail,Department,Division,Domain,Trustee,Trustee SID,Originating Group,Rights,Type,Inherited,Applies To,Source")
	assert.Contains(t, body, `C:\Finance,alice,Alice,Ng,alice@example.com,Finance,Operations,DOMAIN,DOMAIN\Alice,S-1-5-21-1000,DOMAIN\Finance-Team,读取和执行,允许,是,此文件夹和文件,Inherited from share root`)
}

func TestExportToExcelWritesSheetValues(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions.xlsx")

	err := exporter.ExportToExcel([]models.Permission{{
		Path:        `C:\Finance`,
		Trustee:     `DOMAIN\Alice`,
		Rights:      "Read",
		Type:        "Allow",
		Inherited:   true,
		Source:      "Inherited",
		AccountName: "Alice",
	}}, outputPath, Options{})

	require.NoError(t, err)
	workbook, err := excelize.OpenFile(outputPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = workbook.Close() })

	cellValue, err := workbook.GetCellValue("1", "A5")
	require.NoError(t, err)
	assert.Equal(t, "Alice", cellValue)

	cellValue, err = workbook.GetCellValue("1", "A4")
	require.NoError(t, err)
	assert.Equal(t, "Account Name", cellValue)

	cellValue, err = workbook.GetCellValue("1", "A2")
	require.NoError(t, err)
	assert.Equal(t, "用户权限汇总", cellValue)
}

func TestExportToExcelSupportsScanResultsMode(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "scan-results.xlsx")

	err := exporter.ExportToExcel([]models.Permission{{
		Path:        `C:\Finance`,
		Trustee:     `DOMAIN\Alice`,
		TrusteeSID:  `S-1-5-21-1000`,
		Rights:      "Read",
		Type:        "Allow",
		Inherited:   false,
		AppliesTo:   "This Folder Only",
		Source:      "Explicit",
		AccountName: "Alice",
	}}, outputPath, Options{Mode: "scan-results"})

	require.NoError(t, err)
	workbook, err := excelize.OpenFile(outputPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = workbook.Close() })

	cellValue, err := workbook.GetCellValue("1", "A2")
	require.NoError(t, err)
	assert.Equal(t, "扫描结果明细", cellValue)

	cellValue, err = workbook.GetCellValue("1", "A4")
	require.NoError(t, err)
	assert.Equal(t, "Path", cellValue)

	cellValue, err = workbook.GetCellValue("1", "B5")
	require.NoError(t, err)
	assert.Equal(t, "Alice", cellValue)
}

func TestExportToHTMLWritesTable(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions.html")

	err := exporter.ExportToHTML([]models.Permission{
		{Path: `C:\Audit`, Trustee: `DOMAIN\Team`, Rights: "Modify", Type: "Allow", Inherited: false, Source: "Explicit"},
		{Path: `C:\Audit\Logs`, Trustee: `DOMAIN\Reader`, Rights: "Read", Type: "Allow", Inherited: true, Source: "Inherited"},
	}, outputPath, Options{})

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	body := string(contents)

	assert.Contains(t, body, "<table class=\"primary-table\">")
	assert.Contains(t, body, "权限汇总")
	assert.NotContains(t, body, "访问控制列表")
	assert.Contains(t, body, "DOMAIN\\Team")
}

func TestExportToHTMLEscapesValues(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions.html")

	err := exporter.ExportToHTML([]models.Permission{{
		Path:        `C:\Finance <Q4>`,
		Trustee:     `DOMAIN\Alice & Bob`,
		Rights:      "Read",
		Type:        "Allow",
		Inherited:   false,
		Source:      "Explicit",
		AccountName: `Alice <Lead>`,
	}}, outputPath, Options{})

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	body := string(contents)

	assert.Contains(t, body, "<th>Account Name</th>")
	assert.Contains(t, body, "Alice &lt;Lead&gt;")
	assert.Contains(t, body, "DOMAIN\\Alice &amp; Bob")
	assert.Contains(t, body, "用户权限汇总")
}

func TestExportToHTMLWritesEmptyState(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "permissions-empty.html")

	err := exporter.ExportToHTML(nil, outputPath, Options{})
	require.NoError(t, err)

	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "当前没有可导出的用户权限结果。")
}

func TestExportToHTMLSupportsScanResultsMode(t *testing.T) {
	exporter := NewExporter()
	outputPath := filepath.Join(t.TempDir(), "scan-results.html")

	err := exporter.ExportToHTML([]models.Permission{{
		Path:        `C:\Finance`,
		Trustee:     `DOMAIN\Alice`,
		TrusteeSID:  `S-1-5-21-1000`,
		Rights:      "Read",
		Type:        "Allow",
		Inherited:   false,
		AppliesTo:   "This Folder Only",
		Source:      "Explicit",
		AccountName: `Alice`,
	}}, outputPath, Options{Mode: "scan-results", Title: "Finance Scan Results"})

	require.NoError(t, err)
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	body := string(contents)

	assert.Contains(t, body, "Finance Scan Results")
	assert.Contains(t, body, "扫描结果明细")
	assert.Contains(t, body, "<th>Path</th>")
	assert.Contains(t, body, "C:\\Finance")
	assert.NotContains(t, body, "当前没有可导出的扫描结果")
}
