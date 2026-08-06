import fs from 'fs';
import path from 'path';

describe('OpenAD operations workspace styles', () => {
  const css = fs.readFileSync(
    path.join(process.cwd(), 'styles', 'openad-operations.css'),
    'utf8',
  );

  it('defines stable navigation descriptions and operational layouts', () => {
    expect(css).toContain('.pp-rail-description');
    expect(css).toContain('.pp-readonly-badge');
    expect(css).toContain('.openad-overview-status');
    expect(css).toContain('.openad-operations-grid');
    expect(css).toContain('.openad-reports-grid');
  });

  it('collapses the overview to one column at narrow widths', () => {
    expect(css).toMatch(/@media \(max-width: 900px\)[\s\S]*\.openad-operations-grid/);
    expect(css).toMatch(/@media \(max-width: 900px\)[\s\S]*\.openad-evidence-grid/);
  });

  it('keeps the quick AD connection form readable and aligned on desktop', () => {
    expect(css).toMatch(/\.openad-quick-connect-card \.openad-quick-connect-header p \{[\s\S]*font-size: 11px;[\s\S]*line-height: 16px;/);
    expect(css).toMatch(/\.openad-quick-connect-card \.openad-quick-connect-content \{[\s\S]*display: flex;[\s\S]*gap: 12px;/);
    expect(css).toMatch(/\.openad-quick-connect-card \.openad-quick-connect-field-hint \{[\s\S]*min-height: 32px;[\s\S]*line-height: 16px;/);
    expect(css).toMatch(/\.openad-quick-connect-card \.openad-quick-connect-content \{[\s\S]*grid-template-columns: minmax\(240px, 1\.2fr\)[\s\S]*align-items: start;/);
    expect(css).toMatch(/\.openad-quick-connect-card \.openad-quick-connect-content > button \{[\s\S]*align-self: start;[\s\S]*min-width: 112px;[\s\S]*margin-top: 20px;/);
  });
});
