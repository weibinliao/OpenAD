import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import PermissionViewerQuickReports from '../PermissionViewerQuickReports';

describe('Permission report deep links', () => {
  it('opens the report mode requested by the OpenAD overview', () => {
    render(
      <PermissionViewerQuickReports
        locale="zh-CN"
        permissions={[]}
        adReady={false}
        exactPathCount={0}
        exportReady={false}
        initialMode="owner"
        onApplyTrusteeFilter={jest.fn()}
        onApplyPathFilter={jest.fn()}
        onClearFilters={jest.fn()}
        onOpenReportControls={jest.fn()}
        onOpenHistory={jest.fn()}
      />,
    );

    expect(screen.getByRole('tab', { name: '责任人复核' })).toHaveAttribute('aria-selected', 'true');
  });

  it('notifies the report center when the operator switches modes', () => {
    const onModeChange = jest.fn();
    render(
      <PermissionViewerQuickReports
        locale="en"
        permissions={[]}
        adReady={false}
        exactPathCount={0}
        exportReady={false}
        mode="user"
        onModeChange={onModeChange}
        onApplyTrusteeFilter={jest.fn()}
        onApplyPathFilter={jest.fn()}
        onClearFilters={jest.fn()}
        onOpenReportControls={jest.fn()}
        onOpenHistory={jest.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('tab', { name: 'Folder Report' }));
    expect(onModeChange).toHaveBeenCalledWith('folder');
  });

  it('removes the duplicate report-workspace heading when embedded in the report center', () => {
    const { container } = render(
      <PermissionViewerQuickReports
        locale="en"
        permissions={[]}
        adReady={false}
        exactPathCount={0}
        exportReady={false}
        embedded
        onApplyTrusteeFilter={jest.fn()}
        onApplyPathFilter={jest.fn()}
        onClearFilters={jest.fn()}
        onOpenReportControls={jest.fn()}
        onOpenHistory={jest.fn()}
      />,
    );

    expect(screen.queryByText('Report workspace')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'User, folder, and owner reports' })).not.toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'User Access' })).toBeInTheDocument();
    expect(container.querySelector('section')).toHaveClass('permission-quick-reports-embedded');
  });

  it('localizes risk labels in the Chinese report table', () => {
    render(
      <PermissionViewerQuickReports
        locale="zh-CN"
        permissions={[{
          path: '\\\\fs01\\Finance',
          trustee: 'EXAMPLE\\alice',
          trustee_sid: 'S-1-5-21-1',
          rights: 'Modify',
          type: 'Allow',
          inherited: false,
          risk_level: 'high',
        }]}
        adReady
        exactPathCount={1}
        exportReady
        embedded
        onApplyTrusteeFilter={jest.fn()}
        onApplyPathFilter={jest.fn()}
        onClearFilters={jest.fn()}
        onOpenReportControls={jest.fn()}
        onOpenHistory={jest.fn()}
      />,
    );

    expect(screen.getByText('高')).toBeInTheDocument();
    expect(screen.queryByText('high')).not.toBeInTheDocument();
  });
});
