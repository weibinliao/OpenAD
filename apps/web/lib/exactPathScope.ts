function normalizePath(input: string) {
  return input.trim().replace(/\//g, '\\');
}

export function normalizePathForExactMatch(input: string) {
  const normalized = normalizePath(input);
  if (!normalized) {
    return '';
  }

  const withoutTrailingSlashes = normalized.replace(/\\+$/g, '');
  if (!withoutTrailingSlashes) {
    return '';
  }

  if (/^[A-Za-z]:$/.test(withoutTrailingSlashes)) {
    return `${withoutTrailingSlashes}\\`.toLowerCase();
  }

  return withoutTrailingSlashes.toLowerCase();
}

export function hasExactPathMatch(candidatePath: string, selectedPath: string) {
  const selectedKey = normalizePathForExactMatch(selectedPath);
  if (!selectedKey) {
    return false;
  }

  return normalizePathForExactMatch(candidatePath) === selectedKey;
}

export function filterRowsForExactSelectedPath<T extends { path: string }>(items: readonly T[], selectedPath: string) {
  return items.filter((item) => hasExactPathMatch(item.path, selectedPath));
}
