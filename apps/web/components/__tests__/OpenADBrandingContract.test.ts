import { readFileSync } from 'fs';
import { resolve } from 'path';

const readSource = (path: string) => readFileSync(resolve(process.cwd(), path), 'utf8');

const desktopFrame = readSource('components/shell/DesktopWindowFrame.tsx');
const reportCenter = readSource('components/ReportCenterWorkspace.tsx');
const scanConsole = readSource('components/ScanExplorerConsole.tsx');
const explorer = readSource('components/explorer/Explorer.tsx');
const legacyI18n = readSource('contexts/I18nContext.tsx');
const sharedEnglishI18n = readSource('lib/i18n/en.ts');
const sharedChineseI18n = readSource('lib/i18n/zh.ts');
const runtimeHealth = readSource('hooks/useRuntimeHealth.ts');
const runtimeMarker = readSource('public/openad-runtime.json');
const apiMain = readSource('../backend/cmd/api/main.go');
const exportHandler = readSource('../backend/cmd/api/handlers_export.go');
const auditHandler = readSource('../backend/cmd/api/handlers_audit.go');
const fileActivity = readSource('../backend/cmd/api/file_activity.go');
const csvExporter = readSource('../backend/internal/export/exporter.go');
const v2Exporter = readSource('../backend/internal/export/exporter_v2.go');
const cliRoot = readSource('../backend/internal/cli/root.go');

describe('OpenAD public branding contract', () => {
  it('uses OpenAD on every public product surface', () => {
    for (const source of [desktopFrame, reportCenter, scanConsole, explorer]) {
      expect(source).not.toContain('PermissionProtector');
    }

    expect(desktopFrame).toContain("title = 'OpenAD'");
    expect(reportCenter).toContain('OpenAD - Management Summary');
  });

  it('uses OpenAD for runtime contracts, reports, and operational guidance', () => {
    expect(legacyI18n).toContain("appTitle: 'OpenAD'");
    expect(legacyI18n).toContain("auditSummaryTitle: 'OpenAD - Audit Summary'");
    expect(sharedEnglishI18n).toContain("appName: 'OpenAD'");
    expect(sharedChineseI18n).toContain("appName: 'OpenAD'");

    expect(runtimeMarker).toContain('"product": "OpenAD"');
    expect(runtimeHealth).toContain("data.service === 'openad'");
    expect(apiMain).toContain('"service":        "openad"');

    expect(exportHandler).toContain('OpenAD - Management Summary');
    expect(exportHandler).toContain('OpenAD - Compliance Summary');
    expect(exportHandler).toContain('OpenAD - Operations Summary');
    expect(exportHandler).toContain('preparedBy = "OpenAD"');
    expect(auditHandler).toContain('OpenAD - Audit Summary');
    expect(fileActivity).not.toContain('Run PermissionProtector');
    expect(csvExporter).toContain('return "OpenAD"');
    expect(v2Exporter).toContain('return "OpenAD"');
    expect(cliRoot).toContain('Short: "OpenAD command line interface"');
  });
});
