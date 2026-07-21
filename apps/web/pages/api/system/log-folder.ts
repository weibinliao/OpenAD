import type { NextApiRequest, NextApiResponse } from 'next';
import fs from 'fs';
import path from 'path';

export default function handler(_request: NextApiRequest, response: NextApiResponse) {
  const logsPath = path.resolve(process.cwd(), '..', '..', '.local', 'logs');
  fs.mkdirSync(logsPath, { recursive: true });
  response.status(200).json({ path: logsPath, exists: true });
}
