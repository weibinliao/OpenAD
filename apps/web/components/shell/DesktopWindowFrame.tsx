import React, {
  useCallback,
  useEffect,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
} from 'react';
import { Layers, Maximize2, Minus, Square, X } from 'lucide-react';
import { cn } from '../../lib/cn';

export type DesktopTheme = 'light' | 'dark' | 'geek';
export type DesktopOSStyle = 'windows' | 'mac';
export type DesktopWindowCommand = 'drag' | 'minimize' | 'maximize' | 'close';
export type DesktopResizeDirection =
  | 'top'
  | 'right'
  | 'bottom'
  | 'left'
  | 'top-left'
  | 'top-right'
  | 'bottom-right'
  | 'bottom-left';

declare global {
  interface Window {
    chrome?: {
      webview?: {
        postMessage: (message: unknown) => void;
      };
    };
  }
}

const OS_STYLE_STORAGE_KEY = 'permission-protector.desktop-os-style';
const RESIZE_DIRECTIONS: DesktopResizeDirection[] = [
  'top',
  'right',
  'bottom',
  'left',
  'top-left',
  'top-right',
  'bottom-right',
  'bottom-left',
];

export function useDesktopOSStyle(defaultStyle: DesktopOSStyle = 'windows') {
  const [osStyle, setOSStyle] = useState<DesktopOSStyle>(defaultStyle);

  useEffect(() => {
    const stored = window.localStorage.getItem(OS_STYLE_STORAGE_KEY);
    if (stored === 'windows' || stored === 'mac') {
      setOSStyle(stored);
    }
  }, []);

  const updateOSStyle = useCallback((nextStyle: DesktopOSStyle) => {
    setOSStyle(nextStyle);
    window.localStorage.setItem(OS_STYLE_STORAGE_KEY, nextStyle);
  }, []);

  return [osStyle, updateOSStyle] as const;
}

export function sendDesktopWindowCommand(action: DesktopWindowCommand) {
  if (typeof window === 'undefined') return;
  window.chrome?.webview?.postMessage({ type: 'permission-protector-window', action });
}

export function sendDesktopWindowResizeStart(
  direction: DesktopResizeDirection,
  screenX: number,
  screenY: number,
) {
  if (typeof window === 'undefined') return;
  window.chrome?.webview?.postMessage({
    type: 'permission-protector-window',
    action: 'resize-start',
    direction,
    screenX,
    screenY,
    scaleFactor: window.devicePixelRatio || 1,
  });
}

export function sendDesktopWindowResizeMove(screenX: number, screenY: number) {
  if (typeof window === 'undefined') return;
  window.chrome?.webview?.postMessage({
    type: 'permission-protector-window',
    action: 'resize-move',
    screenX,
    screenY,
  });
}

export function sendDesktopWindowResizeEnd() {
  if (typeof window === 'undefined') return;
  window.chrome?.webview?.postMessage({
    type: 'permission-protector-window',
    action: 'resize-end',
  });
}

interface DesktopWindowFrameProps {
  children: ReactNode;
  theme: DesktopTheme;
  osStyle: DesktopOSStyle;
  className?: string;
  contentClassName?: string;
  title?: string;
  logoSrc?: string;
  logoDataUrl?: string;
}

export default function DesktopWindowFrame({
  children,
  theme,
  osStyle,
  className,
  contentClassName,
  title = 'OpenAD',
  logoSrc,
  logoDataUrl = '',
}: DesktopWindowFrameProps) {
  const activeResizeDirection = useRef<DesktopResizeDirection | null>(null);

  useEffect(() => {
    window.chrome?.webview?.postMessage({
      type: 'permission-protector-branding',
      logoDataUrl,
    });
  }, [logoDataUrl]);

  useEffect(() => {
    const continueWindowResize = (event: globalThis.MouseEvent) => {
      if (!activeResizeDirection.current || (event.buttons & 1) !== 1) return;
      event.preventDefault();
      sendDesktopWindowResizeMove(event.screenX, event.screenY);
    };

    const endWindowResize = (event: globalThis.MouseEvent) => {
      if (!activeResizeDirection.current) return;
      activeResizeDirection.current = null;
      event.preventDefault();
      sendDesktopWindowResizeEnd();
    };

    window.addEventListener('mousemove', continueWindowResize);
    window.addEventListener('mouseup', endWindowResize);
    window.addEventListener('blur', sendDesktopWindowResizeEnd);
    return () => {
      window.removeEventListener('mousemove', continueWindowResize);
      window.removeEventListener('mouseup', endWindowResize);
      window.removeEventListener('blur', sendDesktopWindowResizeEnd);
    };
  }, []);

  const beginWindowDrag = (event: MouseEvent<HTMLElement>) => {
    if (event.button !== 0 || (event.target as HTMLElement).closest('button')) return;
    sendDesktopWindowCommand('drag');
  };

  const beginWindowResize = (
    event: MouseEvent<HTMLElement>,
    direction: DesktopResizeDirection,
  ) => {
    if (event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    activeResizeDirection.current = direction;
    sendDesktopWindowResizeStart(direction, event.screenX, event.screenY);
  };

  return (
    <main className={cn('openad-native', className)} data-theme={theme} data-os={osStyle}>
      <section className="app-window">
        {RESIZE_DIRECTIONS.map((direction) => (
          <div
            key={direction}
            className={`desktop-resize-handle is-${direction}`}
            data-resize-direction={direction}
            aria-hidden="true"
            onMouseDown={(event) => beginWindowResize(event, direction)}
          />
        ))}

        <header
          className="titlebar"
          onMouseDown={beginWindowDrag}
          onDoubleClick={() => sendDesktopWindowCommand('maximize')}
        >
          <div className="mac-controls" aria-label="macOS window controls">
            <button
              className="tl-btn tl-close"
              type="button"
              aria-label="Close window"
              data-symbol="close"
              onClick={() => sendDesktopWindowCommand('close')}
            >
              <X data-testid="mac-close-symbol" aria-hidden />
            </button>
            <button
              className="tl-btn tl-min"
              type="button"
              aria-label="Minimize window"
              data-symbol="minimize"
              onClick={() => sendDesktopWindowCommand('minimize')}
            >
              <Minus data-testid="mac-minimize-symbol" aria-hidden />
            </button>
            <button
              className="tl-btn tl-max"
              type="button"
              aria-label="Maximize window"
              data-symbol="maximize"
              onClick={() => sendDesktopWindowCommand('maximize')}
            >
              <Maximize2 data-testid="mac-maximize-symbol" aria-hidden />
            </button>
          </div>

          {osStyle === 'windows' ? (
            <div className="win-title">
              {logoSrc ? <img src={logoSrc} alt="" /> : <Layers className="h-3.5 w-3.5" aria-hidden />}
              <span>{title}</span>
            </div>
          ) : null}
          {osStyle === 'mac' ? <div className="mac-title">{title}</div> : null}

          <div className="win-controls" aria-label="Windows window controls">
            <button className="win-btn" type="button" aria-label="Minimize window" onClick={() => sendDesktopWindowCommand('minimize')}>
              <Minus className="h-3.5 w-3.5" aria-hidden />
            </button>
            <button className="win-btn" type="button" aria-label="Maximize window" onClick={() => sendDesktopWindowCommand('maximize')}>
              <Square className="h-3 w-3" aria-hidden />
            </button>
            <button className="win-btn close" type="button" aria-label="Close window" onClick={() => sendDesktopWindowCommand('close')}>
              <X className="h-3.5 w-3.5" aria-hidden />
            </button>
          </div>
        </header>

        <div className={cn('desktop-window-content', contentClassName)}>{children}</div>
      </section>
    </main>
  );
}
