import { fireEvent, render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import FileActivityPage from '../../pages/file-activity';

const push = jest.fn();

jest.mock('next/router', () => ({
  useRouter: () => ({ push }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'en', t: (key: string) => key }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    config: { adServer: '', baseDN: '', username: '', password: '' },
    connection: { connected: false },
  }),
}));

describe('FileActivityPage prerequisites', () => {
  beforeEach(() => {
    push.mockReset();
    global.fetch = jest.fn(() => new Promise(() => {})) as jest.MockedFunction<typeof fetch>;
  });

  test('keeps the readiness layout stable and opens AD settings', () => {
    render(<FileActivityPage />);

    expect(screen.getByRole('status', { name: 'Checking file activity prerequisites' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Connect AD' }));
    expect(push).toHaveBeenCalledWith('/settings');
  });
});
