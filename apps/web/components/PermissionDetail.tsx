import React from 'react';
import {
  Building2,
  FileCode2,
  FolderTree,
  Mail,
  Network,
  Shield,
  UserRound,
  X,
} from 'lucide-react';
import { useI18n } from '../contexts/I18nContext';
import { Badge, riskTone } from './ui/badge';

export interface PermissionDetailItem {
  path: string;
  trustee: string;
  trustee_sid: string;
  rights: string;
  type: string;
  inherited: boolean;
  source?: string;
  applies_to?: string;
  account_type?: string;
  risk_level?: string;
  parent_delta?: string;
  account_name?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  department?: string;
  division?: string;
  domain?: string;
  originating_group?: string;
  group_inheritance_hierarchy?: string;
}

interface PermissionDetailProps {
  permission: PermissionDetailItem | null;
  isOpen: boolean;
  onClose: () => void;
}

function copy(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function inferRisk(permission: PermissionDetailItem) {
  const provided = (permission.risk_level || '').trim().toLowerCase();
  if (provided === 'high' || provided === 'medium' || provided === 'low') {
    return provided;
  }

  const rights = permission.rights.trim().toLowerCase();
  if (['full control', 'write', 'delete', 'take ownership', 'change permissions', 'modify'].some((token) => rights.includes(token))) {
    return 'high';
  }
  if (rights.includes('execute')) {
    return 'medium';
  }
  return rights ? 'low' : 'unknown';
}

function FactTile({ label, value }: { label: React.ReactNode; value: React.ReactNode }) {
  return (
    <div className="rounded-md bg-surface-sunken px-3 py-2.5">
      <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{label}</div>
      <div className="mt-1 break-words text-sm text-fg">{value}</div>
    </div>
  );
}

function DetailSection({ icon: Icon, title, children }: { icon: React.ComponentType<{ className?: string }>; title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border border-line bg-surface-base p-4 shadow-token-sm">
      <div className="flex items-center gap-2 text-sm font-semibold text-fg">
        <Icon className="h-4 w-4 text-fg-muted" />
        {title}
      </div>
      {children}
    </section>
  );
}

export default function PermissionDetail({ permission, isOpen, onClose }: PermissionDetailProps) {
  const { locale } = useI18n();

  if (!isOpen || !permission) {
    return null;
  }

  const risk = inferRisk(permission);
  const identityFacts = [
    { label: copy(locale, 'Account Name', '账户名'), value: permission.account_name || permission.trustee },
    { label: copy(locale, 'Display Name', '显示名'), value: [permission.first_name, permission.last_name].filter(Boolean).join(' ') || '-' },
    { label: copy(locale, 'Domain', '域'), value: permission.domain || '-' },
    { label: copy(locale, 'Account Type', '账户类型'), value: permission.account_type || '-' },
    { label: copy(locale, 'Department', '部门'), value: permission.department || '-' },
    { label: copy(locale, 'Division', '事业部'), value: permission.division || '-' },
  ];
  const permissionFacts = [
    { label: copy(locale, 'Entry Type', '条目类型'), value: permission.type || '-' },
    { label: copy(locale, 'Inherited', '继承'), value: permission.inherited ? copy(locale, 'Yes', '是') : copy(locale, 'No', '否') },
    { label: copy(locale, 'Applies To', '作用范围'), value: permission.applies_to || '-' },
    { label: copy(locale, 'Parent Delta', '与父级差异'), value: permission.parent_delta || '-' },
    { label: copy(locale, 'Source', '来源'), value: permission.source || '-' },
    { label: copy(locale, 'Originating Group', '来源组'), value: permission.originating_group || '-' },
  ];

  return (
    <div className="app-v2 fixed inset-0 z-50 bg-black/50" onClick={onClose}>
      <aside
        className="ml-auto flex h-full w-full max-w-[460px] flex-col border-l border-line bg-surface-overlay shadow-token-lg"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="border-b border-line px-5 py-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-2xs font-semibold uppercase tracking-wide text-fg-muted">{copy(locale, 'Inspector', '检查面板')}</div>
              <h3 className="mt-1 text-lg font-semibold tracking-tight text-fg">{permission.account_name || permission.trustee}</h3>
              <p className="mt-1 break-all text-xs text-fg-muted">{permission.trustee}</p>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="rounded-md p-1.5 text-fg-muted transition-colors hover:bg-surface-sunken hover:text-fg"
              aria-label={copy(locale, 'Close', '关闭')}
            >
              <X className="h-4 w-4" />
            </button>
          </div>
          <div className="mt-3 flex flex-wrap gap-1.5">
            <Badge tone={riskTone(risk)} className="uppercase">
              {risk}
            </Badge>
            <Badge tone="neutral">{permission.inherited ? copy(locale, 'Inherited', '继承') : copy(locale, 'Explicit', '显式')}</Badge>
            <Badge tone="neutral">{permission.type || '-'}</Badge>
          </div>
        </div>

        <div className="flex-1 space-y-4 overflow-y-auto bg-surface-page px-5 py-5">
          <DetailSection icon={Shield} title={copy(locale, 'Permission Summary', '权限摘要')}>
            <div className="mt-3 rounded-md bg-surface-sunken px-3 py-2.5 text-sm leading-6 text-fg">
              {permission.rights || '-'}
            </div>
            <div className="mt-3 grid gap-2 sm:grid-cols-2">
              {permissionFacts.map((item) => (
                <FactTile key={item.label} label={item.label} value={item.value} />
              ))}
            </div>
          </DetailSection>

          <DetailSection icon={UserRound} title={copy(locale, 'Identity Context', '身份上下文')}>
            <div className="mt-3 grid gap-2 sm:grid-cols-2">
              {identityFacts.map((item) => (
                <FactTile key={item.label} label={item.label} value={item.value} />
              ))}
            </div>
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              <FactTile
                label={
                  <span className="inline-flex items-center gap-1.5">
                    <Mail className="h-3 w-3" /> E-Mail
                  </span>
                }
                value={<span className="break-all">{permission.email || '-'}</span>}
              />
              <FactTile
                label={
                  <span className="inline-flex items-center gap-1.5">
                    <Building2 className="h-3 w-3" /> SID
                  </span>
                }
                value={<span className="break-all font-mono text-xs">{permission.trustee_sid || '-'}</span>}
              />
            </div>
          </DetailSection>

          <DetailSection icon={FolderTree} title={copy(locale, 'Path Scope', '路径范围')}>
            <div className="mt-3 break-all rounded-md bg-surface-sunken px-3 py-2.5 font-mono text-xs text-fg">
              {permission.path}
            </div>
          </DetailSection>

          <DetailSection icon={Network} title={copy(locale, 'Inheritance Story', '继承链说明')}>
            <div className="mt-3 rounded-md bg-surface-sunken px-3 py-2.5 text-sm leading-6 text-fg">
              {permission.group_inheritance_hierarchy || permission.originating_group || permission.source || '-'}
            </div>
          </DetailSection>

          <section className="rounded-lg border border-dashed border-line p-4">
            <div className="flex items-center gap-2 text-sm font-semibold text-fg">
              <FileCode2 className="h-4 w-4 text-fg-muted" />
              {copy(locale, 'Why this row matters', '这条记录为什么重要')}
            </div>
            <p className="mt-2 text-sm leading-6 text-fg-muted">
              {copy(
                locale,
                'This inspector pulls the identity, scope, and inheritance clues into one place so you do not need to scan a very wide table to understand a single ACL row.',
                '这个检查面板把身份、作用范围和继承线索集中到一起，你不需要再横向滚动超宽表格才能读懂单条 ACL 记录。'
              )}
            </p>
          </section>
        </div>
      </aside>
    </div>
  );
}
