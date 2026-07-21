package export

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/xuri/excelize/v2"
)

type Exporter struct{}

type Options struct {
	Title        string
	Mode         string
	Template     string
	Sections     []string
	Organization string
	PreparedBy   string
	ReportPeriod string
	FocusAreas   []string
	ADFields     []string
	FileColumns  []string
	UserRows     []UserRow
}

type UserRow struct {
	ID                        string   `json:"id,omitempty"`
	Path                      string   `json:"path,omitempty"`
	Trustee                   string   `json:"trustee,omitempty"`
	TrusteeSID                string   `json:"trustee_sid,omitempty"`
	AccountName               string   `json:"account_name,omitempty"`
	FirstName                 string   `json:"first_name,omitempty"`
	LastName                  string   `json:"last_name,omitempty"`
	Email                     string   `json:"email,omitempty"`
	Department                string   `json:"department,omitempty"`
	Division                  string   `json:"division,omitempty"`
	Domain                    string   `json:"domain,omitempty"`
	OriginatingGroup          string   `json:"originating_group,omitempty"`
	Permissions               string   `json:"permissions,omitempty"`
	GroupInheritanceHierarchy string   `json:"group_inheritance_hierarchy,omitempty"`
	PermissionCount           int      `json:"permission_count,omitempty"`
	RiskLevel                 string   `json:"risk_level,omitempty"`
	AppliesToSummary          string   `json:"applies_to_summary,omitempty"`
	InheritanceSummary        string   `json:"inheritance_summary,omitempty"`
	RowCount                  int      `json:"row_count,omitempty"`
	MemberKeys                []string `json:"member_keys,omitempty"`
}

type htmlReport struct {
	ReportTitle    string
	ReportSubTitle string
	Mode           string
	GeneratedAt    string
	UserRows       []UserRow
	Permissions    []models.Permission
}

type scanResultRow struct {
	Path             string
	AccountName      string
	FirstName        string
	LastName         string
	Email            string
	Department       string
	Division         string
	Domain           string
	Trustee          string
	TrusteeSID       string
	OriginatingGroup string
	Rights           string
	Type             string
	Inherited        string
	AppliesTo        string
	Source           string
}

func NewExporter() *Exporter {
	return &Exporter{}
}

func (e *Exporter) ExportToCSV(permissions []models.Permission, filename string, options Options) error {
	if normalizeExportMode(options.Mode) == "scan-results" {
		return e.exportScanResultsToCSV(permissions, filename, options)
	}

	if err := ensureParentDir(filename); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	report := buildReport(permissions, options)
	exportRows := exportPresentationRows(report.UserRows)
	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{fmt.Sprintf("%s %s", report.GeneratedAt, report.ReportTitle)}); err != nil {
		return err
	}

	headers := []string{
		"Account Name",
		"First Name",
		"Last Name",
		"E-Mail",
		"Department",
		"Division",
		"Domain",
		"Originating Group",
		"Permissions",
		"Group Inheritance Hierarchy",
	}
	if err := writer.Write(headers); err != nil {
		return err
	}

	for _, row := range exportRows {
		if err := writer.Write([]string{
			row.AccountName,
			row.FirstName,
			row.LastName,
			row.Email,
			row.Department,
			row.Division,
			row.Domain,
			row.OriginatingGroup,
			row.Permissions,
			row.GroupInheritanceHierarchy,
		}); err != nil {
			return err
		}
	}

	return writer.Error()
}

func (e *Exporter) ExportToExcel(permissions []models.Permission, filename string, options Options) error {
	if normalizeExportMode(options.Mode) == "scan-results" {
		return e.exportScanResultsToExcel(permissions, filename, options)
	}

	if err := ensureParentDir(filename); err != nil {
		return err
	}

	report := buildReport(permissions, options)
	exportRows := exportPresentationRows(report.UserRows)
	book := excelize.NewFile()
	defer book.Close()

	const sheet = "1"
	book.SetSheetName("Sheet1", sheet)

	titleStyle, _ := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "#225797"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	timestampStyle, _ := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "#225797"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	primaryHeaderStyle, _ := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#225797"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "#CDD8E5", Style: 1},
			{Type: "right", Color: "#CDD8E5", Style: 1},
			{Type: "top", Color: "#CDD8E5", Style: 1},
			{Type: "bottom", Color: "#CDD8E5", Style: 1},
		},
	})
	bodyStyle, _ := book.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "#E1E7EF", Style: 1},
			{Type: "right", Color: "#E1E7EF", Style: 1},
			{Type: "top", Color: "#E1E7EF", Style: 1},
			{Type: "bottom", Color: "#E1E7EF", Style: 1},
		},
	})

	_ = book.MergeCell(sheet, "B1", "I1")
	book.SetCellValue(sheet, "A1", report.GeneratedAt)
	book.SetCellValue(sheet, "B1", report.ReportTitle)

	userHeaders := []string{
		"Account Name",
		"First Name",
		"Last Name",
		"E-Mail",
		"Department",
		"Division",
		"Domain",
		"Originating Group",
		"Permissions",
		"Group Inheritance Hierarchy",
	}
	for i, header := range userHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 4)
		book.SetCellValue(sheet, cell, header)
	}
	book.SetCellValue(sheet, "A2", report.ReportSubTitle)
	book.SetCellValue(sheet, "A3", fmt.Sprintf("共 %d 条汇总记录。", len(exportRows)))

	for rowIndex, row := range exportRows {
		values := []any{
			row.AccountName,
			row.FirstName,
			row.LastName,
			row.Email,
			row.Department,
			row.Division,
			row.Domain,
			row.OriginatingGroup,
			row.Permissions,
			row.GroupInheritanceHierarchy,
		}
		for colIndex, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+5)
			book.SetCellValue(sheet, cell, value)
		}
	}

	lastUserRow := maxInt(4, len(exportRows)+4)

	_ = book.SetCellStyle(sheet, "A1", "A1", timestampStyle)
	_ = book.SetCellStyle(sheet, "B1", "I1", titleStyle)
	_ = book.SetCellStyle(sheet, "A4", "J4", primaryHeaderStyle)
	if len(exportRows) > 0 {
		_ = book.SetCellStyle(sheet, "A5", fmt.Sprintf("J%d", lastUserRow), bodyStyle)
	}
	_ = book.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, XSplit: 0, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"})
	_ = book.AutoFilter(sheet, "A4:J4", nil)

	_ = book.SetRowHeight(sheet, 1, 24)
	_ = book.SetRowHeight(sheet, 2, 18)
	_ = book.SetRowHeight(sheet, 3, 18)
	_ = book.SetColWidth(sheet, "A", "A", 18)
	_ = book.SetColWidth(sheet, "B", "C", 14)
	_ = book.SetColWidth(sheet, "D", "D", 22)
	_ = book.SetColWidth(sheet, "E", "F", 16)
	_ = book.SetColWidth(sheet, "G", "G", 14)
	_ = book.SetColWidth(sheet, "H", "H", 22)
	_ = book.SetColWidth(sheet, "I", "I", 30)
	_ = book.SetColWidth(sheet, "J", "J", 28)

	if index, err := book.GetSheetIndex(sheet); err == nil {
		book.SetActiveSheet(index)
	}

	return book.SaveAs(filename)
}

func (e *Exporter) ExportToHTML(permissions []models.Permission, filename string, options Options) error {
	if normalizeExportMode(options.Mode) == "scan-results" {
		return e.exportScanResultsToHTML(permissions, filename, options)
	}

	if err := ensureParentDir(filename); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	report := buildReport(permissions, options)
	const doc = `<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.ReportTitle}}</title><style>body{margin:0;background:#f5f7fb;color:#1f2933;font-family:Arial,Verdana,"Segoe UI",sans-serif;font-size:12px;line-height:1.4}main{width:min(1280px,calc(100% - 32px));margin:18px auto 42px}.report-card{background:#fff;border:1px solid #d8e0ea;box-shadow:0 18px 48px rgba(15,23,42,.08)}.report-header{display:flex;justify-content:space-between;align-items:flex-start;gap:16px;color:#225797;padding:18px 20px 12px;border-bottom:1px solid #e3e9f1;background:linear-gradient(180deg,#fff,#f8fbff)}.timestamp{font-size:12px;font-weight:600;white-space:nowrap}.title-wrap{text-align:center;flex:1}.title{font-size:18px;font-weight:700}.subtitle{margin-top:6px;font-size:12px;color:#5a6673}.summary-line{margin:0;padding:10px 20px;color:#5a6673;font-size:12px;border-bottom:1px solid #edf2f7;background:#fbfdff}.section-title{margin:0;padding:12px 20px 6px;color:#225797;font-size:14px;font-weight:700}.empty{margin:0 20px 18px;color:#5a6673}.table-wrap{width:100%;overflow:auto;padding:0 20px 20px}table{width:100%;border-collapse:collapse}th,td{border:1px solid #cfd8e3;padding:6px 8px;vertical-align:top}th{text-align:left;font-weight:700;color:#225797;background:#f7fafc;position:sticky;top:0}td{word-break:break-word}.primary-table{table-layout:fixed}.primary-table col:nth-child(1){width:13%}.primary-table col:nth-child(2){width:9%}.primary-table col:nth-child(3){width:9%}.primary-table col:nth-child(4){width:14%}.primary-table col:nth-child(5){width:10%}.primary-table col:nth-child(6){width:9%}.primary-table col:nth-child(7){width:8%}.primary-table col:nth-child(8){width:12%}.primary-table col:nth-child(9){width:8%}.primary-table col:nth-child(10){width:8%}</style></head><body><main><div class="report-card"><div class="report-header"><span class="timestamp">{{.GeneratedAt}}</span><div class="title-wrap"><div class="title">{{.ReportTitle}}</div><div class="subtitle">{{.ReportSubTitle}}</div></div><span></span></div><p class="summary-line">共 {{len .UserRows}} 条汇总记录。</p><h1 class="section-title">权限汇总</h1>{{if .UserRows}}<div class="table-wrap"><table class="primary-table"><colgroup><col><col><col><col><col><col><col><col><col><col></colgroup><thead><tr><th>Account Name</th><th>First Name</th><th>Last Name</th><th>E-Mail</th><th>Department</th><th>Division</th><th>Domain</th><th>Originating Group</th><th>Permissions</th><th>Group Inheritance Hierarchy</th></tr></thead><tbody>{{range .UserRows}}<tr><td>{{.AccountName}}</td><td>{{.FirstName}}</td><td>{{.LastName}}</td><td>{{.Email}}</td><td>{{.Department}}</td><td>{{.Division}}</td><td>{{.Domain}}</td><td>{{.OriginatingGroup}}</td><td>{{.Permissions}}</td><td>{{.GroupInheritanceHierarchy}}</td></tr>{{end}}</tbody></table></div>{{else}}<p class="empty">当前没有可导出的用户权限结果。</p>{{end}}</div></main></body></html>`

	tmpl, err := template.New("export").Parse(doc)
	if err != nil {
		return err
	}

	return tmpl.Execute(file, report)
}

func (e *Exporter) exportScanResultsToCSV(permissions []models.Permission, filename string, options Options) error {
	if err := ensureParentDir(filename); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{fmt.Sprintf("%s %s", formatExportTimestamp(time.Now()), reportDocumentTitle(options))}); err != nil {
		return err
	}

	headers := []string{
		"Path",
		"Account Name",
		"First Name",
		"Last Name",
		"E-Mail",
		"Department",
		"Division",
		"Domain",
		"Trustee",
		"Trustee SID",
		"Originating Group",
		"Rights",
		"Type",
		"Inherited",
		"Applies To",
		"Source",
	}
	if err := writer.Write(headers); err != nil {
		return err
	}

	for _, row := range buildScanResultRows(permissions) {
		if err := writer.Write([]string{
			row.Path,
			row.AccountName,
			row.FirstName,
			row.LastName,
			row.Email,
			row.Department,
			row.Division,
			row.Domain,
			row.Trustee,
			row.TrusteeSID,
			row.OriginatingGroup,
			row.Rights,
			row.Type,
			row.Inherited,
			row.AppliesTo,
			row.Source,
		}); err != nil {
			return err
		}
	}

	return writer.Error()
}

func (e *Exporter) exportScanResultsToExcel(permissions []models.Permission, filename string, options Options) error {
	if err := ensureParentDir(filename); err != nil {
		return err
	}

	rows := buildScanResultRows(permissions)
	book := excelize.NewFile()
	defer book.Close()

	const sheet = "1"
	book.SetSheetName("Sheet1", sheet)

	titleStyle, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 16, Color: "#225797"}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	timestampStyle, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 12, Color: "#225797"}, Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"}})
	headStyle, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#225797"}}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}})
	bodyStyle, _ := book.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})

	_ = book.MergeCell(sheet, "B1", "P1")
	book.SetCellValue(sheet, "A1", formatExportTimestamp(time.Now()))
	book.SetCellValue(sheet, "B1", reportDocumentTitle(options))
	book.SetCellValue(sheet, "A2", reportSubTitle("scan-results", options))
	book.SetCellValue(sheet, "A3", fmt.Sprintf("共 %d 条扫描结果记录。", len(rows)))

	headers := []string{"Path", "Account Name", "First Name", "Last Name", "E-Mail", "Department", "Division", "Domain", "Trustee", "Trustee SID", "Originating Group", "Rights", "Type", "Inherited", "Applies To", "Source"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 4)
		book.SetCellValue(sheet, cell, header)
	}

	for rowIndex, row := range rows {
		values := []any{row.Path, row.AccountName, row.FirstName, row.LastName, row.Email, row.Department, row.Division, row.Domain, row.Trustee, row.TrusteeSID, row.OriginatingGroup, row.Rights, row.Type, row.Inherited, row.AppliesTo, row.Source}
		for colIndex, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+5)
			book.SetCellValue(sheet, cell, value)
		}
	}

	lastRow := maxInt(4, len(rows)+4)
	_ = book.SetCellStyle(sheet, "A1", "A1", timestampStyle)
	_ = book.SetCellStyle(sheet, "B1", "P1", titleStyle)
	_ = book.SetCellStyle(sheet, "A4", "P4", headStyle)
	if len(rows) > 0 {
		_ = book.SetCellStyle(sheet, "A5", fmt.Sprintf("P%d", lastRow), bodyStyle)
	}
	_ = book.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, XSplit: 0, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"})
	_ = book.AutoFilter(sheet, "A4:P4", nil)
	_ = book.SetColWidth(sheet, "A", "A", 32)
	_ = book.SetColWidth(sheet, "B", "D", 16)
	_ = book.SetColWidth(sheet, "E", "E", 24)
	_ = book.SetColWidth(sheet, "F", "G", 16)
	_ = book.SetColWidth(sheet, "H", "H", 14)
	_ = book.SetColWidth(sheet, "I", "J", 24)
	_ = book.SetColWidth(sheet, "K", "K", 24)
	_ = book.SetColWidth(sheet, "L", "L", 28)
	_ = book.SetColWidth(sheet, "M", "P", 18)

	if index, err := book.GetSheetIndex(sheet); err == nil {
		book.SetActiveSheet(index)
	}

	return book.SaveAs(filename)
}

func (e *Exporter) exportScanResultsToHTML(permissions []models.Permission, filename string, options Options) error {
	if err := ensureParentDir(filename); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	data := struct {
		Title       string
		SubTitle    string
		GeneratedAt string
		Rows        []scanResultRow
	}{
		Title:       reportDocumentTitle(options),
		SubTitle:    reportSubTitle("scan-results", options),
		GeneratedAt: formatExportTimestamp(time.Now()),
		Rows:        buildScanResultRows(permissions),
	}

	const doc = `<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><style>body{margin:0;background:#f5f7fb;color:#1f2933;font-family:Arial,Verdana,"Segoe UI",sans-serif;font-size:12px;line-height:1.4}main{width:min(1480px,calc(100% - 32px));margin:18px auto 42px}.report-card{background:#fff;border:1px solid #d8e0ea;box-shadow:0 18px 48px rgba(15,23,42,.08)}.report-header{display:flex;justify-content:space-between;align-items:flex-start;gap:16px;color:#225797;padding:18px 20px 12px;border-bottom:1px solid #e3e9f1;background:linear-gradient(180deg,#fff,#f8fbff)}.timestamp{font-size:12px;font-weight:600;white-space:nowrap}.title-wrap{text-align:center;flex:1}.title{font-size:18px;font-weight:700}.subtitle{margin-top:6px;font-size:12px;color:#5a6673}.summary-line{margin:0;padding:10px 20px;color:#5a6673;font-size:12px;border-bottom:1px solid #edf2f7;background:#fbfdff}.section-title{margin:0;padding:12px 20px 6px;color:#225797;font-size:14px;font-weight:700}.empty{margin:0 20px 18px;color:#5a6673}.table-wrap{width:100%;overflow:auto;padding:0 20px 20px}table{width:100%;border-collapse:collapse}th,td{border:1px solid #cfd8e3;padding:6px 8px;vertical-align:top}th{text-align:left;font-weight:700;color:#225797;background:#f7fafc;position:sticky;top:0}td{word-break:break-word}.primary-table{table-layout:fixed}</style></head><body><main><div class="report-card"><div class="report-header"><span class="timestamp">{{.GeneratedAt}}</span><div class="title-wrap"><div class="title">{{.Title}}</div><div class="subtitle">{{.SubTitle}}</div></div><span></span></div><p class="summary-line">共 {{len .Rows}} 条扫描结果记录。</p><h1 class="section-title">扫描结果</h1>{{if .Rows}}<div class="table-wrap"><table class="primary-table"><thead><tr><th>Path</th><th>Account Name</th><th>First Name</th><th>Last Name</th><th>E-Mail</th><th>Department</th><th>Division</th><th>Domain</th><th>Trustee</th><th>Trustee SID</th><th>Originating Group</th><th>Rights</th><th>Type</th><th>Inherited</th><th>Applies To</th><th>Source</th></tr></thead><tbody>{{range .Rows}}<tr><td>{{.Path}}</td><td>{{.AccountName}}</td><td>{{.FirstName}}</td><td>{{.LastName}}</td><td>{{.Email}}</td><td>{{.Department}}</td><td>{{.Division}}</td><td>{{.Domain}}</td><td>{{.Trustee}}</td><td>{{.TrusteeSID}}</td><td>{{.OriginatingGroup}}</td><td>{{.Rights}}</td><td>{{.Type}}</td><td>{{.Inherited}}</td><td>{{.AppliesTo}}</td><td>{{.Source}}</td></tr>{{end}}</tbody></table></div>{{else}}<p class="empty">当前没有可导出的扫描结果。</p>{{end}}</div></main></body></html>`

	tmpl, err := template.New("export-scan-results").Parse(doc)
	if err != nil {
		return err
	}

	return tmpl.Execute(file, data)
}

func ensureParentDir(filename string) error {
	parent := filepath.Dir(filename)
	if parent == "." || parent == "" {
		return nil
	}
	return os.MkdirAll(parent, 0o755)
}

func buildReport(permissions []models.Permission, options Options) htmlReport {
	mode := normalizeExportMode(options.Mode)
	rows := cloneUserRows(options.UserRows)
	if len(rows) == 0 {
		rows = buildUserRows(permissions)
	}
	clonedPermissions := append([]models.Permission(nil), permissions...)
	sort.Slice(clonedPermissions, func(i, j int) bool {
		leftPath := strings.ToLower(strings.TrimSpace(clonedPermissions[i].Path))
		rightPath := strings.ToLower(strings.TrimSpace(clonedPermissions[j].Path))
		if leftPath != rightPath {
			return leftPath < rightPath
		}
		return strings.ToLower(strings.TrimSpace(clonedPermissions[i].Trustee)) < strings.ToLower(strings.TrimSpace(clonedPermissions[j].Trustee))
	})

	return htmlReport{
		ReportTitle:    reportDocumentTitle(options),
		ReportSubTitle: reportSubTitle(mode, options),
		Mode:           mode,
		GeneratedAt:    formatExportTimestamp(time.Now()),
		UserRows:       exportPresentationRows(rows),
		Permissions:    clonedPermissions,
	}
}

func normalizeExportMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "scan-results":
		return "scan-results"
	default:
		return "management-summary"
	}
}

func buildScanResultRows(permissions []models.Permission) []scanResultRow {
	cloned := append([]models.Permission(nil), permissions...)
	sort.Slice(cloned, func(i, j int) bool {
		leftPath := strings.ToLower(strings.TrimSpace(cloned[i].Path))
		rightPath := strings.ToLower(strings.TrimSpace(cloned[j].Path))
		if leftPath != rightPath {
			return leftPath < rightPath
		}
		leftTrustee := strings.ToLower(strings.TrimSpace(cloned[i].Trustee))
		rightTrustee := strings.ToLower(strings.TrimSpace(cloned[j].Trustee))
		if leftTrustee != rightTrustee {
			return leftTrustee < rightTrustee
		}
		return strings.ToLower(strings.TrimSpace(cloned[i].Rights)) < strings.ToLower(strings.TrimSpace(cloned[j].Rights))
	})

	rows := make([]scanResultRow, 0, len(cloned))
	for _, permission := range cloned {
		rows = append(rows, scanResultRow{
			Path:             strings.TrimSpace(permission.Path),
			AccountName:      fallback(strings.TrimSpace(permission.AccountName), extractAccountName(permission.Trustee), strings.TrimSpace(permission.TrusteeSID)),
			FirstName:        strings.TrimSpace(permission.FirstName),
			LastName:         strings.TrimSpace(permission.LastName),
			Email:            strings.TrimSpace(permission.Email),
			Department:       strings.TrimSpace(permission.Department),
			Division:         strings.TrimSpace(permission.Division),
			Domain:           deriveDomain(permission),
			Trustee:          strings.TrimSpace(permission.Trustee),
			TrusteeSID:       strings.TrimSpace(permission.TrusteeSID),
			OriginatingGroup: deriveOriginatingGroup(permission),
			Rights:           translateRightsZh(permission.Rights),
			Type:             translateTypeZh(permission.Type),
			Inherited:        translateBoolZh(permission.Inherited),
			AppliesTo:        translateAppliesToZh(permission.AppliesTo),
			Source:           strings.TrimSpace(permission.Source),
		})
	}

	return rows
}

func exportPresentationRows(rows []UserRow) []UserRow {
	if len(rows) == 0 {
		return nil
	}

	type aggregate struct {
		representative UserRow
		permissions    []string
		groups         []string
		hierarchies    []string
	}

	aggregated := make(map[string]*aggregate, len(rows))
	for _, row := range rows {
		key := strings.ToLower(strings.Join([]string{
			strings.TrimSpace(fallback(row.AccountName, row.Trustee, row.TrusteeSID)),
			strings.TrimSpace(row.FirstName),
			strings.TrimSpace(row.LastName),
			strings.TrimSpace(row.Email),
			strings.TrimSpace(row.Department),
			strings.TrimSpace(row.Division),
			strings.TrimSpace(row.Domain),
			strings.TrimSpace(row.OriginatingGroup),
			strings.TrimSpace(row.Permissions),
			strings.TrimSpace(row.GroupInheritanceHierarchy),
		}, "::"))

		current, found := aggregated[key]
		if !found {
			aggregated[key] = &aggregate{
				representative: row,
				permissions:    []string{row.Permissions},
				groups:         []string{row.OriginatingGroup},
				hierarchies:    []string{row.GroupInheritanceHierarchy},
			}
			continue
		}

		current.permissions = append(current.permissions, row.Permissions)
		current.groups = append(current.groups, row.OriginatingGroup)
		current.hierarchies = append(current.hierarchies, row.GroupInheritanceHierarchy)
		if profileRichnessForUserRow(row) > profileRichnessForUserRow(current.representative) {
			current.representative = row
		}
	}

	result := make([]UserRow, 0, len(aggregated))
	for _, item := range aggregated {
		merged := item.representative
		merged.OriginatingGroup = joinDistinctStrings(item.groups, " / ")
		merged.Permissions = joinDistinctStrings(item.permissions, " / ")
		merged.GroupInheritanceHierarchy = joinDistinctStrings(item.hierarchies, " / ")
		result = append(result, merged)
	}

	sort.Slice(result, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(fallback(result[i].AccountName, result[i].Trustee, result[i].TrusteeSID)))
		rightName := strings.ToLower(strings.TrimSpace(fallback(result[j].AccountName, result[j].Trustee, result[j].TrusteeSID)))
		if leftName != rightName {
			return leftName < rightName
		}
		leftGroup := strings.ToLower(strings.TrimSpace(result[i].OriginatingGroup))
		rightGroup := strings.ToLower(strings.TrimSpace(result[j].OriginatingGroup))
		return leftGroup < rightGroup
	})

	return result
}

func profileRichnessForUserRow(row UserRow) int {
	fields := []string{
		strings.TrimSpace(row.AccountName),
		strings.TrimSpace(row.FirstName),
		strings.TrimSpace(row.LastName),
		strings.TrimSpace(row.Email),
		strings.TrimSpace(row.Department),
		strings.TrimSpace(row.Division),
		strings.TrimSpace(row.Domain),
		strings.TrimSpace(row.OriginatingGroup),
		strings.TrimSpace(row.GroupInheritanceHierarchy),
	}
	score := 0
	for _, field := range fields {
		if field != "" {
			score++
		}
	}
	return score
}

func cloneUserRows(rows []UserRow) []UserRow {
	if len(rows) == 0 {
		return nil
	}

	cloned := make([]UserRow, len(rows))
	for index, row := range rows {
		cloned[index] = row
		if len(row.MemberKeys) > 0 {
			cloned[index].MemberKeys = append([]string(nil), row.MemberKeys...)
		}
	}

	return cloned
}

func buildUserRows(permissions []models.Permission) []UserRow {
	type groupedUserPermissions struct {
		representative models.Permission
		permissions    []models.Permission
	}

	grouped := make(map[string]*groupedUserPermissions, len(permissions))
	for _, permission := range permissions {
		key := exportUserRowKey(permission)
		current, found := grouped[key]
		if !found {
			grouped[key] = &groupedUserPermissions{representative: permission, permissions: []models.Permission{permission}}
			continue
		}
		current.permissions = append(current.permissions, permission)
		if profileRichnessForPermission(permission) > profileRichnessForPermission(current.representative) {
			current.representative = permission
		}
	}

	rows := make([]UserRow, 0, len(grouped))
	for _, group := range grouped {
		representative := group.representative
		permissionLabels := make([]string, 0, len(group.permissions))
		originatingGroups := make([]string, 0, len(group.permissions))
		inheritanceHierarchy := make([]string, 0, len(group.permissions))
		appliesTo := make([]string, 0, len(group.permissions))
		explicitCount := 0

		for _, permission := range group.permissions {
			permissionLabels = append(permissionLabels, fallback(formatPermissionDisplay(permission), strings.TrimSpace(permission.Rights)))
			originatingGroups = append(originatingGroups, deriveOriginatingGroup(permission))
			inheritanceHierarchy = append(inheritanceHierarchy, deriveGroupInheritanceHierarchy(permission))
			appliesTo = append(appliesTo, strings.TrimSpace(permission.AppliesTo))
			if !permission.Inherited {
				explicitCount++
			}
		}

		inheritedCount := len(group.permissions) - explicitCount
		inheritanceSummary := "-"
		switch {
		case explicitCount > 0 && inheritedCount > 0:
			inheritanceSummary = "Mixed"
		case explicitCount > 0:
			inheritanceSummary = "Explicit"
		case inheritedCount > 0:
			inheritanceSummary = "Inherited"
		}

		rows = append(rows, UserRow{
			Path:                      strings.TrimSpace(representative.Path),
			Trustee:                   strings.TrimSpace(representative.Trustee),
			TrusteeSID:                strings.TrimSpace(representative.TrusteeSID),
			AccountName:               fallback(strings.TrimSpace(representative.AccountName), extractAccountName(representative.Trustee), strings.TrimSpace(representative.TrusteeSID)),
			FirstName:                 strings.TrimSpace(representative.FirstName),
			LastName:                  strings.TrimSpace(representative.LastName),
			Email:                     strings.TrimSpace(representative.Email),
			Department:                strings.TrimSpace(representative.Department),
			Division:                  strings.TrimSpace(representative.Division),
			Domain:                    deriveDomain(representative),
			OriginatingGroup:          joinDistinctStrings(originatingGroups, " / "),
			Permissions:               joinDistinctStrings(permissionLabels, " / "),
			GroupInheritanceHierarchy: joinDistinctStrings(inheritanceHierarchy, " / "),
			PermissionCount:           len(dedupeNonEmptyStrings(permissionLabels)),
			AppliesToSummary:          joinDistinctStrings(appliesTo, " · "),
			InheritanceSummary:        inheritanceSummary,
			RowCount:                  len(group.permissions),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(rows[i].AccountName))
		rightName := strings.ToLower(strings.TrimSpace(rows[j].AccountName))
		if leftName != rightName {
			return leftName < rightName
		}
		return strings.ToLower(strings.TrimSpace(rows[i].OriginatingGroup)) < strings.ToLower(strings.TrimSpace(rows[j].OriginatingGroup))
	})

	return rows
}

func exportUserRowKey(permission models.Permission) string {
	return strings.ToLower(strings.Join([]string{
		strings.TrimSpace(permission.Path),
		deriveDomain(permission),
		fallback(strings.TrimSpace(permission.AccountName), extractAccountName(permission.Trustee), strings.TrimSpace(permission.TrusteeSID)),
	}, "::"))
}

func profileRichnessForPermission(permission models.Permission) int {
	fields := []string{
		strings.TrimSpace(permission.AccountName),
		strings.TrimSpace(permission.FirstName),
		strings.TrimSpace(permission.LastName),
		strings.TrimSpace(permission.Email),
		strings.TrimSpace(permission.Department),
		strings.TrimSpace(permission.Division),
		strings.TrimSpace(permission.Domain),
		strings.TrimSpace(permission.OriginatingGroup),
		strings.TrimSpace(permission.GroupInheritanceHierarchy),
	}
	score := 0
	for _, field := range fields {
		if field != "" {
			score++
		}
	}
	return score
}

func joinDistinctStrings(values []string, separator string) string {
	if strings.TrimSpace(separator) == "" {
		separator = " / "
	}
	items := dedupeNonEmptyStrings(values)
	if len(items) == 0 {
		return ""
	}
	return strings.Join(items, separator)
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func fallback(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func extractAccountName(trustee string) string {
	trimmed := strings.TrimSpace(trustee)
	if i := strings.LastIndex(trimmed, `\`); i >= 0 && i < len(trimmed)-1 {
		return trimmed[i+1:]
	}
	return trimmed
}

func deriveDomain(permission models.Permission) string {
	if value := strings.TrimSpace(permission.Domain); value != "" {
		return value
	}
	if trustee := strings.TrimSpace(permission.Trustee); trustee != "" {
		if i := strings.LastIndex(trustee, `\`); i > 0 {
			return strings.ToUpper(strings.TrimSpace(trustee[:i]))
		}
	}
	if email := strings.TrimSpace(permission.Email); email != "" {
		if i := strings.LastIndex(email, "@"); i >= 0 && i < len(email)-1 {
			return strings.ToUpper(strings.TrimSpace(email[i+1:]))
		}
	}
	return ""
}

func deriveOriginatingGroup(permission models.Permission) string {
	if value := strings.TrimSpace(permission.OriginatingGroup); value != "" {
		return value
	}
	if via := extractEffectiveGroupFromSource(permission.Source); via != "" {
		return via
	}
	return strings.TrimSpace(permission.Trustee)
}

func deriveGroupInheritanceHierarchy(permission models.Permission) string {
	if value := strings.TrimSpace(permission.GroupInheritanceHierarchy); value != "" {
		return value
	}
	if via := extractEffectiveGroupFromSource(permission.Source); via != "" {
		return via
	}
	return ""
}

func deriveParentDelta(permission models.Permission) string {
	if value := strings.TrimSpace(permission.ParentDelta); value != "" {
		return value
	}
	if permission.Inherited {
		return "Inherited from Parent"
	}
	appliesTo := strings.ToLower(strings.TrimSpace(permission.AppliesTo))
	if strings.Contains(appliesTo, "subfolders") || strings.Contains(appliesTo, "files") {
		return "Explicit Inheritance Override"
	}
	return "Explicit on Current Item"
}

func extractEffectiveGroupFromSource(source string) string {
	trimmed := strings.TrimSpace(source)
	marker := "effective via "
	if index := strings.LastIndex(strings.ToLower(trimmed), marker); index >= 0 {
		return strings.TrimSpace(trimmed[index+len(marker):])
	}
	return ""
}

func reportBrandName() string {
	if value := strings.TrimSpace(os.Getenv("REPORT_BRAND_NAME")); value != "" {
		return value
	}
	return "OpenAD"
}

func reportDocumentTitle(options Options) string {
	if value := strings.TrimSpace(options.Title); value != "" {
		return value
	}
	return fmt.Sprintf("%s 权限报告", reportBrandName())
}

func reportSubTitle(mode string, options Options) string {
	if mode == "scan-results" {
		return "扫描结果明细"
	}

	templateID := strings.ToLower(strings.TrimSpace(options.Template))
	switch templateID {
	case "management":
		return "用户权限汇总"
	case "compliance":
		return "权限合规视图"
	case "operations":
		return "运行权限视图"
	default:
		return "用户权限汇总"
	}
}

func formatExportTimestamp(value time.Time) string {
	return value.Format("2006/1/2 15:04")
}

func translateBoolZh(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func translateTypeZh(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allow":
		return "允许"
	case "deny":
		return "拒绝"
	default:
		return strings.TrimSpace(value)
	}
}

func translateAccountTypeZh(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user":
		return "用户"
	case "group":
		return "组"
	case "domain":
		return "域"
	case "alias":
		return "别名"
	case "wellknowngroup":
		return "内置组"
	case "deletedaccount":
		return "已删除账户"
	case "computer":
		return "计算机"
	case "label":
		return "标签"
	default:
		return strings.TrimSpace(value)
	}
}

func translateAppliesToZh(value string) string {
	replacer := strings.NewReplacer(
		"This Folder, Subfolders and Files", "此文件夹、子文件夹和文件",
		"This Folder and Subfolders", "此文件夹和子文件夹",
		"This Folder and Files", "此文件夹和文件",
		"This Folder Only", "仅此文件夹",
		"Subfolders and Files Only", "仅子文件夹和文件",
		"Subfolders Only", "仅子文件夹",
		"Files Only", "仅文件",
		"(No Propagate)", "（不传播）",
	)
	return replacer.Replace(strings.TrimSpace(value))
}

func translateParentDeltaZh(value string) string {
	switch strings.TrimSpace(value) {
	case "Inherited from Parent":
		return "继承自父级"
	case "Explicit Inheritance Override":
		return "显式覆盖继承"
	case "Explicit on Current Item":
		return "仅当前项显式存在"
	default:
		return strings.TrimSpace(value)
	}
}

func translateRightsZh(value string) string {
	replacer := strings.NewReplacer(
		"Read and Execute", "读取和执行",
		"ReadAndExecute", "读取和执行",
		"Full Control", "完全控制",
		"FullControl", "完全控制",
		"Modify", "修改",
		"Read", "读取",
		"Write", "写入",
		"Execute", "执行",
		"Delete", "删除",
		"Take Ownership", "取得所有权",
		"Change Permissions", "更改权限",
		"Synchronize", "同步",
	)
	return replacer.Replace(strings.TrimSpace(value))
}

func formatPermissionDisplay(permission models.Permission) string {
	typeLabel := translateTypeZh(permission.Type)
	rightsLabel := translateRightsZh(permission.Rights)
	appliesLabel := translateAppliesToZh(permission.AppliesTo)
	parts := make([]string, 0, 2)
	if typeLabel != "" && rightsLabel != "" {
		parts = append(parts, typeLabel+": "+rightsLabel)
	} else if rightsLabel != "" {
		parts = append(parts, rightsLabel)
	}
	if appliesLabel != "" {
		parts = append(parts, appliesLabel)
	}
	return strings.Join(parts, ", ")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
