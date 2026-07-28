import { riskLabel, scanStatusLabel } from '../ui/badge';

describe('shared badge labels', () => {
  test('localizes backend risk and scan status values', () => {
    expect(riskLabel('critical', 'en')).toBe('Critical');
    expect(riskLabel('high', 'zh-CN')).toBe('高');
    expect(scanStatusLabel('completed', 'en')).toBe('Completed');
    expect(scanStatusLabel('failed', 'zh-CN')).toBe('失败');
  });
});
