import { act, renderHook } from '@testing-library/react';
import { checkRuntimeHealth, useRuntimeHealth } from './useRuntimeHealth';

function response(body: unknown, ok = true) {
  return {
    ok,
    json: async () => body,
  } as Response;
}

describe('runtime health', () => {
  afterEach(() => {
    jest.useRealTimers();
    jest.restoreAllMocks();
  });

  it('accepts only the OpenAD health contract', async () => {
    const healthyFetch = jest.fn().mockResolvedValue(response({
      service: 'openad',
      status: 'healthy',
      database: true,
    })) as unknown as typeof fetch;
    const genericFetch = jest.fn().mockResolvedValue(response({ status: 'healthy' })) as unknown as typeof fetch;

    await expect(checkRuntimeHealth(healthyFetch)).resolves.toBe('healthy');
    await expect(checkRuntimeHealth(genericFetch)).resolves.toBe('offline');
  });

  it('checks on mount, every 30 seconds, and when the window regains focus', async () => {
    jest.useFakeTimers();
    const fetcher = jest.fn().mockResolvedValue(response({
      service: 'openad',
      status: 'healthy',
      database: true,
    })) as unknown as typeof fetch;

    const { result } = renderHook(() => useRuntimeHealth({ fetcher }));

    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current).toBe('healthy');
    expect(fetcher).toHaveBeenCalledTimes(1);

    await act(async () => {
      await jest.advanceTimersByTimeAsync(30_000);
    });
    expect(fetcher).toHaveBeenCalledTimes(2);

    await act(async () => {
      window.dispatchEvent(new Event('focus'));
      await Promise.resolve();
    });
    expect(fetcher).toHaveBeenCalledTimes(3);
  });
});
