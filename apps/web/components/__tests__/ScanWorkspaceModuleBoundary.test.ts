import { existsSync, readFileSync } from 'fs';
import { resolve } from 'path';

describe('Scan Workspace module boundary', () => {
  test('keeps scan execution in the page and routes analysis to owning modules', () => {
    const source = readFileSync(resolve(process.cwd(), 'pages/scan-workspace.tsx'), 'utf8');

    expect(source).toContain('<ScanCompletionSummary');
    expect(source).not.toContain('<ScanReviewTabs');
    expect(source).not.toContain('<ExactPathPermissionTable');
    expect(source).not.toContain('<UNCDomainAccessMatrix');
    expect(source).not.toContain('<PermissionViewerQuickReports');
    expect(source).not.toContain('<ExportReportPreviewV2');
    expect(source).not.toContain('<ScanWorkflowNavigation');
    expect(source).not.toContain('onOpenTemplates');
    expect(source).not.toContain('report-view');
    expect(source).not.toContain('Exact-path ACL rows are ready below');
    expect(source).not.toContain("from '../components/PermissionViewerQuickReports'");
    expect(source).not.toContain("from '../components/ExportReportPreviewV2'");
    expect(source).not.toContain("from '../components/PermissionReportWorkspace'");
    expect(source).not.toContain("from '../lib/reportPayload'");
    expect(source).not.toContain('readReportDefaults');
    expect(source).not.toContain('writeReportDefaults');
    expect(source).not.toContain('readScanDefaults');
    expect(source).not.toContain('writeScanDefaults');
    expect(source).not.toContain('/api/export/');
    expect(source).not.toContain('/api/sessions');
    expect(source).not.toContain('/api/ad/users/query');
    expect(source).not.toContain('/api/ad/principals/expand');
    expect(source).toContain('connection_id: activeProfile.id');
  });

  test('owns report composition in the standalone report center', () => {
    const pagePath = resolve(process.cwd(), 'pages/reports.tsx');
    const workspacePath = resolve(process.cwd(), 'components/ReportCenterWorkspace.tsx');

    expect(existsSync(pagePath)).toBe(true);
    expect(existsSync(workspacePath)).toBe(true);
    if (!existsSync(pagePath) || !existsSync(workspacePath)) return;

    const pageSource = readFileSync(pagePath, 'utf8');
    const workspaceSource = readFileSync(workspacePath, 'utf8');
    expect(pageSource).toContain('<ReportCenterWorkspace');
    expect(workspaceSource).toContain('<PermissionViewerQuickReports');
    expect(workspaceSource).toContain('<PermissionReportWorkspace');
    expect(workspaceSource).toContain('<ExportReportPreviewV2');
  });
});
