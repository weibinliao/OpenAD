import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import OpenADOverview from '../../pages/index';

const push = jest.fn();
const mockRefreshProfiles = jest.fn();
const mockSetActiveProfileId = jest.fn();
let mockADConnected = true;
let mockRiskFindings = [
  {
    id: 'risk-1',
    fingerprint: 'risk-1',
    severity: 'high',
    status: 'open',
    type: 'broad-access',
    title: 'Everyone 可修改财务共享',
    path: '\\\\fs01\\Finance',
    trustee: 'Everyone',
    trusteeSid: 'S-1-1-0',
    rights: 'Modify',
    inherited: false,
    source: 'scan',
    firstSeenAt: '2026-07-11T01:00:00.000Z',
    lastSeenAt: '2026-07-11T01:00:00.000Z',
    seenCount: 1,
    suggestedAction: '替换宽泛主体',
  },
];

jest.mock('next/router', () => ({
  useRouter: () => ({ push }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'zh-CN' }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    activeProfile: mockADConnected ? { name: 'dc01.example.com' } : null,
    connection: { connected: mockADConnected },
    refreshProfiles: mockRefreshProfiles,
    setActiveProfileId: mockSetActiveProfileId,
  }),
}));

jest.mock('../../hooks/useRuntimeHealth', () => ({
  useRuntimeHealth: () => 'healthy',
}));

jest.mock('../../lib/riskFindings', () => ({
  ...jest.requireActual('../../lib/riskFindings'),
  loadRiskFindings: () => Promise.resolve(mockRiskFindings),
}));

describe('OpenAD operations overview', () => {
  beforeEach(() => {
    push.mockReset();
    mockRefreshProfiles.mockReset();
    mockSetActiveProfileId.mockReset();
    mockADConnected = true;
    mockRiskFindings = [
      {
        id: 'risk-1',
        fingerprint: 'risk-1',
        severity: 'high',
        status: 'open',
        type: 'broad-access',
        title: 'Everyone 可修改财务共享',
        path: '\\\\fs01\\Finance',
        trustee: 'Everyone',
        trusteeSid: 'S-1-1-0',
        rights: 'Modify',
        inherited: false,
        source: 'scan',
        firstSeenAt: '2026-07-11T01:00:00.000Z',
        lastSeenAt: '2026-07-11T01:00:00.000Z',
        seenCount: 1,
        suggestedAction: '替换宽泛主体',
      },
    ];
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

    await waitFor(() => expect(screen.getAllByText('\\\\fs01\\Finance').length).toBeGreaterThan(0));
  });

  it('routes primary work and report actions to existing modules', async () => {
    render(<OpenADOverview />);

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: /^开始权限扫描/ }));
    expect(push).toHaveBeenCalledWith('/scan-workspace');

    fireEvent.click(screen.getByRole('button', { name: /^用户权限报告/ }));
    expect(push).toHaveBeenCalledWith('/reports?mode=user');
  });

  it('shows a retryable error instead of unlabelled sample sessions when history is unavailable', async () => {
    (global.fetch as jest.Mock).mockRejectedValue(new Error('offline'));

    render(<OpenADOverview />);

    expect(await screen.findByText('最近活动暂不可用')).toBeInTheDocument();
    expect(screen.queryByText('\\\\fs01\\Finance')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '重试加载最近活动' }));
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(2));
  });

  it('puts the highest-priority open risks in the attention panel', async () => {
    mockRiskFindings = [
      { ...mockRiskFindings[0], id: 'low', fingerprint: 'low', severity: 'low', title: '低风险' },
      { ...mockRiskFindings[0], id: 'medium', fingerprint: 'medium', severity: 'medium', title: '中风险' },
      { ...mockRiskFindings[0], id: 'high', fingerprint: 'high', severity: 'high', title: '高风险' },
      { ...mockRiskFindings[0], id: 'critical', fingerprint: 'critical', severity: 'critical', title: '严重风险' },
    ];

    render(<OpenADOverview />);

    expect(await screen.findByText('严重风险')).toBeInTheDocument();
    expect(screen.queryByText('低风险')).not.toBeInTheDocument();
  });

  it('shows the normalized risk distribution alongside the attention queue', async () => {
    mockRiskFindings = [
      { ...mockRiskFindings[0], id: 'critical', fingerprint: 'critical', severity: 'critical' },
      { ...mockRiskFindings[0], id: 'high', fingerprint: 'high', severity: 'high' },
      { ...mockRiskFindings[0], id: 'medium', fingerprint: 'medium', severity: 'medium' },
      { ...mockRiskFindings[0], id: 'low', fingerprint: 'low', severity: 'low' },
    ];

    render(<OpenADOverview />);

    expect(await screen.findByText('风险级别分布')).toBeInTheDocument();
    expect(screen.getByTestId('risk-bar-critical')).toHaveStyle({ width: '100%' });
  });

  it('shows quick AD connection only while no connection is active', async () => {
    mockADConnected = false;
    const { rerender } = render(<OpenADOverview />);

    expect(await screen.findByText('连接你的域')).toBeInTheDocument();

    mockADConnected = true;
    rerender(<OpenADOverview />);
    expect(screen.queryByText('连接你的域')).not.toBeInTheDocument();
  });

  it('includes latest permission count and completion time in environment status', async () => {
    render(<OpenADOverview />);

    expect(await screen.findByText(/1,741 个权限项/)).toBeInTheDocument();
    expect(screen.getByText(/完成于/)).toBeInTheDocument();
  });
});
