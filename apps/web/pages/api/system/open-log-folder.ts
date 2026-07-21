import type { NextApiRequest, NextApiResponse } from 'next';
import fs from 'fs';
import path from 'path';
import { spawn } from 'child_process';

function resolveLogsPath() {
  return path.resolve(process.cwd(), '..', '..', '.local', 'logs');
}

function openFolder(target: string) {
  if (process.platform === 'win32') {
    spawn('explorer.exe', [target], { detached: true, stdio: 'ignore' }).unref();
    return;
  }
  if (process.platform === 'darwin') {
    spawn('open', [target], { detached: true, stdio: 'ignore' }).unref();
    return;
  }
  spawn('xdg-open', [target], { detached: true, stdio: 'ignore' }).unref();
}

export default function handler(request: NextApiRequest, response: NextApiResponse) {
  if (request.method !== 'POST') {
    response.setHeader('Allow', 'POST');
    response.status(405).json({ error: 'Method Not Allowed' });
    return;
  }

  const logsPath = resolveLogsPath();
  fs.mkdirSync(logsPath, { recursive: true });

  try {
    openFolder(logsPath);
    response.status(200).json({ ok: true, path: logsPath });
  } catch (error) {
    response.status(500).json({
      error: error instanceof Error ? error.message : 'Failed to open log folder',
      path: logsPath,
    });
  }
}
