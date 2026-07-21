import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';

import { I18nProvider, useI18n } from './I18nContext';

const STORAGE_KEY = 'fsa.locale';

function LocaleProbe() {
  const { locale, setLocale } = useI18n();

  return (
    <div>
      <span data-testid="locale">{locale}</span>
      <button type="button" onClick={() => setLocale('zh-CN')}>
        Switch to Chinese
      </button>
    </div>
  );
}

describe('I18nProvider', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.lang = '';
    jest.restoreAllMocks();
  });

  it('uses a saved locale on the first render without overwriting it with English', () => {
    window.localStorage.setItem(STORAGE_KEY, 'zh-CN');
    const setItemSpy = jest.spyOn(Storage.prototype, 'setItem');

    render(
      <I18nProvider>
        <LocaleProbe />
      </I18nProvider>,
    );

    expect(screen.getByTestId('locale')).toHaveTextContent('zh-CN');
    expect(setItemSpy).not.toHaveBeenCalledWith(STORAGE_KEY, 'en');
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('zh-CN');
    expect(document.documentElement.lang).toBe('zh-CN');
  });

  it('persists an explicit locale change', () => {
    render(
      <I18nProvider>
        <LocaleProbe />
      </I18nProvider>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Switch to Chinese' }));

    expect(screen.getByTestId('locale')).toHaveTextContent('zh-CN');
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('zh-CN');
    expect(document.documentElement.lang).toBe('zh-CN');
  });
});
