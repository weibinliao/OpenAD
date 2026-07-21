import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const projectRoot = path.resolve(scriptDirectory, '..');
const nextDirectory = path.join(projectRoot, process.env.NEXT_STATIC_EXPORT === '1' ? '.next-static' : '.next');
const outputDirectory = path.join(projectRoot, 'out');

rmSync(outputDirectory, { recursive: true, force: true });
mkdirSync(outputDirectory, { recursive: true });

if (process.env.NEXT_STATIC_EXPORT === '1') {
  const requiredPages = ['index.html', '404.html'];

  for (const page of requiredPages) {
    const pagePath = path.join(nextDirectory, page);
    if (!existsSync(pagePath)) {
      throw new Error(`Missing exported page: ${pagePath}`);
    }
  }

  cpSync(nextDirectory, outputDirectory, { recursive: true });
} else {
  const pagesDirectory = path.join(nextDirectory, 'server', 'pages');
  const staticDirectory = path.join(nextDirectory, 'static');
  const requiredPages = ['index.html', '404.html'];
  const optionalPages = ['500.html'];

  for (const page of requiredPages) {
    const pagePath = path.join(pagesDirectory, page);
    if (!existsSync(pagePath)) {
      throw new Error(`Missing exported page: ${pagePath}`);
    }
  }

  for (const page of [...requiredPages, ...optionalPages]) {
    const sourcePath = path.join(pagesDirectory, page);
    if (!existsSync(sourcePath)) {
      continue;
    }

    cpSync(sourcePath, path.join(outputDirectory, page));
  }

  const nextStaticOutputDirectory = path.join(outputDirectory, '_next', 'static');
  mkdirSync(path.dirname(nextStaticOutputDirectory), { recursive: true });
  cpSync(staticDirectory, nextStaticOutputDirectory, { recursive: true });
}
