package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/models"
)

func TestSanitizeExportFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{name: "quote", filename: `finance"report`, expected: "finance-report.csv"},
		{name: "semicolon", filename: "finance;report", expected: "finance-report.csv"},
		{name: "CRLF", filename: "finance\r\nX-Injected: yes", expected: "finance--X-Injected- yes.csv"},
		{name: "path separators", filename: `..\private/report.csv`, expected: "report.csv"},
		{name: "parent directory", filename: "..", expected: "permissions-export.csv"},
		{name: "only dots", filename: "....", expected: "permissions-export.csv"},
		{name: "empty", filename: "", expected: "permissions-export.csv"},
		{name: "Chinese", filename: "权限报告", expected: "权限报告.csv"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, sanitizeExportFilename(test.filename, "csv"))
		})
	}
}

func TestExportContentDispositionUsesASCIIFallbackAndUTF8Filename(t *testing.T) {
	header := exportContentDisposition("权限报告.csv")

	assert.Equal(
		t,
		`attachment; filename="permissions-export.csv"; filename*=UTF-8''%E6%9D%83%E9%99%90%E6%8A%A5%E5%91%8A.csv`,
		header,
	)
	assert.NotContains(t, header, "\r")
	assert.NotContains(t, header, "\n")
}

func TestExportContentDispositionPercentEncodesNonAttrCharacters(t *testing.T) {
	header := exportContentDisposition("a@b=100%.csv")

	assert.Equal(
		t,
		`attachment; filename="a@b=100%.csv"; filename*=UTF-8''a%40b%3D100%25.csv`,
		header,
	)
}

func TestExportDownloadSanitizesContentDispositionBeforeWritingResponse(t *testing.T) {
	exporter := &stubExporter{}
	router := newTestRouter(applicationDependencies{exporter: exporter})

	recorder := performJSONRequestToRouter(t, router, "/api/export/download", ExportDownloadRequest{
		Permissions: []models.Permission{{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`, Rights: "Read"}},
		Format:      "csv",
		Filename:    "finance\";report\r\nX-Injected: yes",
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(
		t,
		`attachment; filename="finance--report--X-Injected- yes.csv"; filename*=UTF-8''finance--report--X-Injected-%20yes.csv`,
		recorder.Header().Get("Content-Disposition"),
	)
	assert.Empty(t, recorder.Header().Values("X-Injected"))
	assert.Contains(t, recorder.Body.String(), "Path,Trustee")
	require.NotNil(t, exporter.lastCSV)
	assert.NoFileExists(t, exporter.lastCSV.filename)
}

func TestExportRoutesRejectDeclaredBodyOverLimit(t *testing.T) {
	for _, endpoint := range []string{"/api/export", "/api/export/download", "/api/export/summary"} {
		t.Run(endpoint, func(t *testing.T) {
			exporter := &stubExporter{}
			router := newTestRouter(applicationDependencies{exporter: exporter})

			request := httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.ContentLength = exportRequestBodyLimitBytes + 1
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "narrow the scan scope or export permissions in smaller batches")
			assert.Nil(t, exporter.lastCSV)
		})
	}
}

func TestBindExportJSONRejectsUnknownLengthBodyOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/limited-export", func(context *gin.Context) {
		var request ExportDownloadRequest
		if !bindExportJSON(context, &request, 32) {
			return
		}
		context.Status(http.StatusNoContent)
	})

	body := `{"permissions":[],"filename":"` + strings.Repeat("x", 64) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/limited-export", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = -1
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "narrow the scan scope or export permissions in smaller batches")
}
