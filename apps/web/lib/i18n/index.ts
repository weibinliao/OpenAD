import { useI18n } from '../../contexts/I18nContext';
import { en, type Dict } from './en';
import { zh } from './zh';

const dicts: Record<'en' | 'zh', Dict> = { en, zh };

/**
 * Typed dictionary hook for redesigned pages.
 * Usage: const d = useDict(); d.nav.dashboard
 */
export function useDict(): Dict {
  const { locale } = useI18n();
  return dicts[locale === 'zh-CN' ? 'zh' : 'en'];
}

export { en, zh };
export type { Dict };
