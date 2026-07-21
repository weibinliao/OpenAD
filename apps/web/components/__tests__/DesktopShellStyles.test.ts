import fs from 'fs';
import path from 'path';

const styles = fs.readFileSync(
  path.join(process.cwd(), 'styles', 'desktop-shell.css'),
  'utf8',
);

describe('desktop shell runtime status styles', () => {
  it('keeps runtime copy bounded inside the expanded status card', () => {
    expect(styles).toMatch(/\.pp-runtime-copy\s*\{[^}]*min-width:\s*0;[^}]*overflow:\s*hidden;/s);
    expect(styles).toMatch(/\.pp-runtime-copy strong\s*\{[^}]*text-overflow:\s*ellipsis;/s);
    expect(styles).toMatch(/\.pp-runtime-copy small\s*\{[^}]*white-space:\s*nowrap;/s);
  });

  it('styles health states and the runtime detail surface', () => {
    expect(styles).toContain('.pp-runtime-trigger.is-healthy .pp-runtime-dot');
    expect(styles).toContain('.pp-runtime-trigger.is-offline .pp-runtime-dot');
    expect(styles).toContain('.pp-runtime-menu-status');
    expect(styles).toContain('.pp-runtime-detail-row');
  });

  it('shows the expected resize cursor on every window edge and corner', () => {
    expect(styles).toMatch(/\.desktop-resize-handle\.is-top[^}]*cursor:\s*ns-resize;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-right[^}]*cursor:\s*ew-resize;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-bottom[^}]*cursor:\s*ns-resize;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-left[^}]*cursor:\s*ew-resize;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-top-left[^}]*cursor:\s*nwse-resize;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-top-right[^}]*cursor:\s*nesw-resize;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-bottom-right[^}]*cursor:\s*nwse-resize;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-bottom-left[^}]*cursor:\s*nesw-resize;/s);
  });

  it('anchors generous WebView resize handles to every outer edge and corner', () => {
    expect(styles).toMatch(/\.desktop-resize-handle\.is-right[^}]*right:\s*0;[^}]*width:\s*12px;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-bottom[^}]*bottom:\s*0;[^}]*height:\s*12px;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-left[^}]*left:\s*0;[^}]*width:\s*12px;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-top[^}]*top:\s*0;[^}]*height:\s*12px;/s);
    expect(styles).toMatch(/\.desktop-resize-handle\.is-top-left[^}]*top:\s*0;[^}]*left:\s*0;[^}]*width:\s*20px;[^}]*height:\s*20px;/s);
  });

  it('keeps resize hit areas visually hidden in every interaction state', () => {
    expect(styles).toMatch(/\.desktop-resize-handle\s*\{[^}]*background:\s*transparent;[^}]*opacity:\s*0;/s);
    expect(styles).not.toContain('.desktop-resize-handle::after');
    expect(styles).not.toContain('.desktop-resize-handle:hover');
  });

  it('does not reserve a redundant branded header above the navigation rail', () => {
    expect(styles).not.toContain('.pp-rail-header');
    expect(styles).not.toContain('.pp-rail-brand');
    expect(styles).toMatch(/\.pp-rail-pin\s*\{[^}]*position:\s*absolute;[^}]*top:\s*8px;[^}]*right:\s*8px;/s);
  });
});
