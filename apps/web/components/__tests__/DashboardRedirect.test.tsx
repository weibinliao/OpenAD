import React from 'react';
import { render, waitFor } from '@testing-library/react';
import DashboardPage from '../../pages/dashboard';

const replace = jest.fn();

jest.mock('next/router', () => ({
  useRouter: () => ({ replace }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'zh-CN' }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    activeProfile: { name: 'dc01.example.com' },
  }),
}));

describe('/dashboard redirect', () => {
  beforeEach(() => {
    replace.mockReset();
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ items: [], pagination: { total: 0 } }),
    }) as jest.Mock;
  });

  it('renders no legacy overview while replacing the route with home', async () => {
    const { container } = render(<DashboardPage />);

    expect(container).toBeEmptyDOMElement();
    await waitFor(() => expect(replace).toHaveBeenCalledWith('/'));
  });
});
