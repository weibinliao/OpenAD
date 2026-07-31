import { apiBase } from './runtimeApi';

export interface DirectoryUser {
  dn: string;
  username: string;
  display_name: string;
  email?: string;
  first_name?: string;
  last_name?: string;
  department?: string;
  division?: string;
  domain?: string;
  group_dns?: string[];
  groups?: string[];
}

export interface DirectoryGroup {
  dn: string;
  name?: string;
}

export type DirectoryMatch =
  | { kind: 'user'; dn: string; label: string; secondary: string; user: DirectoryUser }
  | { kind: 'group'; dn: string; label: string; secondary: string; group: DirectoryGroup };

interface DirectorySearchOptions {
  connectionId: string;
  query: string;
  limit?: number;
  signal?: AbortSignal;
}

export function groupNameFromDN(distinguishedName: string) {
  const firstPart = distinguishedName.split(',')[0]?.trim() || distinguishedName;
  return firstPart.replace(/^(CN|OU)=/i, '') || distinguishedName;
}

export function directoryUserDirectGroupDNs(user: DirectoryUser) {
  const directGroupDNs = user.group_dns;
  const candidates = Array.isArray(directGroupDNs) ? directGroupDNs : user.groups || [];
  return candidates.filter((value) => /^(CN|OU)=/i.test(value.trim()));
}

export function directoryGroupName(group: DirectoryGroup) {
  return group.name || groupNameFromDN(group.dn);
}

export function directoryMatches(
  users: DirectoryUser[],
  groups: DirectoryGroup[],
): DirectoryMatch[] {
  return [
    ...users.map((user): DirectoryMatch => ({
      kind: 'user',
      dn: user.dn,
      label: user.display_name || user.username || groupNameFromDN(user.dn),
      secondary: [user.username, user.email].filter(Boolean).join(' · '),
      user,
    })),
    ...groups.map((group): DirectoryMatch => ({
      kind: 'group',
      dn: group.dn,
      label: directoryGroupName(group),
      secondary: group.dn,
      group,
    })),
  ];
}

export async function searchDirectoryObjects({
  connectionId,
  query,
  limit = 8,
  signal,
}: DirectorySearchOptions) {
  const trimmed = query.trim();
  if (trimmed.length < 2) {
    return { users: [] as DirectoryUser[], groups: [] as DirectoryGroup[], matches: [] as DirectoryMatch[] };
  }

  const request = (path: string) => fetch(`${apiBase()}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ connection_id: connectionId, query: trimmed, limit }),
    signal,
  });
  const [userResponse, groupResponse] = await Promise.all([
    request('/api/ad/users/query'),
    request('/api/ad/groups/query'),
  ]);
  const [userData, groupData] = await Promise.all([
    userResponse.json().catch(() => ({})),
    groupResponse.json().catch(() => ({})),
  ]);

  if (!userResponse.ok) {
    throw new Error(userData.error || 'User search failed.');
  }
  if (!groupResponse.ok) {
    throw new Error(groupData.error || 'Group search failed.');
  }

  const users = Array.isArray(userData.users) ? userData.users as DirectoryUser[] : [];
  const groups = Array.isArray(groupData.groups) ? groupData.groups as DirectoryGroup[] : [];
  return { users, groups, matches: directoryMatches(users, groups) };
}
