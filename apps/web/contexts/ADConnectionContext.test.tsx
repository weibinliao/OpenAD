import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import { ADConnectionProvider, useADConnection } from './ADConnectionContext';

function ConnectionProbe() {
  const { activeProfile, connection, profilesLoading } = useADConnection();

  return (
    <div>
      <span data-testid="connection-state">{connection.connected ? 'connected' : 'disconnected'}</span>
      <span data-testid="active-profile">{activeProfile?.id || 'none'}</span>
      <span data-testid="profiles-loading">{profilesLoading ? 'loading' : 'loaded'}</span>
    </div>
  );
}

describe('ADConnectionProvider', () => {
  beforeEach(() => {
    window.localStorage.clear();
    window.localStorage.setItem(
      'fsa.adConnection',
      JSON.stringify({
        config: {
          adServer: 'dc01.example.com',
          baseDN: 'DC=example,DC=com',
          username: 'EXAMPLE\\alice',
          password: '',
        },
        connection: {
          connected: true,
          message: 'Example DC',
          testedAt: '2026-08-01T00:00:00Z',
        },
      })
    );
    window.localStorage.setItem('fsa.adActiveProfile', 'deleted-profile');
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  test('clears stale connected state after the server confirms no profiles remain', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ items: [] }),
    } as Response);

    render(
      <ADConnectionProvider>
        <ConnectionProbe />
      </ADConnectionProvider>
    );

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());
    await waitFor(() => {
      expect(screen.getByTestId('profiles-loading')).toHaveTextContent('loaded');
      expect(screen.getByTestId('active-profile')).toHaveTextContent('none');
      expect(screen.getByTestId('connection-state')).toHaveTextContent('disconnected');
    });
  });
});
