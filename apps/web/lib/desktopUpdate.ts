export const OPENAD_RELEASE_PAGE = 'https://github.com/weibinliao/OpenAD/releases/latest';

export type DesktopUpdateAction = 'status' | 'check' | 'install';

export interface DesktopUpdateMessage {
  type?: string;
  action?: string;
  ok?: boolean;
  currentVersion?: string;
  latestVersion?: string;
  releaseName?: string;
  releaseNotes?: string;
  publishedAt?: string | null;
  releaseUrl?: string;
  updateAvailable?: boolean;
  launched?: boolean;
  error?: string;
}

interface DesktopWebView {
  postMessage: (message: unknown) => void;
  addEventListener?: (type: 'message', listener: (event: MessageEvent<unknown>) => void) => void;
  removeEventListener?: (type: 'message', listener: (event: MessageEvent<unknown>) => void) => void;
}

function getDesktopWebView(): DesktopWebView | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const browserWindow = window as unknown as { chrome?: { webview?: DesktopWebView } };
  return browserWindow.chrome?.webview || null;
}

export function hasDesktopUpdateBridge() {
  const webview = getDesktopWebView();
  return Boolean(webview?.postMessage && webview?.addEventListener);
}

export function sendDesktopUpdateRequest(action: DesktopUpdateAction) {
  const webview = getDesktopWebView();
  if (!webview?.postMessage) {
    return false;
  }

  webview.postMessage({ type: 'openad-update', action });
  return true;
}

export function subscribeToDesktopUpdates(listener: (message: DesktopUpdateMessage) => void) {
  const webview = getDesktopWebView();
  if (!webview?.addEventListener) {
    return () => undefined;
  }

  const handleMessage = (event: MessageEvent<unknown>) => {
    if (!event.data || typeof event.data !== 'object') {
      return;
    }

    const message = event.data as DesktopUpdateMessage;
    if (message.type === 'openad-update') {
      listener(message);
    }
  };
  webview.addEventListener('message', handleMessage);

  return () => {
    webview.removeEventListener?.('message', handleMessage);
  };
}

export function openOpenADReleasePage() {
  if (typeof window === 'undefined') {
    return;
  }

  window.open(OPENAD_RELEASE_PAGE, '_blank', 'noopener,noreferrer');
}
