import { fireEvent, render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { deriveScanStage, ScanReviewTabs, ScanWorkflowNavigation } from '../ScanWorkspaceFlow';

describe('ScanWorkspaceFlow', () => {
  test('derives the active stage from scan state', () => {
    expect(deriveScanStage({ path: '', running: false, failed: false, hasResults: false, imported: false })).toBe('target');
    expect(deriveScanStage({ path: 'C:\\Data', running: false, failed: false, hasResults: false, imported: false })).toBe('configure');
    expect(deriveScanStage({ path: 'C:\\Data', running: true, failed: false, hasResults: false, imported: false })).toBe('review');
    expect(deriveScanStage({ path: '', running: false, failed: false, hasResults: false, imported: true })).toBe('review');
  });

  test('renders a clear three-stage workflow', () => {
    render(<ScanWorkflowNavigation locale="en" activeStage="configure" />);

    expect(screen.getByText('Select target')).toBeInTheDocument();
    expect(screen.getByText('Configure')).toHaveAttribute('aria-current', 'step');
    expect(screen.getByText('Run and review')).toBeInTheDocument();
  });

  test('changes the active review surface', () => {
    const onChange = jest.fn();
    render(<ScanReviewTabs locale="en" value="overview" onChange={onChange} />);

    fireEvent.click(screen.getByRole('tab', { name: 'ACL Evidence' }));

    expect(onChange).toHaveBeenCalledWith('acl');
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true');
  });
});
