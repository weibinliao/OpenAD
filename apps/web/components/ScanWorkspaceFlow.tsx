import React from 'react';
import { Check, FolderSearch, PlayCircle, Settings2 } from 'lucide-react';
import { cn } from '../lib/cn';

export type ScanStage = 'target' | 'configure' | 'review';
export type ScanReviewView = 'overview' | 'acl' | 'access' | 'reports' | 'export';

export function deriveScanStage({
  path,
  running,
  failed,
  hasResults,
  imported,
}: {
  path: string;
  running: boolean;
  failed: boolean;
  hasResults: boolean;
  imported: boolean;
}): ScanStage {
  if (running || failed || hasResults || imported) return 'review';
  if (path.trim()) return 'configure';
  return 'target';
}

const copy = {
  en: {
    target: 'Select target',
    targetDescription: 'Choose a local folder or UNC share',
    configure: 'Configure',
    configureDescription: 'Set depth, inheritance, and AD resolution',
    review: 'Run and review',
    reviewDescription: 'Follow progress and inspect evidence',
    overview: 'Overview',
    acl: 'ACL Evidence',
    access: 'Access Matrix',
    reports: 'Permission Reports',
    export: 'Export',
  },
  'zh-CN': {
    target: '选择目标',
    targetDescription: '选择本地目录或 UNC 共享路径',
    configure: '配置扫描',
    configureDescription: '设置深度、继承和 AD 解析',
    review: '执行与复核',
    reviewDescription: '查看进度并核对权限证据',
    overview: '概览',
    acl: 'ACL 证据',
    access: '访问矩阵',
    reports: '权限报告',
    export: '导出',
  },
} as const;

export function ScanWorkflowNavigation({ locale, activeStage }: { locale: 'en' | 'zh-CN'; activeStage: ScanStage }) {
  const labels = copy[locale];
  const stages = [
    { id: 'target' as const, label: labels.target, description: labels.targetDescription, icon: FolderSearch },
    { id: 'configure' as const, label: labels.configure, description: labels.configureDescription, icon: Settings2 },
    { id: 'review' as const, label: labels.review, description: labels.reviewDescription, icon: PlayCircle },
  ];
  const activeIndex = stages.findIndex((stage) => stage.id === activeStage);

  return (
    <ol className="grid overflow-hidden rounded-lg border border-line bg-surface-raised md:grid-cols-3" aria-label="Scan workflow">
      {stages.map((stage, index) => {
        const Icon = stage.icon;
        const active = stage.id === activeStage;
        const complete = index < activeIndex;
        return (
          <li
            key={stage.id}
            className={cn(
              'relative flex min-h-[78px] items-center gap-3 px-4 py-3 md:border-r md:border-line md:last:border-r-0',
              active && 'bg-accent-soft',
            )}
          >
            <span className={cn(
              'flex h-8 w-8 shrink-0 items-center justify-center rounded-md border',
              active ? 'border-accent/40 bg-accent text-fg-on-accent' : complete ? 'border-success/40 bg-success-soft text-success' : 'border-line bg-surface-base text-fg-muted',
            )}>
              {complete ? <Check className="h-4 w-4" /> : <Icon className="h-4 w-4" />}
            </span>
            <span className="min-w-0">
              <span className="block text-xs font-semibold text-fg" aria-current={active ? 'step' : undefined}>{stage.label}</span>
              <span className="mt-0.5 block text-2xs leading-4 text-fg-muted">{stage.description}</span>
            </span>
            <span className="absolute right-3 top-2 text-2xs tabular-nums text-fg-subtle">0{index + 1}</span>
          </li>
        );
      })}
    </ol>
  );
}

export function ScanReviewTabs({
  locale,
  value,
  onChange,
}: {
  locale: 'en' | 'zh-CN';
  value: ScanReviewView;
  onChange: (view: ScanReviewView) => void;
}) {
  const labels = copy[locale];
  const tabs: Array<{ id: ScanReviewView; label: string }> = [
    { id: 'overview', label: labels.overview },
    { id: 'acl', label: labels.acl },
    { id: 'access', label: labels.access },
    { id: 'reports', label: labels.reports },
    { id: 'export', label: labels.export },
  ];

  return (
    <div className="overflow-x-auto border-b border-line" role="tablist" aria-label="Scan result views">
      <div className="flex min-w-max gap-1">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={value === tab.id}
            onClick={() => onChange(tab.id)}
            className={cn(
              'relative h-10 px-3 text-xs font-medium text-fg-muted transition-colors hover:text-fg',
              value === tab.id && 'text-fg after:absolute after:inset-x-2 after:bottom-0 after:h-0.5 after:bg-accent',
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>
    </div>
  );
}
