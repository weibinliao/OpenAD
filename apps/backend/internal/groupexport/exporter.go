package groupexport

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/xuri/excelize/v2"
)

type Row struct {
	GroupName      string
	GroupDN        string
	MemberType     string
	DisplayName    string
	SAMAccountName string
	Email          string
	Department     string
	Division       string
	Domain         string
	SID            string
	MemberDN       string
	Membership     string
	Depth          int
	MembershipPath string
}

type Exporter struct{}

func NewExporter() *Exporter {
	return &Exporter{}
}

var headers = []string{
	"Group Name", "Group DN", "Member Type", "Display Name", "sAMAccountName", "Email",
	"Department", "Division", "Domain", "SID", "Member DN", "Membership", "Depth", "Membership Path",
}

func (exporter *Exporter) CSV(_ models.ADGroup, rows []Row) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buffer)
	if err := writer.Write(headers); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := writer.Write(row.values()); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (exporter *Exporter) XLSX(_ models.ADGroup, rows []Row) ([]byte, error) {
	book := excelize.NewFile()
	defer book.Close()
	const sheet = "Members"
	book.SetSheetName("Sheet1", sheet)

	headerStyle, err := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#225797"}},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "#C9D4E2", Style: 1},
			{Type: "right", Color: "#C9D4E2", Style: 1},
			{Type: "top", Color: "#C9D4E2", Style: 1},
			{Type: "bottom", Color: "#C9D4E2", Style: 1},
		},
	})
	if err != nil {
		return nil, err
	}

	for index, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			return nil, err
		}
	}
	lastColumn, _ := excelize.ColumnNumberToName(len(headers))
	if err := book.SetCellStyle(sheet, "A1", lastColumn+"1", headerStyle); err != nil {
		return nil, err
	}
	_ = book.SetRowHeight(sheet, 1, 28)

	for rowIndex, row := range rows {
		for columnIndex, value := range row.values() {
			cell, _ := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+2)
			if err := book.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}

	widths := map[string]float64{
		"A": 20, "B": 42, "C": 14, "D": 24, "E": 20, "F": 28, "G": 20,
		"H": 18, "I": 16, "J": 28, "K": 42, "L": 14, "M": 10, "N": 48,
	}
	for column, width := range widths {
		_ = book.SetColWidth(sheet, column, column, width)
	}
	_ = book.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	endRow := len(rows) + 1
	if endRow < 1 {
		endRow = 1
	}
	_ = book.AutoFilter(sheet, fmt.Sprintf("A1:%s%d", lastColumn, endRow), nil)

	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func RowsFromDirectMembers(group models.ADGroup) []Row {
	rows := make([]Row, 0, len(group.Members))
	for _, member := range group.Members {
		rows = append(rows, rowForPrincipal(group, member, 0, nil))
	}
	return rows
}

func RowsFromResolution(group models.ADGroup, resolution models.ADGroupResolution) []Row {
	rows := make([]Row, 0, len(resolution.Members))
	for _, member := range resolution.Members {
		rows = append(rows, rowForPrincipal(group, member.ADPrincipal, member.Depth, member.Path))
	}
	return rows
}

func rowForPrincipal(group models.ADGroup, principal models.ADPrincipal, depth int, path []string) Row {
	membership := "direct"
	if depth > 0 {
		membership = "nested"
	}
	displayName := firstNonEmpty(principal.Name, principal.SAMAccountName, principal.DN, principal.SID)
	labels := make([]string, 0, len(path)+2)
	if len(path) == 0 {
		labels = append(labels, firstNonEmpty(group.Name, dnLabel(group.DN)))
	} else {
		for _, value := range path {
			labels = append(labels, dnLabel(value))
		}
	}
	labels = append(labels, displayName)

	return Row{
		GroupName:      firstNonEmpty(group.Name, dnLabel(group.DN)),
		GroupDN:        group.DN,
		MemberType:     string(principal.Type),
		DisplayName:    displayName,
		SAMAccountName: principal.SAMAccountName,
		Email:          principal.Email,
		Department:     principal.Department,
		Division:       principal.Division,
		Domain:         principal.Domain,
		SID:            principal.SID,
		MemberDN:       principal.DN,
		Membership:     membership,
		Depth:          depth,
		MembershipPath: strings.Join(dedupeNonEmpty(labels), " > "),
	}
}

func (row Row) values() []string {
	return []string{
		row.GroupName, row.GroupDN, row.MemberType, row.DisplayName, row.SAMAccountName,
		row.Email, row.Department, row.Division, row.Domain, row.SID, row.MemberDN,
		row.Membership, strconv.Itoa(row.Depth), row.MembershipPath,
	}
}

func dnLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	for _, part := range strings.Split(trimmed, ",") {
		part = strings.TrimSpace(part)
		if len(part) > 3 && strings.EqualFold(part[:3], "CN=") {
			return strings.TrimSpace(part[3:])
		}
	}
	return trimmed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func dedupeNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		key := strings.ToLower(trimmed)
		if key == "" {
			continue
		}
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
