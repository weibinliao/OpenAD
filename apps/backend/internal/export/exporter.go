//go:build ignore
// +build ignore

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

type exportUserRow struct {
	AccountName               string
	FirstName                 string
	LastName                  string
	Email                     string
	Department                string
	Division                  string
	Domain                    string
	OriginatingGroup          string
	Permissions               string
	GroupInheritanceHierarchy string
	Path                      string
	EntryType                 string
	Inherited                 string
	Source                    string
	AppliesTo                 string
	AccountType               string
	RiskLevel                 string
	ParentDelta               string
}

type trusteeSummary struct {
	Name  string
	Count int
}

type pathSummary struct {
	Path  string
	Count int
}

type htmlReport struct {
	BrandName      string
	ReportTitle    string
	GeneratedAt    string
	TotalCount     int
	ExplicitCount  int
	InheritedCount int
	UniquePaths    int
	UniqueTrustees int
	DenyCount      int
	HighRiskCount  int
	TopTrustees    []trusteeSummary
	TopPaths       []pathSummary
	UserRows       []exportUserRow
	Permissions    []models.Permission
}

func NewExporter() *Exporter {
	return &Exporter{}
}

func (e *Exporter) ExportToCSV(permissions []models.Permission, filename string) error {
	if err := ensureParentDir(filename); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	report := buildHTMLReport(permissions)
	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{report.ReportTitle}); err != nil {
		return err
	}
	if err := writer.Write([]string{
		"Generated At",
		"Total Entries",
		"Explicit",
		"Inherited",
		"Unique Paths",
		"Unique Trustees",
		"High Risk",
		"Deny",
	}); err != nil {
		return err
	}
	if err := writer.Write([]string{
		report.GeneratedAt,
		fmt.Sprint(report.TotalCount),
		fmt.Sprint(report.ExplicitCount),
		fmt.Sprint(report.InheritedCount),
		fmt.Sprint(report.UniquePaths),
		fmt.Sprint(report.UniqueTrustees),
		fmt.Sprint(report.HighRiskCount),
		fmt.Sprint(report.DenyCount),
	}); err != nil {
		return err
	}
	if err := writer.Write([]string{}); err != nil {
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
		"Path",
		"Entry Type",
		"Inherited",
		"Source",
		"Applies To",
		"Account Type",
		"Risk Level",
		"Parent Delta",
	}
	if err := writer.Write(headers); err != nil {
		return err
	}

	for _, row := range report.UserRows {
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
			row.Path,
			row.EntryType,
			row.Inherited,
			row.Source,
			row.AppliesTo,
			row.AccountType,
			row.RiskLevel,
			row.ParentDelta,
		}); err != nil {
			return err
		}
	}

	return writer.Error()
}

func (e *Exporter) ExportToExcel(permissions []models.Permission, filename string) error {
	if err := ensureParentDir(filename); err != nil {
		return err
	}

	report := buildHTMLReport(permissions)
	workbook := excelize.NewFile()
	defer workbook.Close()

	const (
		overviewSheet = "概览"
		userSheet     = "用户权限"
		aclSheet      = "ACL 明细"
	)

	workbook.SetSheetName("Sheet1", overviewSheet)
	_, _ = workbook.NewSheet(userSheet)
	_, _ = workbook.NewSheet(aclSheet)

	titleStyle, _ := workbook.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "#16324F"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerStyle, _ := workbook.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#F8FBFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#16324F"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	cardStyle, _ := workbook.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#EDF3FB"}},
		Border:    []excelize.Border{{Type: "left", Color: "#2A6EF0", Style: 2}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	textWrapStyle, _ := workbook.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	monoWrapStyle, _ := workbook.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Consolas", Size: 10},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})

	_ = workbook.MergeCell(overviewSheet, "A1", "H1")
	workbook.SetCellValue(overviewSheet, "A1", report.ReportTitle)
	workbook.SetCellValue(overviewSheet, "A2", "Generated At")
	workbook.SetCellValue(overviewSheet, "B2", report.GeneratedAt)
	workbook.SetCellValue(overviewSheet, "A4", "Metric")
	workbook.SetCellValue(overviewSheet, "B4", "Value")
	workbook.SetCellValue(overviewSheet, "D4", "Top Trustee")
	workbook.SetCellValue(overviewSheet, "E4", "Entries")
	workbook.SetCellValue(overviewSheet, "G4", "Top Path")
	workbook.SetCellValue(overviewSheet, "H4", "Entries")

	overviewMetrics := [][2]any{
		{"Total Entries", report.TotalCount},
		{"Explicit", report.ExplicitCount},
		{"Inherited", report.InheritedCount},
		{"Unique Paths", report.UniquePaths},
		{"Unique Trustees", report.UniqueTrustees},
		{"High Risk", report.HighRiskCount},
		{"Deny", report.DenyCount},
	}
	for index, metric := range overviewMetrics {
		rowNumber := index + 5
		workbook.SetCellValue(overviewSheet, fmt.Sprintf("A%d", rowNumber), metric[0])
		workbook.SetCellValue(overviewSheet, fmt.Sprintf("B%d", rowNumber), metric[1])
	}
	for index, item := range report.TopTrustees {
		rowNumber := index + 5
		workbook.SetCellValue(overviewSheet, fmt.Sprintf("D%d", rowNumber), item.Name)
		workbook.SetCellValue(overviewSheet, fmt.Sprintf("E%d", rowNumber), item.Count)
	}
	for index, item := range report.TopPaths {
		rowNumber := index + 5
		workbook.SetCellValue(overviewSheet, fmt.Sprintf("G%d", rowNumber), item.Path)
		workbook.SetCellValue(overviewSheet, fmt.Sprintf("H%d", rowNumber), item.Count)
	}

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
		"Path",
		"Entry Type",
		"Inherited",
		"Source",
		"Applies To",
		"Account Type",
		"Risk Level",
		"Parent Delta",
	}
	_ = workbook.MergeCell(userSheet, "A1", "R1")
	workbook.SetCellValue(userSheet, "A1", report.ReportTitle)
	workbook.SetCellValue(userSheet, "A2", "用户权限报告")
	for index, header := range userHeaders {
		cell, _ := excelize.CoordinatesToCellName(index+1, 4)
		workbook.SetCellValue(userSheet, cell, header)
	}
	for index, row := range report.UserRows {
		values := []any{row.AccountName, row.FirstName, row.LastName, row.Email, row.Department, row.Division, row.Domain, row.OriginatingGroup, row.Permissions, row.GroupInheritanceHierarchy, row.Path, row.EntryType, row.Inherited, row.Source, row.AppliesTo, row.AccountType, row.RiskLevel, row.ParentDelta}
		for colIndex, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, index+5)
			workbook.SetCellValue(userSheet, cell, value)
		}
	}

	aclHeaders := []string{"Account", "Type", "Permission Description", "Inherited", "Applies To", "Account Type", "Risk Level", "Parent Delta", "Path", "SID", "Source Group"}
	_ = workbook.MergeCell(aclSheet, "A1", "K1")
	workbook.SetCellValue(aclSheet, "A1", report.ReportTitle)
	workbook.SetCellValue(aclSheet, "A2", "访问控制附录")
	for index, header := range aclHeaders {
		cell, _ := excelize.CoordinatesToCellName(index+1, 4)
		workbook.SetCellValue(aclSheet, cell, header)
	}
	for index, permission := range report.Permissions {
		values := []any{
			permission.Trustee,
			translateTypeZh(permission.Type),
			formatPermissionDisplay(permission),
			translateBoolZh(permission.Inherited),
			translateAppliesToZh(permission.AppliesTo),
			translateAccountTypeZh(permission.AccountType),
			translateRiskLevelZh(fallback(permission.RiskLevel, inferRiskLevel(permission.Rights))),
			translateParentDeltaZh(fallback(permission.ParentDelta, deriveParentDelta(permission))),
			permission.Path,
			permission.TrusteeSID,
			deriveOriginatingGroup(permission),
		}
		for colIndex, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, index+5)
			workbook.SetCellValue(aclSheet, cell, value)
		}
	}

	_ = workbook.SetCellStyle(overviewSheet, "A1", "H1", titleStyle)
	_ = workbook.SetCellStyle(overviewSheet, "A2", "B2", cardStyle)
	_ = workbook.SetCellStyle(overviewSheet, "A4", "H4", headerStyle)
	_ = workbook.SetCellStyle(userSheet, "A1", "R1", titleStyle)
	_ = workbook.SetCellStyle(userSheet, "A2", "A2", cardStyle)
	_ = workbook.SetCellStyle(userSheet, "A4", "R4", headerStyle)
	_ = workbook.SetCellStyle(aclSheet, "A1", "K1", titleStyle)
	_ = workbook.SetCellStyle(aclSheet, "A2", "A2", cardStyle)
	_ = workbook.SetCellStyle(aclSheet, "A4", "K4", headerStyle)
	_ = workbook.SetCellStyle(userSheet, "A5", "R1048576", textWrapStyle)
	_ = workbook.SetCellStyle(aclSheet, "A5", "K1048576", textWrapStyle)
	_ = workbook.SetCellStyle(aclSheet, "J5", "J1048576", monoWrapStyle)
	_ = workbook.SetPanes(userSheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 4})
	_ = workbook.SetPanes(aclSheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 4})
	_ = workbook.SetColWidth(overviewSheet, "A", "H", 18)
	_ = workbook.SetColWidth(overviewSheet, "G", "G", 48)
	_ = workbook.SetColWidth(userSheet, "A", "H", 18)
	_ = workbook.SetColWidth(userSheet, "I", "I", 40)
	_ = workbook.SetColWidth(userSheet, "J", "J", 30)
	_ = workbook.SetColWidth(userSheet, "K", "K", 48)
	_ = workbook.SetColWidth(userSheet, "N", "O", 24)
	_ = workbook.SetColWidth(userSheet, "P", "R", 18)
	_ = workbook.SetColWidth(aclSheet, "A", "B", 20)
	_ = workbook.SetColWidth(aclSheet, "C", "C", 42)
	_ = workbook.SetColWidth(aclSheet, "D", "H", 18)
	_ = workbook.SetColWidth(aclSheet, "I", "I", 48)
	_ = workbook.SetColWidth(aclSheet, "J", "J", 34)
	_ = workbook.SetColWidth(aclSheet, "K", "K", 24)

	if index, err := workbook.GetSheetIndex(userSheet); err == nil {
		workbook.SetActiveSheet(index)
	}

	return workbook.SaveAs(filename)
}

func (e *Exporter) ExportToHTML(permissions []models.Permission, filename string) error {
	if err := ensureParentDir(filename); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	report := buildHTMLReport(permissions)

	const htmlDocument = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.ReportTitle}}</title>
    <style>
        :root { color-scheme: light; }
        * { box-sizing: border-box; }
        body { margin: 0; font-family: "IBM Plex Sans", "Segoe UI", "Microsoft YaHei", sans-serif; background: radial-gradient(circle at top left, rgba(20,184,166,0.18), transparent 34%), radial-gradient(circle at top right, rgba(37,99,235,0.12), transparent 24%), #edf3f6; color: #112031; }
        main { width: min(1480px, calc(100% - 34px)); margin: 24px auto 54px; }
        .hero, .panel { background: linear-gradient(180deg, rgba(255,255,255,0.94), rgba(248,252,253,0.98)); border: 1px solid rgba(17,32,49,0.08); border-radius: 26px; box-shadow: 0 26px 70px rgba(15,23,42,0.08); }
        .hero { padding: 26px 30px; }
        .hero-kicker { color: #0f766e; text-transform: uppercase; letter-spacing: .18em; font-size: 11px; font-weight: 700; }
        h1 { margin: 12px 0 6px; font-size: 34px; letter-spacing: -0.03em; }
        .meta { color: #587182; font-size: 13px; line-height: 1.7; }
        .metrics { list-style: none; margin: 22px 0 0; padding: 0; display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); }
        .metrics li { border-radius: 18px; padding: 14px 16px; border: 1px solid rgba(17,32,49,0.08); background: rgba(255,255,255,0.76); }
        .metrics strong { display: block; margin-bottom: 8px; color: #597385; font-size: 11px; letter-spacing: .15em; text-transform: uppercase; }
        .metrics span { font-size: 28px; font-weight: 700; font-variant-numeric: tabular-nums; }
        .grid { display: grid; gap: 18px; margin-top: 18px; grid-template-columns: repeat(2, minmax(0, 1fr)); }
        .panel { overflow: hidden; }
        .full { grid-column: 1 / -1; }
        .panel header { padding: 16px 20px; border-bottom: 1px solid rgba(17,32,49,0.08); background: linear-gradient(90deg, rgba(15,118,110,0.08), transparent 34%); }
        .panel header h2 { margin: 0; font-size: 18px; }
        .panel header p { margin: 6px 0 0; color: #587182; font-size: 13px; line-height: 1.6; }
        .table-wrap { overflow: auto; max-height: 760px; }
        table { width: 100%; min-width: 1120px; border-collapse: collapse; }
        th, td { padding: 11px 14px; border-bottom: 1px solid rgba(17,32,49,0.08); text-align: left; vertical-align: top; }
        th { position: sticky; top: 0; background: #e8f5f4; color: #185564; font-size: 12px; letter-spacing: .04em; z-index: 1; }
        td { font-size: 13px; line-height: 1.6; }
        tr:nth-child(even) td { background: rgba(2,132,199,0.03); }
        .summary-list { list-style: none; margin: 0; padding: 18px 20px 22px; display: grid; gap: 10px; }
        .summary-list li { border-radius: 16px; padding: 12px 14px; border: 1px solid rgba(17,32,49,0.08); background: rgba(255,255,255,0.76); }
        .summary-list span { display: block; margin-bottom: 5px; color: #587182; font-size: 12px; }
        .note-grid { display: grid; gap: 12px; padding: 18px 20px 22px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
        .note-card { border-radius: 16px; padding: 14px 16px; border: 1px solid rgba(17,32,49,0.08); background: rgba(255,255,255,0.76); }
        .note-card strong { display: block; margin-bottom: 8px; }
        .badge { display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 999px; font-size: 12px; border: 1px solid rgba(15,118,110,0.18); background: rgba(15,118,110,0.08); color: #0f766e; }
        .badge.high { border-color: rgba(194,65,12,0.2); background: rgba(194,65,12,0.08); color: #c2410c; }
        .badge.medium { border-color: rgba(161,98,7,0.2); background: rgba(161,98,7,0.08); color: #a16207; }
        .mono { font-family: "IBM Plex Mono", Consolas, monospace; font-size: 12px; }
        .empty { padding: 22px; color: #587182; }
        @media (max-width: 960px) {
            main { width: min(100%, calc(100% - 20px)); }
            .hero { padding: 20px; }
            .grid { grid-template-columns: 1fr; }
            h1 { font-size: 28px; }
        }
    </style>
</head>
<body>
    <main>
        <section class="hero">
            <div class="hero-kicker">{{.BrandName}}</div>
            <h1>{{.ReportTitle}}</h1>
            <div class="meta">
                ????: {{.GeneratedAt}}<br>
                ???????????????????????????????????????
            </div>
            <ul class="metrics">
                <li><strong>????</strong><span>{{.TotalCount}}</span></li>
                <li><strong>????</strong><span>{{.ExplicitCount}}</span></li>
                <li><strong>????</strong><span>{{.InheritedCount}}</span></li>
                <li><strong>????</strong><span>{{.UniquePaths}}</span></li>
                <li><strong>????</strong><span>{{.UniqueTrustees}}</span></li>
                <li><strong>???</strong><span>{{.HighRiskCount}}</span></li>
            </ul>
        </section>
        <div class="grid">
            <section class="panel">
                <header><h2>????</h2><p>???????????????????????????????????</p></header>
                {{if .TopTrustees}}<ul class="summary-list">{{range .TopTrustees}}<li><span>Trustee</span><strong>{{.Name}}</strong><div>{{.Count}} entries</div></li>{{end}}</ul>{{else}}<div class="empty">???????</div>{{end}}
            </section>
            <section class="panel">
                <header><h2>????</h2><p>ACL ????????????????????????????????</p></header>
                {{if .TopPaths}}<ul class="summary-list">{{range .TopPaths}}<li><span>Path</span><strong>{{.Path}}</strong><div>{{.Count}} entries</div></li>{{end}}</ul>{{else}}<div class="empty">???????</div>{{end}}
            </section>
            <section class="panel full">
                <header><h2>??????</h2><p>???????????????????????????????????????????</p></header>
                <div class="table-wrap">{{if .UserRows}}<table><thead><tr><th>Account Name</th><th>First Name</th><th>Last Name</th><th>E-Mail</th><th>Department</th><th>Division</th><th>Domain</th><th>Originating Group</th><th>Permissions</th><th>Applies To</th><th>Risk Level</th><th>Path</th><th>Parent Delta</th></tr></thead><tbody>{{range .UserRows}}<tr><td>{{.AccountName}}</td><td>{{.FirstName}}</td><td>{{.LastName}}</td><td>{{.Email}}</td><td>{{.Department}}</td><td>{{.Division}}</td><td>{{.Domain}}</td><td>{{.OriginatingGroup}}</td><td>{{.Permissions}}</td><td>{{.AppliesTo}}</td><td><span class="badge {{riskClass .RiskLevel}}">{{.RiskLevel}}</span></td><td>{{.Path}}</td><td>{{.ParentDelta}}</td></tr>{{end}}</tbody></table>{{else}}<div class="empty">???????????????</div>{{end}}</div>
            </section>
            <section class="panel full">
                <header><h2>??????</h2><p>?? ACL ?????????????????????????????????????????</p></header>
                <div class="table-wrap">{{if .Permissions}}<table><thead><tr><th>Account</th><th>Type</th><th>Permission Description</th><th>Inherited</th><th>Applies To</th><th>Account Type</th><th>Risk Level</th><th>Parent Delta</th><th>Path</th><th>SID</th><th>Source Group</th></tr></thead><tbody>{{range .Permissions}}<tr><td>{{.Trustee}}</td><td>{{typeZh .Type}}</td><td>{{permissionText .}}</td><td>{{boolZh .Inherited}}</td><td>{{appliesToZh .AppliesTo}}</td><td>{{accountTypeZh .AccountType}}</td><td><span class="badge {{riskClass (fallback .RiskLevel (inferRiskLevel .Rights))}}">{{riskZh (fallback .RiskLevel (inferRiskLevel .Rights))}}</span></td><td>{{parentDeltaZh (fallback .ParentDelta (parentDelta .))}}</td><td>{{.Path}}</td><td class="mono">{{.TrusteeSID}}</td><td>{{originatingGroup .}}</td></tr>{{end}}</tbody></table>{{else}}<div class="empty">???????? ACL ???</div>{{end}}</div>
            </section>
            <section class="panel full">
                <header><h2>????</h2><p>????????????????????????????????????</p></header>
                <div class="note-grid"><div class="note-card"><strong>?????</strong>?? Full Control?Modify?Write?Delete?Change Permissions?Take Ownership ??????????????</div><div class="note-card"><strong>?????</strong>????????????????????????????????</div><div class="note-card"><strong>???</strong>???????? effective via ??????????????????????????????</div></div>
            </section>
        </div>
    </main>
</body>
</html>`

	tmpl, err := template.New("export").Funcs(template.FuncMap{
		"accountTypeZh":    translateAccountTypeZh,
		"appliesToZh":      translateAppliesToZh,
		"boolZh":           translateBoolZh,
		"fallback":         fallback,
		"inferRiskLevel":   inferRiskLevel,
		"originatingGroup": deriveOriginatingGroup,
		"parentDelta":      deriveParentDelta,
		"parentDeltaZh":    translateParentDeltaZh,
		"permissionText":   formatPermissionDisplay,
		"riskClass":        riskClass,
		"riskZh":           translateRiskLevelZh,
		"typeZh":           translateTypeZh,
	}).Parse(htmlDocument)
	if err != nil {
		return err
	}

	return tmpl.Execute(file, report)
}

func ensureParentDir(filename string) error {
	parent := filepath.Dir(filename)
	if parent == "." || parent == "" {
		return nil
	}
	return os.MkdirAll(parent, 0o755)
}

func buildHTMLReport(permissions []models.Permission) htmlReport {
	trusteeCounts := map[string]int{}
	pathCounts := map[string]int{}
	uniquePaths := map[string]struct{}{}
	uniqueTrustees := map[string]struct{}{}
	explicitCount, inheritedCount, denyCount, highRiskCount := 0, 0, 0, 0

	for _, permission := range permissions {
		if trustee := strings.TrimSpace(permission.Trustee); trustee != "" {
			trusteeCounts[trustee]++
			uniqueTrustees[trustee] = struct{}{}
		}
		if path := strings.TrimSpace(permission.Path); path != "" {
			pathCounts[path]++
			uniquePaths[path] = struct{}{}
		}
		if permission.Inherited {
			inheritedCount++
		} else {
			explicitCount++
		}
		if strings.EqualFold(strings.TrimSpace(permission.Type), "deny") {
			denyCount++
		}
		if inferRiskLevel(permission.Rights) == "high" {
			highRiskCount++
		}
	}

	report := htmlReport{
		BrandName:      reportBrandName(),
		ReportTitle:    reportDocumentTitle(),
		GeneratedAt:    formatExportTimestamp(time.Now()),
		TotalCount:     len(permissions),
		ExplicitCount:  explicitCount,
		InheritedCount: inheritedCount,
		UniquePaths:    len(uniquePaths),
		UniqueTrustees: len(uniqueTrustees),
		DenyCount:      denyCount,
		HighRiskCount:  highRiskCount,
		TopTrustees:    summarizeTopTrustees(trusteeCounts, 6),
		TopPaths:       summarizeTopPaths(pathCounts, 6),
		UserRows:       buildUserRows(permissions),
		Permissions:    append([]models.Permission(nil), permissions...),
	}

	sort.Slice(report.Permissions, func(i, j int) bool {
		leftRisk := riskRank(inferRiskLevel(report.Permissions[i].Rights))
		rightRisk := riskRank(inferRiskLevel(report.Permissions[j].Rights))
		if leftRisk != rightRisk {
			return leftRisk > rightRisk
		}
		if report.Permissions[i].Path != report.Permissions[j].Path {
			return strings.ToLower(report.Permissions[i].Path) < strings.ToLower(report.Permissions[j].Path)
		}
		return strings.ToLower(report.Permissions[i].Trustee) < strings.ToLower(report.Permissions[j].Trustee)
	})

	return report
}

func buildUserRows(permissions []models.Permission) []exportUserRow {
	rows := make([]exportUserRow, 0, len(permissions))
	for _, permission := range permissions {
		rows = append(rows, exportUserRow{
			AccountName:               fallback(permission.AccountName, extractAccountName(permission.Trustee)),
			FirstName:                 strings.TrimSpace(permission.FirstName),
			LastName:                  strings.TrimSpace(permission.LastName),
			Email:                     strings.TrimSpace(permission.Email),
			Department:                strings.TrimSpace(permission.Department),
			Division:                  strings.TrimSpace(permission.Division),
			Domain:                    deriveDomain(permission),
			OriginatingGroup:          deriveOriginatingGroup(permission),
			Permissions:               fallback(strings.TrimSpace(permission.Rights), formatPermissionDisplay(permission)),
			GroupInheritanceHierarchy: deriveGroupInheritanceHierarchy(permission),
			Path:                      strings.TrimSpace(permission.Path),
			EntryType:                 fallback(strings.TrimSpace(permission.Type), "Allow"),
			Inherited:                 boolWord(permission.Inherited),
			Source:                    deriveSource(permission),
			AppliesTo:                 strings.TrimSpace(permission.AppliesTo),
			AccountType:               strings.TrimSpace(permission.AccountType),
			RiskLevel:                 strings.ToUpper(fallback(strings.TrimSpace(permission.RiskLevel), inferRiskLevel(permission.Rights))),
			ParentDelta:               fallback(strings.TrimSpace(permission.ParentDelta), deriveParentDelta(permission)),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if riskRank(rows[i].RiskLevel) != riskRank(rows[j].RiskLevel) {
			return riskRank(rows[i].RiskLevel) > riskRank(rows[j].RiskLevel)
		}
		if rows[i].Path != rows[j].Path {
			return strings.ToLower(rows[i].Path) < strings.ToLower(rows[j].Path)
		}
		return strings.ToLower(rows[i].AccountName) < strings.ToLower(rows[j].AccountName)
	})

	return rows
}

func fallback(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func extractAccountName(trustee string) string {
	trimmed := strings.TrimSpace(trustee)
	if trimmed == "" {
		return ""
	}
	if index := strings.LastIndex(trimmed, `\`); index >= 0 && index < len(trimmed)-1 {
		return trimmed[index+1:]
	}
	return trimmed
}

func deriveDomain(permission models.Permission) string {
	if value := strings.TrimSpace(permission.Domain); value != "" {
		return value
	}
	if trustee := strings.TrimSpace(permission.Trustee); trustee != "" {
		if index := strings.LastIndex(trustee, `\`); index > 0 {
			return strings.ToUpper(strings.TrimSpace(trustee[:index]))
		}
	}
	if email := strings.TrimSpace(permission.Email); email != "" {
		if index := strings.LastIndex(email, "@"); index >= 0 && index < len(email)-1 {
			return strings.ToUpper(strings.TrimSpace(email[index+1:]))
		}
	}
	return ""
}

func deriveOriginatingGroup(permission models.Permission) string {
	if value := strings.TrimSpace(permission.OriginatingGroup); value != "" {
		return value
	}
	source := strings.TrimSpace(permission.Source)
	marker := "effective via "
	if index := strings.LastIndex(strings.ToLower(source), marker); index >= 0 {
		return strings.TrimSpace(source[index+len(marker):])
	}
	return source
}

func deriveGroupInheritanceHierarchy(permission models.Permission) string {
	if value := strings.TrimSpace(permission.GroupInheritanceHierarchy); value != "" {
		return value
	}
	if source := strings.TrimSpace(permission.Source); source != "" {
		return source
	}
	if permission.Inherited {
		return "Inherited from parent"
	}
	return "Direct assignment"
}

func deriveSource(permission models.Permission) string {
	if value := strings.TrimSpace(permission.Source); value != "" {
		return value
	}
	if permission.Inherited {
		return "Inherited"
	}
	return "Explicit"
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

func formatPermissionDisplay(permission models.Permission) string {
	typeLabel := translateTypeZh(permission.Type)
	rightsLabel := translateRightsZh(permission.Rights)
	appliesToLabel := translateAppliesToZh(permission.AppliesTo)
	parts := make([]string, 0, 2)
	if typeLabel != "" && rightsLabel != "" {
		parts = append(parts, typeLabel+": "+rightsLabel)
	} else if rightsLabel != "" {
		parts = append(parts, rightsLabel)
	}
	if appliesToLabel != "" {
		parts = append(parts, appliesToLabel)
	}
	return strings.Join(parts, ", ")
}

func reportBrandName() string {
	if value := strings.TrimSpace(os.Getenv("REPORT_BRAND_NAME")); value != "" {
		return value
	}
	return "OpenAD"
}

func reportDocumentTitle() string {
	return fmt.Sprintf("%s 权限报告", reportBrandName())
}

func formatExportTimestamp(value time.Time) string {
	return value.Format("2006/1/2 15:04")
}

func inferRiskLevel(rights string) string {
	if isHighRiskPermission(rights) {
		return "high"
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(rights)), "execute") {
		return "medium"
	}
	return "low"
}

func translateBoolZh(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func boolWord(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
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

func translateRiskLevelZh(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
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
	result := strings.TrimSpace(value)
	for _, replacement := range []struct{ from, to string }{
		{"This Folder, Subfolders and Files", "此文件夹、子文件夹和文件"},
		{"This Folder and Subfolders", "此文件夹和子文件夹"},
		{"This Folder and Files", "此文件夹和文件"},
		{"This Folder Only", "仅此文件夹"},
		{"Subfolders and Files Only", "仅子文件夹和文件"},
		{"Subfolders Only", "仅子文件夹"},
		{"Files Only", "仅文件"},
		{"(No Propagate)", "（不传播）"},
	} {
		result = strings.ReplaceAll(result, replacement.from, replacement.to)
	}
	return result
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
	result := strings.TrimSpace(value)
	for _, replacement := range []struct{ from, to string }{
		{"Read and Execute", "读取和执行"},
		{"ReadAndExecute", "读取和执行"},
		{"Full Control", "完全控制"},
		{"FullControl", "完全控制"},
		{"Modify", "修改"},
		{"Read", "读取"},
		{"Write", "写入"},
		{"Execute", "执行"},
		{"Delete", "删除"},
		{"Take Ownership", "取得所有权"},
		{"Change Permissions", "更改权限"},
		{"Synchronize", "同步"},
	} {
		result = strings.ReplaceAll(result, replacement.from, replacement.to)
	}
	return result
}

func isHighRiskPermission(rights string) bool {
	value := strings.ToLower(strings.TrimSpace(rights))
	for _, token := range []string{"full control", "modify", "write", "delete", "change permissions", "take ownership"} {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func summarizeTopTrustees(counts map[string]int, limit int) []trusteeSummary {
	items := make([]trusteeSummary, 0, len(counts))
	for trustee, count := range counts {
		items = append(items, trusteeSummary{Name: trustee, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func summarizeTopPaths(counts map[string]int, limit int) []pathSummary {
	items := make([]pathSummary, 0, len(counts))
	for path, count := range counts {
		items = append(items, pathSummary{Path: path, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return strings.ToLower(items[i].Path) < strings.ToLower(items[j].Path)
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func riskClass(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high", "高":
		return "high"
	case "medium", "中":
		return "medium"
	default:
		return "low"
	}
}

func riskRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW":
		return 1
	}
	return 0
}

/*
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

type exportUserRow struct {
	AccountName               string
	FirstName                 string
	LastName                  string
	Email                     string
	Department                string
	Division                  string
	Domain                    string
	OriginatingGroup          string
	Permissions               string
	GroupInheritanceHierarchy string
	Path                      string
	EntryType                 string
	Inherited                 string
	Source                    string
	AppliesTo                 string
	AccountType               string
	RiskLevel                 string
	ParentDelta               string
}

type trusteeSummary struct {
	Name  string
	Count int
}

type pathSummary struct {
	Path  string
	Count int
}

type htmlReport struct {
	BrandName      string
	GeneratedAt    string
	TotalCount     int
	ExplicitCount  int
	InheritedCount int
	UniquePaths    int
	UniqueTrustees int
	DenyCount      int
	HighRiskCount  int
	TopTrustees    []trusteeSummary
	TopPaths       []pathSummary
	UserRows       []exportUserRow
	Permissions    []models.Permission
}

func NewExporter() *Exporter {
	return &Exporter{}
}

func (e *Exporter) ExportToCSV(permissions []models.Permission, filename string) error {
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

	if err := writer.Write([]string{fmt.Sprintf("%s %s", formatExportTimestamp(time.Now()), reportDocumentTitle())}); err != nil {
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

	for _, row := range buildUserRows(permissions) {
		record := []string{
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
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return writer.Error()
}

func (e *Exporter) ExportToExcel(permissions []models.Permission, filename string) error {
	if err := ensureParentDir(filename); err != nil {
		return err
	}

	report := buildHTMLReport(permissions)
	workbook := excelize.NewFile()
	defer workbook.Close()

	userSheet := "权限报告"
	aclSheet := "访问控制列表"

	workbook.SetSheetName("Sheet1", userSheet)
	_, _ = workbook.NewSheet(aclSheet)
	_ = workbook.MergeCell(userSheet, "B1", "J1")
	workbook.SetCellValue(userSheet, "A1", report.GeneratedAt)
	workbook.SetCellValue(userSheet, "B1", reportDocumentTitle())
	workbook.SetCellValue(userSheet, "A2", "扫描总条目")
	workbook.SetCellValue(userSheet, "B2", report.TotalCount)
	workbook.SetCellValue(userSheet, "C2", "显式")
	workbook.SetCellValue(userSheet, "D2", report.ExplicitCount)
	workbook.SetCellValue(userSheet, "E2", "继承")
	workbook.SetCellValue(userSheet, "F2", report.InheritedCount)
	workbook.SetCellValue(userSheet, "G2", "高风险")
	workbook.SetCellValue(userSheet, "H2", report.HighRiskCount)

	userHeaders := []string{
		"Account Name", "First Name", "Last Name", "E-Mail", "Department", "Division", "Domain",
		"Originating Group", "Permissions", "Group Inheritance Hierarchy",
	}
	for index, header := range userHeaders {
		cell, _ := excelize.CoordinatesToCellName(index+1, 4)
		workbook.SetCellValue(userSheet, cell, header)
	}

	for index, row := range report.UserRows {
		values := []any{
			row.AccountName, row.FirstName, row.LastName, row.Email, row.Department, row.Division,
			row.Domain, row.OriginatingGroup, row.Permissions, row.GroupInheritanceHierarchy,
		}
		for colIndex, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, index+5)
			workbook.SetCellValue(userSheet, cell, value)
		}
	}

	aclHeaders := []string{
		"Account", "类型", "权限说明", "继承", "作用范围", "账户类型", "风险级别", "与父级差异", "路径", "SID", "来源组",
	}
	for index, header := range aclHeaders {
		cell, _ := excelize.CoordinatesToCellName(index+1, 3)
		workbook.SetCellValue(aclSheet, cell, header)
	}
	workbook.SetCellValue(aclSheet, "A1", report.GeneratedAt)
	workbook.SetCellValue(aclSheet, "B1", "访问控制列表")

	for index, permission := range report.Permissions {
		values := []any{
			permission.Trustee,
			translateTypeZh(permission.Type),
			formatPermissionDisplay(permission),
			translateBoolZh(permission.Inherited),
			translateAppliesToZh(permission.AppliesTo),
			translateAccountTypeZh(permission.AccountType),
			translateRiskLevelZh(fallback(permission.RiskLevel, inferRiskLevel(permission.Rights))),
			translateParentDeltaZh(fallback(permission.ParentDelta, deriveParentDelta(permission))),
			permission.Path,
			permission.TrusteeSID,
			deriveOriginatingGroup(permission),
		}
		for colIndex, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, index+4)
			workbook.SetCellValue(aclSheet, cell, value)
		}
	}

	headerStyle, _ := workbook.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#E7EEF8"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#16324F"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	titleStyle, _ := workbook.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "#16324F"},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	cardStyle, _ := workbook.NewStyle(&excelize.Style{
		Fill:   excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#EDF3FB"}},
		Border: []excelize.Border{{Type: "left", Color: "#2A6EF0", Style: 2}},
	})

	_ = workbook.SetCellStyle(userSheet, "B1", "J1", titleStyle)
	_ = workbook.SetCellStyle(userSheet, "A2", "H2", cardStyle)
	_ = workbook.SetCellStyle(userSheet, "A4", "J4", headerStyle)
	_ = workbook.SetCellStyle(aclSheet, "A3", "K3", headerStyle)
	_ = workbook.SetPanes(userSheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 4})
	_ = workbook.SetPanes(aclSheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 3})
	_ = workbook.SetColWidth(userSheet, "A", "J", 22)
	_ = workbook.SetColWidth(userSheet, "I", "I", 36)
	_ = workbook.SetColWidth(aclSheet, "A", "K", 22)
	_ = workbook.SetColWidth(aclSheet, "C", "C", 42)
	_ = workbook.SetColWidth(aclSheet, "I", "I", 42)
	_ = workbook.SetColWidth(aclSheet, "J", "J", 34)

	if index, err := workbook.GetSheetIndex(userSheet); err == nil {
		workbook.SetActiveSheet(index)
	}

	return workbook.SaveAs(filename)
}

func (e *Exporter) ExportToHTML(permissions []models.Permission, filename string) error {
	if err := ensureParentDir(filename); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	report := buildHTMLReport(permissions)

	const htmlDocument = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.BrandName}}</title>
    <style>
        :root { color-scheme: light; }
        body { margin: 0; font-family: "IBM Plex Sans", "Segoe UI", sans-serif; background: radial-gradient(circle at top left, rgba(20,184,166,0.16), transparent 38%), radial-gradient(circle at top right, rgba(14,165,233,0.12), transparent 28%), #edf4f6; color: #10202f; }
        main { width: min(1480px, calc(100% - 36px)); margin: 24px auto 56px; }
        .hero, .panel { background: linear-gradient(180deg, rgba(255,255,255,0.9), rgba(248,252,252,0.96)); border: 1px solid rgba(16,32,47,0.1); border-radius: 24px; box-shadow: 0 24px 68px rgba(15,23,42,0.08); }
        .hero { padding: 24px 28px; }
        .kicker { color: #0f766e; text-transform: uppercase; letter-spacing: .18em; font-size: 11px; font-weight: 700; }
        h1 { margin: 12px 0 4px; font-size: 34px; }
        .meta { color: #567080; font-size: 13px; }
        .metrics { list-style: none; margin: 20px 0 0; padding: 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 12px; }
        .metrics li { padding: 14px 16px; border: 1px solid rgba(16,32,47,0.08); border-radius: 18px; background: rgba(255,255,255,0.74); }
        .metrics strong { display: block; font-size: 11px; letter-spacing: .14em; text-transform: uppercase; color: #537286; margin-bottom: 8px; }
        .metrics span { font-size: 28px; font-weight: 700; }
        .grid { display: grid; gap: 18px; margin-top: 18px; grid-template-columns: repeat(2, minmax(0, 1fr)); }
        .panel { overflow: hidden; }
        .panel header { padding: 16px 20px; border-bottom: 1px solid rgba(16,32,47,0.08); background: linear-gradient(90deg, rgba(15,118,110,0.08), transparent 34%); }
        .panel header h2 { margin: 0; font-size: 18px; }
        .panel header p { margin: 6px 0 0; color: #567080; font-size: 13px; }
        .table-wrap { overflow: auto; max-height: 760px; }
        table { border-collapse: collapse; width: 100%; min-width: 1200px; }
        th, td { padding: 11px 14px; border-bottom: 1px solid rgba(16,32,47,0.08); text-align: left; vertical-align: top; }
        th { position: sticky; top: 0; background: #e8f5f4; color: #185564; font-size: 12px; letter-spacing: .04em; }
        td { font-size: 13px; color: #10202f; }
        tr:nth-child(even) td { background: rgba(2,132,199,0.03); }
        .badge { display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 999px; border: 1px solid rgba(15,118,110,0.18); background: rgba(15,118,110,0.08); color: #0f766e; font-size: 12px; }
        .badge.high { border-color: rgba(194,65,12,0.2); background: rgba(194,65,12,0.08); color: #c2410c; }
        .badge.medium { border-color: rgba(161,98,7,0.2); background: rgba(161,98,7,0.08); color: #a16207; }
        .badge.low { border-color: rgba(15,118,110,0.18); background: rgba(15,118,110,0.08); color: #0f766e; }
        .summary-list { list-style: none; margin: 0; padding: 18px 20px 22px; display: grid; gap: 10px; }
        .summary-list li { border: 1px solid rgba(16,32,47,0.08); border-radius: 16px; padding: 12px 14px; background: rgba(255,255,255,0.74); }
        .summary-list span { display: block; color: #567080; font-size: 12px; margin-bottom: 4px; }
        .empty { padding: 20px; color: #567080; }
        .mono { font-family: "IBM Plex Mono", Consolas, monospace; font-size: 12px; }
        @media (max-width: 980px) { .grid { grid-template-columns: 1fr; } }
    </style>
</head>
<body>
    <main>
        <section class="hero">
            <div class="kicker">权限报告</div>
            <h1>{{.BrandName}}</h1>
            <div class="meta">生成时间 {{.GeneratedAt}}</div>
            <ul class="metrics">
                <li><strong>总权限条目</strong><span>{{.TotalCount}}</span></li>
                <li><strong>显式权限</strong><span>{{.ExplicitCount}}</span></li>
                <li><strong>继承权限</strong><span>{{.InheritedCount}}</span></li>
                <li><strong>唯一路径</strong><span>{{.UniquePaths}}</span></li>
                <li><strong>唯一主体</strong><span>{{.UniqueTrustees}}</span></li>
                <li><strong>高风险</strong><span>{{.HighRiskCount}}</span></li>
            </ul>
        </section>
        <div class="grid">
            <section class="panel">
                <header>
                    <h2>高频主体</h2>
                    <p>当前导出里出现次数最多的用户或组。</p>
                </header>
                {{if .TopTrustees}}
                <ul class="summary-list">
                    {{range .TopTrustees}}
                    <li><span>Trustee</span><strong>{{.Name}}</strong><div>{{.Count}} entries</div></li>
                    {{end}}
                </ul>
                {{else}}
                <div class="empty">暂无主体统计。</div>
                {{end}}
            </section>
            <section class="panel">
                <header>
                    <h2>重点路径</h2>
                    <p>当前报告里 ACL 条目最多的路径。</p>
                </header>
                {{if .TopPaths}}
                <ul class="summary-list">
                    {{range .TopPaths}}
                    <li><span>Path</span><strong>{{.Path}}</strong><div>{{.Count}} entries</div></li>
                    {{end}}
                </ul>
                {{else}}
                <div class="empty">暂无路径统计。</div>
                {{end}}
            </section>
            <section class="panel">
                <header>
                    <h2>权限报告</h2>
                    <p>面向责任人和审计阅读，优先展示用户、部门、来源组和可读权限说明。</p>
                </header>
                <div class="table-wrap">
                    {{if .UserRows}}
                    <table>
                        <thead>
                            <tr>
                                <th>Account Name</th><th>First Name</th><th>Last Name</th><th>E-Mail</th><th>Department</th><th>Division</th><th>Domain</th><th>Originating Group</th><th>Permissions</th><th>Group Inheritance Hierarchy</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range .UserRows}}
                            <tr>
                                <td>{{.AccountName}}</td><td>{{.FirstName}}</td><td>{{.LastName}}</td><td>{{.Email}}</td><td>{{.Department}}</td><td>{{.Division}}</td><td>{{.Domain}}</td><td>{{.OriginatingGroup}}</td><td>{{.Permissions}}</td><td>{{.GroupInheritanceHierarchy}}</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                    {{else}}
                    <div class="empty">当前没有可导出的用户权限视图。</div>
                    {{end}}
                </div>
            </section>
            <section class="panel">
                <header>
                    <h2>访问控制列表</h2>
                    <p>保留 ACL 原始视角，便于排查 SID、继承、作用范围和路径级差异。</p>
                </header>
                <div class="table-wrap">
                    {{if .Permissions}}
                    <table>
                        <thead>
                            <tr>
                                <th>Account</th><th>类型</th><th>权限说明</th><th>继承</th><th>作用范围</th><th>账户类型</th><th>风险级别</th><th>与父级差异</th><th>Path</th><th>SID</th><th>来源组</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range .Permissions}}
                            <tr>
                                <td>{{.Trustee}}</td><td>{{typeZh .Type}}</td><td>{{permissionText .}}</td><td>{{boolZh .Inherited}}</td><td>{{appliesToZh .AppliesTo}}</td><td>{{accountTypeZh .AccountType}}</td><td><span class="badge {{riskClass (fallback .RiskLevel (inferRiskLevel .Rights))}}">{{riskZh (fallback .RiskLevel (inferRiskLevel .Rights))}}</span></td><td>{{parentDeltaZh (fallback .ParentDelta (parentDelta .))}}</td><td>{{.Path}}</td><td class="mono">{{.TrusteeSID}}</td><td>{{originatingGroup .}}</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                    {{else}}
                    <div class="empty">当前没有可导出的 ACL 明细。</div>
                    {{end}}
                </div>
            </section>
        </div>
    </main>
</body>
</html>`

	tmpl, err := template.New("export").Funcs(template.FuncMap{
		"accountTypeZh":   translateAccountTypeZh,
		"appliesToZh":     translateAppliesToZh,
		"boolZh":          translateBoolZh,
		"fallback":         fallback,
		"inferRiskLevel":   inferRiskLevel,
		"originatingGroup": deriveOriginatingGroup,
		"parentDelta":      deriveParentDelta,
		"parentDeltaZh":    translateParentDeltaZh,
		"permissionText":   formatPermissionDisplay,
		"riskClass": func(value string) string {
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "high":
				return "high"
			case "medium":
				return "medium"
			default:
				return "low"
			}
		},
		"riskZh": translateRiskLevelZh,
		"typeZh": translateTypeZh,
	}).Parse(htmlDocument)
	if err != nil {
		return err
	}

	return tmpl.Execute(file, report)
}

func ensureParentDir(filename string) error {
	parentDir := filepath.Dir(filename)
	if parentDir == "." || parentDir == "" {
		return nil
	}
	return os.MkdirAll(parentDir, 0o755)
}

func buildHTMLReport(permissions []models.Permission) htmlReport {
	cloned := append([]models.Permission(nil), permissions...)
	sort.Slice(cloned, func(i, j int) bool {
		if cloned[i].Path == cloned[j].Path {
			return cloned[i].Trustee < cloned[j].Trustee
		}
		return cloned[i].Path < cloned[j].Path
	})

	report := htmlReport{
		BrandName:   reportDocumentTitle(),
		GeneratedAt: formatExportTimestamp(time.Now()),
		TotalCount:  len(cloned),
		Permissions: cloned,
		UserRows:    buildUserRows(cloned),
	}

	paths := make(map[string]struct{})
	trustees := make(map[string]struct{})
	trusteeCounts := make(map[string]int)
	pathCounts := make(map[string]int)

	for _, permission := range cloned {
		normalizedPath := strings.ToLower(strings.TrimSpace(permission.Path))
		if normalizedPath != "" {
			paths[normalizedPath] = struct{}{}
			pathCounts[strings.TrimSpace(permission.Path)]++
		}

		normalizedTrustee := strings.TrimSpace(permission.Trustee)
		if normalizedTrustee != "" {
			trustees[strings.ToLower(normalizedTrustee)] = struct{}{}
			trusteeCounts[normalizedTrustee]++
		}

		if permission.Inherited {
			report.InheritedCount++
		} else {
			report.ExplicitCount++
		}

		if strings.EqualFold(strings.TrimSpace(permission.Type), "deny") {
			report.DenyCount++
		}

		if isHighRiskPermission(permission.Rights) || strings.EqualFold(permission.RiskLevel, "high") {
			report.HighRiskCount++
		}
	}

	report.UniquePaths = len(paths)
	report.UniqueTrustees = len(trustees)
	report.TopTrustees = summarizeTopTrustees(trusteeCounts, 10)
	report.TopPaths = summarizeTopPaths(pathCounts, 10)

	return report
}

func buildUserRows(permissions []models.Permission) []exportUserRow {
	rows := make([]exportUserRow, 0, len(permissions))
	for _, permission := range permissions {
		rows = append(rows, exportUserRow{
			AccountName:               fallback(permission.AccountName, extractAccountName(permission.Trustee)),
			FirstName:                 permission.FirstName,
			LastName:                  permission.LastName,
			Email:                     permission.Email,
			Department:                permission.Department,
			Division:                  permission.Division,
			Domain:                    fallback(permission.Domain, deriveDomain(permission)),
			OriginatingGroup:          deriveOriginatingGroup(permission),
			Permissions:               formatPermissionDisplay(permission),
			GroupInheritanceHierarchy: fallback(permission.GroupInheritanceHierarchy, deriveOriginatingGroup(permission)),
			Path:                      permission.Path,
			EntryType:                 translateTypeZh(permission.Type),
			Inherited:                 translateBoolZh(permission.Inherited),
			Source:                    permission.Source,
			AppliesTo:                 translateAppliesToZh(permission.AppliesTo),
			AccountType:               translateAccountTypeZh(permission.AccountType),
			RiskLevel:                 translateRiskLevelZh(fallback(permission.RiskLevel, inferRiskLevel(permission.Rights))),
			ParentDelta:               translateParentDeltaZh(fallback(permission.ParentDelta, deriveParentDelta(permission))),
		})
	}
	return rows
}

func fallback(primary, secondary string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(secondary)
}

func extractAccountName(trustee string) string {
	trimmed := strings.TrimSpace(trustee)
	if trimmed == "" {
		return ""
	}
	if index := strings.LastIndex(trimmed, `\`); index >= 0 && index < len(trimmed)-1 {
		return trimmed[index+1:]
	}
	if index := strings.LastIndex(trimmed, "@"); index > 0 {
		return trimmed[:index]
	}
	return trimmed
}

func deriveDomain(permission models.Permission) string {
	if strings.TrimSpace(permission.Domain) != "" {
		return strings.TrimSpace(permission.Domain)
	}
	if index := strings.LastIndex(strings.TrimSpace(permission.Trustee), `\`); index > 0 {
		return strings.ToUpper(strings.TrimSpace(permission.Trustee[:index]))
	}
	if permission.Email != "" {
		if index := strings.LastIndex(permission.Email, "@"); index >= 0 && index < len(permission.Email)-1 {
			return strings.ToUpper(permission.Email[index+1:])
		}
	}
	return ""
}

func deriveOriginatingGroup(permission models.Permission) string {
	if strings.TrimSpace(permission.OriginatingGroup) != "" {
		return strings.TrimSpace(permission.OriginatingGroup)
	}
	source := strings.TrimSpace(permission.Source)
	marker := "effective via "
	index := strings.LastIndex(strings.ToLower(source), marker)
	if index >= 0 {
		return strings.TrimSpace(source[index+len(marker):])
	}
	return source
}

func deriveParentDelta(permission models.Permission) string {
	if strings.TrimSpace(permission.ParentDelta) != "" {
		return strings.TrimSpace(permission.ParentDelta)
	}
	if permission.Inherited {
		return "Inherited from Parent"
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(permission.AppliesTo)), "subfolders") ||
		strings.Contains(strings.ToLower(strings.TrimSpace(permission.AppliesTo)), "files") {
		return "Explicit Inheritance Override"
	}
	return "Explicit on Current Item"
}

func formatPermissionDisplay(permission models.Permission) string {
	typeLabel := translateTypeZh(permission.Type)
	rightsLabel := translateRightsZh(permission.Rights)
	appliesToLabel := translateAppliesToZh(permission.AppliesTo)

	parts := make([]string, 0, 2)
	if typeLabel != "" {
		parts = append(parts, typeLabel+": "+rightsLabel)
	} else if rightsLabel != "" {
		parts = append(parts, rightsLabel)
	}
	if appliesToLabel != "" {
		parts = append(parts, appliesToLabel)
	}

	return strings.Join(parts, ", ")
}

func reportBrandName() string {
	brandName := strings.TrimSpace(os.Getenv("REPORT_BRAND_NAME"))
	if brandName == "" {
		return "OpenAD"
	}
	return brandName
}

func reportDocumentTitle() string {
	return fmt.Sprintf("%s - 权限报告", reportBrandName())
}

func formatExportTimestamp(value time.Time) string {
	return value.Format("2006/1/2 15:04")
}

func inferRiskLevel(rights string) string {
	if isHighRiskPermission(rights) {
		return "high"
	}
	value := strings.ToLower(strings.TrimSpace(rights))
	if strings.Contains(value, "execute") {
		return "medium"
	}
	return "low"
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

func translateRiskLevelZh(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
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
	replacements := []struct {
		from string
		to   string
	}{
		{"This Folder, Subfolders and Files", "此文件夹、子文件夹和文件"},
		{"This Folder and Subfolders", "此文件夹和子文件夹"},
		{"This Folder and Files", "此文件夹和文件"},
		{"This Folder Only", "仅此文件夹"},
		{"Subfolders and Files Only", "仅子文件夹和文件"},
		{"Subfolders Only", "仅子文件夹"},
		{"Files Only", "仅文件"},
		{"(No Propagate)", "（不传播）"},
	}

	result := strings.TrimSpace(value)
	for _, replacement := range replacements {
		result = strings.ReplaceAll(result, replacement.from, replacement.to)
	}
	return result
}

func translateParentDeltaZh(value string) string {
	switch strings.TrimSpace(value) {
	case "Inherited from Parent":
		return "继承自父级"
	case "Explicit Inheritance Override":
		return "显式覆盖父级继承"
	case "Explicit on Current Item":
		return "当前项显式设置"
	default:
		return strings.TrimSpace(value)
	}
}

func translateRightsZh(value string) string {
	result := strings.TrimSpace(value)
	if result == "" {
		return ""
	}

	replacements := []struct {
		from string
		to   string
	}{
		{"Read and Execute", "读取和执行"},
		{"ReadAndExecute", "读取和执行"},
		{"Full Control", "完全控制"},
		{"FullControl", "完全控制"},
		{"Modify", "修改"},
		{"Read", "读取"},
		{"Write", "写入"},
		{"Execute", "执行"},
		{"Delete", "删除"},
		{"Take Ownership", "取得所有权"},
		{"Change Permissions", "更改权限"},
		{"Synchronize", "同步"},
	}

	for _, replacement := range replacements {
		result = strings.ReplaceAll(result, replacement.from, replacement.to)
	}

	return result
}

func isHighRiskPermission(rights string) bool {
	value := strings.ToLower(strings.TrimSpace(rights))
	if value == "" {
		return false
	}
	highRiskTokens := []string{"full control", "modify", "write", "delete", "change permissions", "take ownership"}
	for _, token := range highRiskTokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func summarizeTopTrustees(counts map[string]int, limit int) []trusteeSummary {
	if len(counts) == 0 {
		return nil
	}
	items := make([]trusteeSummary, 0, len(counts))
	for trustee, count := range counts {
		items = append(items, trusteeSummary{Name: trustee, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func summarizeTopPaths(counts map[string]int, limit int) []pathSummary {
	if len(counts) == 0 {
		return nil
	}
	items := make([]pathSummary, 0, len(counts))
	for path, count := range counts {
		items = append(items, pathSummary{Path: path, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return strings.ToLower(items[i].Path) < strings.ToLower(items[j].Path)
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}
*/
