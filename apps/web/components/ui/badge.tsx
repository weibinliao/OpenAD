import React, { type HTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

export type BadgeTone =
  | 'neutral'
  | 'accent'
  | 'success'
  | 'warning'
  | 'danger'
  | 'info'
  | 'risk-critical'
  | 'risk-high'
  | 'risk-medium'
  | 'risk-low';

const toneClasses: Record<BadgeTone, string> = {
  neutral: 'bg-surface-sunken text-fg-secondary border-line',
  accent: 'bg-accent-soft text-accent-fg border-accent/30',
  success: 'bg-success-soft text-success border-success/30',
  warning: 'bg-warning-soft text-warning border-warning/30',
  danger: 'bg-danger-soft text-danger border-danger/30',
  info: 'bg-info-soft text-info border-info/30',
  'risk-critical': 'bg-danger-soft text-risk-critical border-risk-critical/40',
  'risk-high': 'bg-warning-soft text-risk-high border-risk-high/40',
  'risk-medium': 'bg-warning-soft text-risk-medium border-risk-medium/40',
  'risk-low': 'bg-success-soft text-risk-low border-risk-low/40',
};

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: BadgeTone;
  dot?: boolean;
}

export function Badge({ tone = 'neutral', dot = false, className, children, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-2xs font-medium leading-4 whitespace-nowrap',
        toneClasses[tone],
        className,
      )}
      {...props}
    >
      {dot ? <span className="h-1.5 w-1.5 rounded-full bg-current" aria-hidden /> : null}
      {children}
    </span>
  );
}

/** Maps backend risk levels (critical/high/medium/low or zh equivalents) to a badge tone. */
export function riskTone(riskLevel: string | undefined | null): BadgeTone {
  switch ((riskLevel || '').toLowerCase()) {
    case 'critical':
    case '严重':
      return 'risk-critical';
    case 'high':
    case '高':
      return 'risk-high';
    case 'medium':
    case '中':
      return 'risk-medium';
    case 'low':
    case '低':
      return 'risk-low';
    default:
      return 'neutral';
  }
}

export function riskLabel(riskLevel: string | undefined | null, locale: string) {
  const normalized = (riskLevel || '').toLowerCase();
  const labels: Record<string, [string, string]> = {
    critical: ['Critical', '严重'],
    high: ['High', '高'],
    medium: ['Medium', '中'],
    low: ['Low', '低'],
  };
  const label = labels[normalized];
  return label ? label[locale === 'zh-CN' ? 1 : 0] : riskLevel || '-';
}

export function scanStatusLabel(status: string | undefined | null, locale: string) {
  const normalized = (status || '').toLowerCase();
  const labels: Record<string, [string, string]> = {
    queued: ['Queued', '排队中'],
    running: ['Running', '运行中'],
    completed: ['Completed', '已完成'],
    failed: ['Failed', '失败'],
    cancelled: ['Cancelled', '已取消'],
  };
  const label = labels[normalized];
  return label ? label[locale === 'zh-CN' ? 1 : 0] : status || '-';
}
