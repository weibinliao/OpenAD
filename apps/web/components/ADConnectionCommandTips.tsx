import React, { useState } from 'react';
import { Check, ChevronDown, Copy, Info, TerminalSquare } from 'lucide-react';
import { cn } from '../lib/cn';

export const AD_DISCOVERY_COMMANDS = [
  { id: 'hostname', command: 'hostname' },
  { id: 'network', command: 'ipconfig /all' },
  { id: 'account', command: 'whoami' },
  { id: 'upn', command: 'whoami /upn' },
  { id: 'dc', command: '(Get-ADDomainController -Discover).HostName' },
  { id: 'baseDn', command: "([ADSI]'LDAP://RootDSE').defaultNamingContext" },
  { id: 'ldap', command: 'Test-NetConnection DC-01 -Port 389' },
  { id: 'ldaps', command: 'Test-NetConnection DC-01 -Port 636' },
] as const;

const copy = {
  en: {
    title: 'How to find these values',
    description: 'Run the matching command on a domain-joined Windows host or domain controller.',
    note: '389 is LDAP; 636 is LDAPS. Get-ADDomainController requires the ActiveDirectory PowerShell module (RSAT). Replace DC-01 with your domain controller name.',
    password: 'Passwords cannot be discovered by OpenAD. Use an approved domain service account and enter its password manually.',
    copied: 'Command copied',
    copy: 'Copy',
    labels: {
      hostname: 'Computer name', network: 'DNS and domain details', account: 'Domain account', upn: 'User principal name',
      dc: 'Discover a domain controller', baseDn: 'Base DN', ldap: 'Test LDAP port', ldaps: 'Test LDAPS port',
    },
  },
  'zh-CN': {
    title: '如何找到这些连接信息',
    description: '请在已加入域的 Windows 主机或域控制器上运行对应命令。',
    note: '389 为 LDAP，636 为 LDAPS。Get-ADDomainController 需要安装 ActiveDirectory PowerShell 模块（RSAT）。请把 DC-01 替换为实际域控制器名称。',
    password: 'OpenAD 无法获取密码。请使用已审批的域服务账号，并手动输入密码。',
    copied: '命令已复制',
    copy: '复制',
    labels: {
      hostname: '计算机名称', network: 'DNS 与域信息', account: '域账号', upn: '用户主体名称',
      dc: '查找域控制器', baseDn: 'Base DN', ldap: '测试 LDAP 端口', ldaps: '测试 LDAPS 端口',
    },
  },
} as const;

export default function ADConnectionCommandTips({
  locale,
  defaultOpen = false,
}: {
  locale: 'en' | 'zh-CN';
  defaultOpen?: boolean;
}) {
  const labels = copy[locale];
  const [copied, setCopied] = useState<string | null>(null);

  const copyCommand = async (id: string, command: string) => {
    await navigator.clipboard.writeText(command);
    setCopied(id);
  };

  return (
    <details className="group rounded-lg border border-line bg-surface-sunken" open={defaultOpen}>
      <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2.5 text-xs font-semibold text-fg">
        <TerminalSquare className="h-4 w-4 text-accent-fg" />
        <span>{labels.title}</span>
        <ChevronDown className="ml-auto h-4 w-4 text-fg-muted transition-transform group-open:rotate-180" />
      </summary>
      <div className="border-t border-line px-3 pb-3 pt-2.5">
        <p className="text-2xs leading-5 text-fg-muted">{labels.description}</p>
        <div className="mt-2 space-y-1.5">
          {AD_DISCOVERY_COMMANDS.map((item) => (
            <div key={item.id} className="grid items-center gap-2 rounded-md border border-line bg-surface-base px-2.5 py-2 sm:grid-cols-[minmax(130px,0.65fr)_minmax(0,1.35fr)_32px]">
              <span className="text-2xs font-medium text-fg-secondary">{labels.labels[item.id]}</span>
              <code className="min-w-0 select-all overflow-x-auto whitespace-nowrap font-mono text-2xs text-fg">{item.command}</code>
              <button
                type="button"
                onClick={() => copyCommand(item.id, item.command)}
                className={cn('flex h-7 w-7 items-center justify-center rounded-md text-fg-muted hover:bg-surface-sunken hover:text-fg', copied === item.id && 'text-success')}
                aria-label={`${labels.copy} ${item.id} command`}
                title={labels.copy}
              >
                {copied === item.id ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
              </button>
            </div>
          ))}
        </div>
        <div className="mt-2 flex gap-2 rounded-md border border-info/30 bg-info-soft px-2.5 py-2 text-2xs leading-5 text-fg-secondary">
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-info" />
          <div><p>{labels.note}</p><p className="mt-1">{labels.password}</p></div>
        </div>
        <p className="sr-only" aria-live="polite">{copied ? labels.copied : ''}</p>
      </div>
    </details>
  );
}
