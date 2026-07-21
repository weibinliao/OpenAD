export function apiBase() {
  const configured = (process.env.NEXT_PUBLIC_API_BASE_URL || '').trim();
  if (configured) {
    return configured.replace(/\/$/, '');
  }

  if (typeof window !== 'undefined') {
    const protocol = window.location.protocol === 'https:' ? 'https:' : 'http:';
    const hostname = window.location.hostname || 'localhost';
    return `${protocol}//${hostname}:18080`;
  }

  return 'http://localhost:18080';
}

export function websocketBase() {
  return apiBase().replace(/^http/i, 'ws');
}
