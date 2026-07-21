import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import OpenADOverview from '../../pages/index';

const push = jest.fn();

jest.mock('next/router', () => ({
  useRouter: () => ({ push }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'zh-CN' }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    activeProfile: { name: 'dc01.example.com' },
    connection: { connected: true },
  }),
}));

jest.mock('../../hooks/useRuntimeHealth', () => ({
  useRuntimeHealth: () => 'healthy',
}));

jest.mock('../../lib/riskFindings', () => ({
  RISK_FINDINGS_UPDATED_EVENT: 'risk-findings-updated',
  readRiskFindings: () => [
    {
      id: 'risk-1',
      severity: 'high',
      status: 'open',
      title: 'Everyone 可修改财务共享',
      path: '\\\\fs01\\Finance',
      trustee: 'Everyone',
      rights: 'Modify',
      suggestedAction: '替换宽泛主体',
    },
  ],
  summarizeRiskFindings: () => ({
    total: 1,
    critical: 0,
    high: 1,
    medium: 0,
    low: 0,
    open: 1,
    exposureScore: 8,
  }),
}));

describe('OpenAD operations overview', () => {
  beforeEach(() => {
    push.mockReset();
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        items: [
          {
            id: 'scan-1',
            root_path: '\\\\fs01\\Finance',
            status: 'completed',
            max_depth: 6,
            include_inherited: true,
            items_scanned: 126,
            permission_count: 1741,
            started_at: '2026-07-11T01:00:00.000Z',
            finished_at: '2026-07-11T01:05:00.000Z',
          },
        ],
        pagination: { total: 1 },
      }),
    }) as jest.Mock;
  });

  it('organizes the first screen around operator status and work', async () => {
    render(<OpenADOverview />);

    expect(screen.getByRole('heading', { name: 'OpenAD 运维总览' })).toBeInTheDocument();
    expect(screen.getByText('环境状态')).toBeInTheDocument();
    expect(screen.getByText('今日操作')).toBeInTheDocument();
    expect(screen.getByText('需要关注')).toBeInTheDocument();
    expect(screen.getByText('用户权限报告')).toBeInTheDocument();
    expect(screen.getByText('文件夹权限报告')).toBeInTheDocument();
    expect(screen.getByText('所有者报告')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByText('\\\\fs01\\Finance')).toBeInTheDocument());
  });

  it('routes primary work and report actions to existing modules', async () => {
    render(<OpenADOverview />);

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: /^开始权限扫描/ }));
    expect(push).toHaveBeenCalledWith('/scan-workspace');

    fireEvent.click(screen.getByRole('button', { name: /^用户权限报告/ }));
    expect(push).toHaveBeenCalledWith('/reports?mode=user');
  });
});
