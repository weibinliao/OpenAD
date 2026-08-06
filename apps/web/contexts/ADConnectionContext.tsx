import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { apiBase } from '../lib/runtimeApi';

export interface ADConnectionConfig {
  adServer: string;
  baseDN: string;
  username: string;
  password: string;
}

export interface ADConnectionSnapshot {
  connected: boolean;
  message: string;
  testedAt: string | null;
}

/** Stored connection profile returned by GET /api/ad/connections (password never included). */
export interface ConnectionProfile {
  id: string;
  name: string;
  server: string;
  base_dn: string;
  bind_user: string;
  is_default: boolean;
  last_tested_at?: string | null;
  last_test_ok?: boolean | null;
  created_at: string;
  updated_at: string;
}

interface ADConnectionState {
  config: ADConnectionConfig;
  connection: ADConnectionSnapshot;
}

interface SaveConnectionPayload {
  config: ADConnectionConfig;
  connected: boolean;
  message?: string;
  testedAt?: string | null;
}

interface ADConnectionContextValue extends ADConnectionState {
  saveConnection: (payload: SaveConnectionPayload) => void;
  clearConnection: () => void;
  /** Stored server-side connection profiles. Empty when the backend is offline. */
  profiles: ConnectionProfile[];
  profilesLoading: boolean;
  /** True when the last profile fetch failed (backend offline or endpoints unavailable). */
  profilesOffline: boolean;
  refreshProfiles: () => Promise<void>;
  activeProfileId: string | null;
  setActiveProfileId: (id: string | null) => void;
  activeProfile: ConnectionProfile | null;
}

const STORAGE_KEY = 'fsa.adConnection';
const ACTIVE_PROFILE_KEY = 'fsa.adActiveProfile';

const defaultState: ADConnectionState = {
  config: {
    adServer: '',
    baseDN: '',
    username: '',
    password: '',
  },
  connection: {
    connected: false,
    message: '',
    testedAt: null,
  },
};

const ADConnectionContext = createContext<ADConnectionContextValue | undefined>(undefined);

function readStoredState(): ADConnectionState {
  if (typeof window === 'undefined') {
    return defaultState;
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return defaultState;
    }

    const parsed = JSON.parse(raw) as Partial<ADConnectionState> | null;
    return {
      config: {
        adServer: parsed?.config?.adServer ?? '',
        baseDN: parsed?.config?.baseDN ?? '',
        username: parsed?.config?.username ?? '',
        password: '',
      },
      connection: {
        connected: Boolean(parsed?.connection?.connected),
        message: parsed?.connection?.message ?? '',
        testedAt: parsed?.connection?.testedAt ?? null,
      },
    };
  } catch {
    return defaultState;
  }
}

function readStoredActiveProfileId(): string | null {
  if (typeof window === 'undefined') {
    return null;
  }
  try {
    return window.localStorage.getItem(ACTIVE_PROFILE_KEY) || null;
  } catch {
    return null;
  }
}

export function ADConnectionProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<ADConnectionState>(defaultState);
  const [hydrated, setHydrated] = useState(false);
  const [profiles, setProfiles] = useState<ConnectionProfile[]>([]);
  const [profilesLoading, setProfilesLoading] = useState(false);
  const [profilesLoaded, setProfilesLoaded] = useState(false);
  const [profilesOffline, setProfilesOffline] = useState(false);
  const [activeProfileId, setActiveProfileIdState] = useState<string | null>(null);

  useEffect(() => {
    setState(readStoredState());
    setActiveProfileIdState(readStoredActiveProfileId());
    setHydrated(true);
  }, []);

  useEffect(() => {
    if (!hydrated || typeof window === 'undefined') {
      return;
    }

    window.localStorage.setItem(STORAGE_KEY, JSON.stringify({
      ...state,
      config: {
        ...state.config,
        password: '',
      },
    }));
  }, [hydrated, state]);

  const saveConnection = useCallback((payload: SaveConnectionPayload) => {
    setState({
      config: payload.config,
      connection: {
        connected: payload.connected,
        message: payload.message ?? '',
        testedAt: payload.testedAt ?? null,
      },
    });
  }, []);

  const clearConnection = useCallback(() => {
    setState(defaultState);
    if (typeof window !== 'undefined') {
      window.localStorage.removeItem(STORAGE_KEY);
    }
  }, []);

  const refreshProfiles = useCallback(async () => {
    setProfilesLoading(true);
    try {
      const response = await fetch(`${apiBase()}/api/ad/connections`);
      const data = (await response.json().catch(() => ({}))) as { items?: ConnectionProfile[]; error?: string };
      if (!response.ok) {
        throw new Error(data.error || 'connections unavailable');
      }
      setProfiles(Array.isArray(data.items) ? data.items : []);
      setProfilesLoaded(true);
      setProfilesOffline(false);
    } catch {
      // Backend offline or old build without connection endpoints: degrade to empty list.
      setProfiles([]);
      setProfilesOffline(true);
    } finally {
      setProfilesLoading(false);
    }
  }, []);

  // Load stored profiles once on mount.
  useEffect(() => {
    void refreshProfiles();
  }, [refreshProfiles]);

  const setActiveProfileId = useCallback((id: string | null) => {
    setActiveProfileIdState(id);
    if (typeof window === 'undefined') {
      return;
    }
    try {
      if (id) {
        window.localStorage.setItem(ACTIVE_PROFILE_KEY, id);
      } else {
        window.localStorage.removeItem(ACTIVE_PROFILE_KEY);
      }
    } catch {
      // Ignore persistence errors.
    }
  }, []);

  // Explicit selection wins; otherwise fall back to the default profile, or
  // the only stored profile. A configured connection should just work without
  // an extra "use" click.
  const activeProfile = useMemo(() => {
    const explicit = profiles.find((profile) => profile.id === activeProfileId);
    if (explicit) return explicit;
    const fallback = profiles.find((profile) => profile.is_default) || (profiles.length === 1 ? profiles[0] : null);
    return fallback || null;
  }, [activeProfileId, profiles]);

  // Keep the legacy connection snapshot aligned with the server-side profile list.
  // This also clears a stale "connected" badge when the final profile is deleted.
  useEffect(() => {
    if (!profilesLoaded || profilesLoading || profilesOffline) {
      return;
    }
    if (!activeProfile) {
      setState(defaultState);
      return;
    }
    if (!activeProfile.last_test_ok) return;

    setState((prev) => ({
      config: prev.config,
      connection: {
        connected: true,
        message: `${activeProfile.name} (${activeProfile.server})`,
        testedAt: activeProfile.last_tested_at ?? prev.connection.testedAt ?? null,
      },
    }));
  }, [activeProfile, profilesLoaded, profilesLoading, profilesOffline]);

  const value = useMemo<ADConnectionContextValue>(
    () => ({
      ...state,
      saveConnection,
      clearConnection,
      profiles,
      profilesLoading,
      profilesOffline,
      refreshProfiles,
      activeProfileId,
      setActiveProfileId,
      activeProfile,
    }),
    [
      activeProfile,
      activeProfileId,
      clearConnection,
      profiles,
      profilesLoading,
      profilesOffline,
      refreshProfiles,
      saveConnection,
      setActiveProfileId,
      state,
    ]
  );

  return <ADConnectionContext.Provider value={value}>{children}</ADConnectionContext.Provider>;
}

export function useADConnection() {
  const context = useContext(ADConnectionContext);
  if (!context) {
    throw new Error('useADConnection must be used within ADConnectionProvider');
  }

  return context;
}
