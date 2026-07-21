import { CheckCircle2, FileText, GitFork, History, RefreshCw, ShieldAlert } from 'lucide-react';
import { useRouter } from 'next/router';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Card } from './ui/card';

export interface ScanCompletionSummaryProps {
  locale: 'en' | 'zh-CN';
  path: string;
  sessionID: string;
  itemsScanned: number;
  permissionCount: number;
  skippedCount: number;
  riskCount: number;
  onRescan: () => void;
}

const copy = {
  en: {
    eyebrow: 'Scan complete',
    title: 'The scan is ready for follow-up work',
    description: 'Confirm the result here, then continue in the module that owns the next task.',
    objects: 'Objects',
    acl: 'ACL rows',
    skipped: 'Skipped',
    risks: 'Risk findings',
    session: 'Session',
    evidence: 'View permission evidence',
    findings: 'View risk findings',
    history: 'Open scan history',
    report: 'Generate report',
    rescan: 'Run again',
  },
  'zh-CN': {
    eyebrow: '扫描完成',
    title: '本次扫描已可进入后续处理',
    description: '先确认本次结果，再进入负责权限、风险或历史工作的模块。',
    objects: '扫描对象',
    acl: 'ACL 记录',
    skipped: '跳过路径',
    risks: '风险发现',
    session: '会话',
    evidence: '查看权限证据',
    findings: '查看风险项',
    history: '查看本次记录',
    report: '生成报告',
    rescan: '重新扫描',
  },
} as const;

export default function ScanCompletionSummary({
  locale,
  path,
  sessionID,
  itemsScanned,
  permissionCount,
  skippedCount,
  riskCount,
  onRescan,
}: ScanCompletionSummaryProps) {
  const router = useRouter();
  const labels = copy[locale];
  const facts = [
    { label: labels.objects, value: itemsScanned },
    { label: labels.acl, value: permissionCount },
    { label: labels.skipped, value: skippedCount },
    { label: labels.risks, value: riskCount },
  ];
  const reportHref = '/reports?mode=user&path=' + encodeURIComponent(path)
    + '&session=' + encodeURIComponent(sessionID);

  return (
    <Card className="p-5" aria-label={labels.eyebrow}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-success-soft text-success">
            <CheckCircle2 className="h-5 w-5" />
          </span>
          <div className="min-w-0">
            <div className="text-2xs font-medium uppercase text-success">{labels.eyebrow}</div>
            <h2 className="mt-0.5 text-sm font-semibold text-fg">{labels.title}</h2>
            <p className="mt-1 text-xs leading-5 text-fg-muted">{labels.description}</p>
          </div>
        </div>
        {sessionID ? <Badge tone="neutral" className="font-mono">{labels.session}: {sessionID}</Badge> : null}
      </div>

      <div className="mt-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        {facts.map((fact) => (
          <div key={fact.label} className="border-l-2 border-line pl-3">
            <div className="text-2xs text-fg-muted">{fact.label}</div>
            <div className="mt-1 text-lg font-semibold tabular-nums text-fg">{fact.value.toLocaleString()}</div>
          </div>
        ))}
      </div>

      <div className="mt-5 flex flex-wrap gap-2 border-t border-line pt-4">
        <Button type="button" onClick={() => void router.push(`/access?path=${encodeURIComponent(path)}`)}>
          <GitFork className="h-4 w-4" />
          {labels.evidence}
        </Button>
        <Button type="button" variant="secondary" onClick={() => void router.push('/findings')}>
          <ShieldAlert className="h-4 w-4" />
          {labels.findings}
        </Button>
        <Button type="button" variant="secondary" onClick={() => void router.push('/history')}>
          <History className="h-4 w-4" />
          {labels.history}
        </Button>
        <Button type="button" variant="secondary" onClick={() => void router.push(reportHref)}>
          <FileText className="h-4 w-4" />
          {labels.report}
        </Button>
        <Button type="button" variant="outline" onClick={onRescan}>
          <RefreshCw className="h-4 w-4" />
          {labels.rescan}
        </Button>
      </div>
    </Card>
  );
}
