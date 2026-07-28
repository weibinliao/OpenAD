import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import ResourcesPage from '../../pages/resources';
import { I18nProvider } from '../../contexts/I18nContext';

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({ activeProfile: null }),
}));

describe('ResourcesPage states', () => {
  beforeEach(() => {
    window.localStorage.setItem('fsa.locale', 'en');
    global.fetch = jest.fn().mockRejectedValue(new Error('offline')) as jest.MockedFunction<typeof fetch>;
  });

  afterEach(() => jest.clearAllMocks());

  test('shows the AD prerequisite and a retryable resource-list error', async () => {
    render(<I18nProvider><ResourcesPage /></I18nProvider>);

    expect(screen.getByText('Connect AD in Settings to resolve user and group names.')).toBeInTheDocument();
    expect(await screen.findByText('Resource list unavailable')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Retry loading resources' }));
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(2));
  });
});
