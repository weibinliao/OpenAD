import type { ComponentType } from 'react';
import {
  Activity,
  FileText,
  FolderTree,
  GitFork,
  History,
  Home,
  ScanLine,
  ScrollText,
  Settings,
  ShieldAlert,
  Users,
} from 'lucide-react';

export type OpenADLocale = 'zh-CN' | 'en';

export interface OpenADNavItem {
  key: string;
  label: string;
  description: string;
  href: string;
  icon: ComponentType<{ className?: string }>;
  aliases?: string[];
}

export interface OpenADNavGroup {
  key: string;
  label: string;
  items: OpenADNavItem[];
}

interface LocalizedCopy {
  zh: string;
  en: string;
}

function copy(locale: OpenADLocale, value: LocalizedCopy) {
  return locale === 'zh-CN' ? value.zh : value.en;
}

export function buildOpenADNavigation(locale: OpenADLocale): OpenADNavGroup[] {
  const item = (
    key: string,
    label: LocalizedCopy,
    description: LocalizedCopy,
    href: string,
    icon: OpenADNavItem['icon'],
    aliases?: string[],
  ): OpenADNavItem => ({
    key,
    label: copy(locale, label),
    description: copy(locale, description),
    href,
    icon,
    aliases,
  });

  return [
    {
      key: 'workbench',
      label: copy(locale, { zh: '工作台', en: 'Workbench' }),
      items: [
        item(
          'home',
          { zh: '总览', en: 'Overview' },
          { zh: '环境状态、待办与快捷操作', en: 'Environment, attention and quick actions' },
          '/',
          Home,
          ['/dashboard'],
        ),
      ],
    },
    {
      key: 'directory',
      label: copy(locale, { zh: '目录服务', en: 'Directory Service' }),
      items: [
        item(
          'identity',
          { zh: '目录浏览', en: 'Directory Explorer' },
          { zh: '用户、组、OU 与目录同步', en: 'Users, groups, OUs and sync' },
          '/identity',
          Users,
        ),
      ],
    },
    {
      key: 'permissions',
      label: copy(locale, { zh: '权限作业', en: 'Permission Operations' }),
      items: [
        item(
          'resources',
          { zh: '资源清单', en: 'Resource Inventory' },
          { zh: '文件夹、共享与报告入口', en: 'Folders, shares and report entry points' },
          '/resources',
          FolderTree,
        ),
        item(
          'scan',
          { zh: '扫描中心', en: 'Scan Center' },
          { zh: '创建、运行与查看权限扫描', en: 'Create, run and inspect permission scans' },
          '/scan-workspace',
          ScanLine,
          ['/ad-workspace'],
        ),
        item(
          'access',
          { zh: '访问分析', en: 'Access Explorer' },
          { zh: '按用户或资源解释有效权限', en: 'Explain effective access by identity or resource' },
          '/access',
          GitFork,
        ),
        item(
          'history',
          { zh: '扫描历史', en: 'Scan History' },
          { zh: '历史记录、差异与证据导出', en: 'History, comparison and evidence exports' },
          '/history',
          History,
        ),
      ],
    },
    {
      key: 'governance',
      label: copy(locale, { zh: '风险治理', en: 'Risk Governance' }),
      items: [
        item(
          'findings',
          { zh: '风险发现', en: 'Risk Findings' },
          { zh: '暴露风险、优先级与复核状态', en: 'Exposure risks, priority and review status' },
          '/findings',
          ShieldAlert,
          ['/risks'],
        ),
        item(
          'file-activity',
          { zh: '文件活动', en: 'File Activity' },
          { zh: 'Windows 文件访问使用证据', en: 'Windows file-access usage evidence' },
          '/file-activity',
          Activity,
        ),
      ],
    },
    {
      key: 'system',
      label: copy(locale, { zh: '系统与合规', en: 'System & Compliance' }),
      items: [
        item(
          'audit',
          { zh: '操作审计', en: 'Operation Audit' },
          { zh: 'OpenAD 请求与操作记录', en: 'OpenAD request and operation evidence' },
          '/audit',
          ScrollText,
          ['/audit/[id]'],
        ),
        item(
          'settings',
          { zh: '系统设置', en: 'System Settings' },
          { zh: 'AD 连接、外观与应用偏好', en: 'AD connections, appearance and app preferences' },
          '/settings',
          Settings,
        ),
      ],
    },
    {
      key: 'outputs',
      label: copy(locale, { zh: '输出', en: 'Outputs' }),
      items: [
        item(
          'reports',
          { zh: '报告中心', en: 'Report Center' },
          { zh: '报告模板、预览与多格式导出', en: 'Templates, previews and multi-format exports' },
          '/reports',
          FileText,
        ),
      ],
    },
  ];
}

export function matchesOpenADRoute(item: OpenADNavItem, pathname: string) {
  const candidates = [item.href, ...(item.aliases || [])];
  return candidates.some((candidate) => (
    pathname === candidate
    || (candidate !== '/' && !candidate.includes('[') && pathname.startsWith(`${candidate}/`))
  ));
}
