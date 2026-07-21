import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import DesktopWindowFrame from '../shell/DesktopWindowFrame';

describe('DesktopWindowFrame', () => {
  const postMessage = jest.fn();

  beforeEach(() => {
    postMessage.mockReset();
    Object.defineProperty(window, 'chrome', {
      configurable: true,
      value: { webview: { postMessage } },
    });
  });

  it('shows symbols inside all macOS window controls', () => {
    const { container } = render(
      <DesktopWindowFrame theme="dark" osStyle="mac">
        <div>Workspace</div>
      </DesktopWindowFrame>,
    );

    const controls = container.querySelector('.mac-controls');
    expect(controls).not.toBeNull();
    const macControls = within(controls as HTMLElement);
    expect(macControls.getByRole('button', { name: 'Close window' })).toHaveAttribute('data-symbol', 'close');
    expect(macControls.getByRole('button', { name: 'Minimize window' })).toHaveAttribute('data-symbol', 'minimize');
    expect(macControls.getByRole('button', { name: 'Maximize window' })).toHaveAttribute('data-symbol', 'maximize');
    expect(screen.getByTestId('mac-close-symbol')).toBeVisible();
    expect(screen.getByTestId('mac-minimize-symbol')).toBeVisible();
    expect(screen.getByTestId('mac-maximize-symbol')).toBeVisible();
  });

  it('keeps the desktop WebView message contract for window commands', () => {
    const { container } = render(
      <DesktopWindowFrame theme="dark" osStyle="windows">
        <div>Workspace</div>
      </DesktopWindowFrame>,
    );

    const controls = container.querySelector('.win-controls');
    expect(controls).not.toBeNull();
    const windowsControls = within(controls as HTMLElement);
    fireEvent.click(windowsControls.getByRole('button', { name: 'Minimize window' }));
    fireEvent.click(windowsControls.getByRole('button', { name: 'Maximize window' }));
    fireEvent.click(windowsControls.getByRole('button', { name: 'Close window' }));

    const windowMessages = postMessage.mock.calls
      .map(([message]) => message)
      .filter((message) => message.type === 'permission-protector-window');
    expect(windowMessages).toEqual([
      { type: 'permission-protector-window', action: 'minimize' },
      { type: 'permission-protector-window', action: 'maximize' },
      { type: 'permission-protector-window', action: 'close' },
    ]);
  });

  it('renders one platform-specific title to avoid duplicated accessible branding', () => {
    const { container } = render(
      <DesktopWindowFrame theme="dark" osStyle="windows" title="OpenAD">
        <div>Workspace</div>
      </DesktopWindowFrame>,
    );

    expect(container.querySelector('.win-title')).toHaveTextContent('OpenAD');
    expect(container.querySelector('.mac-title')).toBeNull();
  });

  it('exposes user-operated resize handles for every edge and corner', () => {
    const { container } = render(
      <DesktopWindowFrame theme="dark" osStyle="windows">
        <div>Workspace</div>
      </DesktopWindowFrame>,
    );

    const handles = Array.from(container.querySelectorAll<HTMLElement>('[data-resize-direction]'));
    expect(handles.map((handle) => handle.dataset.resizeDirection)).toEqual([
      'top',
      'right',
      'bottom',
      'left',
      'top-left',
      'top-right',
      'bottom-right',
      'bottom-left',
    ]);

    fireEvent.mouseDown(handles[1], {
      button: 0,
      screenX: 1280,
      screenY: 420,
    });
    fireEvent.mouseMove(window, {
      buttons: 1,
      screenX: 1180,
      screenY: 420,
    });
    fireEvent.mouseUp(window, {
      button: 0,
      screenX: 1180,
      screenY: 420,
    });

    const resizeMessages = postMessage.mock.calls
      .map(([message]) => message)
      .filter((message) => message.type === 'permission-protector-window' && message.action.startsWith('resize-'));
    expect(resizeMessages).toEqual([
      {
        type: 'permission-protector-window',
        action: 'resize-start',
        direction: 'right',
        screenX: 1280,
        screenY: 420,
        scaleFactor: 1,
      },
      {
        type: 'permission-protector-window',
        action: 'resize-move',
        screenX: 1180,
        screenY: 420,
      },
      {
        type: 'permission-protector-window',
        action: 'resize-end',
      },
    ]);
  });

  it('synchronizes a raster workspace logo to the native desktop host', () => {
    const logoDataUrl = 'data:image/png;base64,ZmFrZS1sb2dv';

    render(
      <DesktopWindowFrame theme="dark" osStyle="windows" logoSrc={logoDataUrl} logoDataUrl={logoDataUrl}>
        <div>Workspace</div>
      </DesktopWindowFrame>,
    );

    expect(postMessage).toHaveBeenCalledWith({
      type: 'permission-protector-branding',
      logoDataUrl,
    });
  });
});
