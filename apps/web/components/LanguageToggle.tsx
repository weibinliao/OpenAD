import { useI18n } from '../contexts/I18nContext';

export default function LanguageToggle() {
  const { locale, setLocale, t } = useI18n();
  const chineseAriaLabel = locale === 'zh-CN' ? '切换语言为中文' : 'Switch language to Chinese';
  const englishAriaLabel = locale === 'zh-CN' ? '切换语言为英文' : 'Switch language to English';

  return (
    <div className="control-strip inline-flex items-center gap-1.5 rounded-full p-1">
      <span className="text-xs text-muted-foreground">{t('langLabel')}</span>
      <button
        type="button"
        aria-label={chineseAriaLabel}
        aria-pressed={locale === 'zh-CN'}
        className="action-chip min-h-0 px-2.5 py-1 text-xs"
        onClick={() => setLocale('zh-CN')}
      >
        ZH
      </button>
      <button
        type="button"
        aria-label={englishAriaLabel}
        aria-pressed={locale === 'en'}
        className="action-chip min-h-0 px-2.5 py-1 text-xs"
        onClick={() => setLocale('en')}
      >
        EN
      </button>
    </div>
  );
}
