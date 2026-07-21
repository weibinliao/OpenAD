import { fireEvent, render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import ScanCompletionSummary from '../ScanCompletionSummary';

const push = jest.fn();

jest.mock('next/router', () => ({
  useRouter: () => ({ push }),
}));

describe('ScanCompletionSummary', () => {
  beforeEach(() => push.mockClear());

  test('shows the completed scan facts and routes follow-up work to owning modules', () => {
    const onRescan = jest.fn();

    render(
      <ScanCompletionSummary
        locale="en"
        path={'\\\\fileserver\\finance'}
        sessionID="session-1"
        itemsScanned={875}
        permissionCount={2461}
        skippedCount={3}
        riskCount={12}
        onRescan={onRescan}
      />,
    );

    expect(screen.getByText('875')).toBeInTheDocument();
    expect(screen.getByText('2,461')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('12')).toBeInTheDocument();
    expect(screen.getByText(/session-1/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'View permission evidence' }));
    expect(push).toHaveBeenLastCalledWith('/access?path=%5C%5Cfileserver%5Cfinance');

    fireEvent.click(screen.getByRole('button', { name: 'View risk findings' }));
    expect(push).toHaveBeenLastCalledWith('/findings');

    fireEvent.click(screen.getByRole('button', { name: 'Open scan history' }));
    expect(push).toHaveBeenLastCalledWith('/history');

    fireEvent.click(screen.getByRole('button', { name: 'Generate report' }));
    expect(push).toHaveBeenLastCalledWith(
      '/reports?mode=user&path=%5C%5Cfileserver%5Cfinance&session=session-1',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Run again' }));
    expect(onRescan).toHaveBeenCalledTimes(1);
  });
});
