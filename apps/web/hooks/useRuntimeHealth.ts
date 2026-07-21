import { useEffect, useState } from 'react';
import { apiBase } from '../lib/runtimeApi';

export type RuntimeHealthState = 'checking' | 'healthy' | 'offline';

interface RuntimeHealthOptions {
  fetcher?: typeof fetch;
  intervalMs?: number;
  timeoutMs?: number;
}

export async function checkRuntimeHealth(
  fetcher: typeof fetch = fetch,
  timeoutMs = 3000,
): Promise<Exclude<RuntimeHealthState, 'checking'>> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await fetcher(`${apiBase()}/health`, { signal: controller.signal });
    if (!response.ok) {
      return 'offline';
    }

    const data = await response.json() as {
      service?: string;
      status?: string;
      database?: boolean;
    };

    return data.service === 'openad'
      && data.status === 'healthy'
      && data.database === true
      ? 'healthy'
      : 'offline';
  } catch {
    return 'offline';
  } finally {
    window.clearTimeout(timeout);
  }
}

export function useRuntimeHealth({
  fetcher = fetch,
  intervalMs = 30_000,
  timeoutMs = 3000,
}: RuntimeHealthOptions = {}): RuntimeHealthState {
  const [state, setState] = useState<RuntimeHealthState>('checking');

  useEffect(() => {
    let active = true;

    const refresh = async () => {
      const next = await checkRuntimeHealth(fetcher, timeoutMs);
      if (active) {
        setState(next);
      }
    };

    void refresh();
    const interval = window.setInterval(() => void refresh(), intervalMs);
    const handleFocus = () => void refresh();
    window.addEventListener('focus', handleFocus);

    return () => {
      active = false;
      window.clearInterval(interval);
      window.removeEventListener('focus', handleFocus);
    };
  }, [fetcher, intervalMs, timeoutMs]);

  return state;
}
