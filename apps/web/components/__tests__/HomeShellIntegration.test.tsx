import React from 'react';
import { render, screen } from '@testing-library/react';
import HomePage from '../../pages/index';

const push = jest.fn();

jest.mock('next/router', () => ({
  useRouter: () => ({ push }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'zh-CN' }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    activeProfile: null,
    connection: { connected: false },
  }),
}));

jest.mock('../../hooks/useRuntimeHealth', () => ({
  useRuntimeHealth: () => 'checking',
}));

jest.mock('../shell/DesktopWindowFrame', () => {
  const ReactModule = jest.requireActual<typeof import('react')>('react');
  return {
    __esModule: true,
    default: ({ children }: { children: React.ReactNode }) => ReactModule.createElement(
      'div',
      { 'data-testid': 'legacy-home-frame' },
      children,
    ),
    useDesktopOSStyle: () => ['windows', jest.fn()],
  };
});

describe('home desktop shell integration', () => {
  beforeEach(() => {
    push.mockReset();
    global.fetch = jest.fn(() => new Promise(() => {})) as jest.Mock;
  });

  it('keeps the home route inside the shared application shell', () => {
    expect((HomePage as typeof HomePage & { desktopExperience?: boolean }).desktopExperience).not.toBe(true);
  });

  it('does not render a nested legacy desktop frame', () => {
    render(<HomePage />);

    expect(screen.queryByTestId('legacy-home-frame')).not.toBeInTheDocument();
  });
});
