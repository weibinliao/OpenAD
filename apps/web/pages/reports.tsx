import Head from 'next/head';
import ReportCenterWorkspace from '../components/ReportCenterWorkspace';
import { useI18n } from '../contexts/I18nContext';

export default function ReportsPage() {
  const { locale } = useI18n();
  const title = locale === 'zh-CN' ? '报告中心 · OpenAD' : 'Report Center · OpenAD';

  return (
    <>
      <Head><title>{title}</title></Head>
      <ReportCenterWorkspace />
    </>
  );
}
