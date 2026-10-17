import { readdir, readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { gzipSync, constants } from 'node:zlib';
import { pathToFileURL } from 'node:url';

// 在构建机预压缩公开资源，NAS 只读取压缩副本，不消耗实时压缩 CPU。
export async function precompress(root) {
  let count = 0;
  for (const entry of await readdir(root, { withFileTypes: true })) {
    const path = join(root, entry.name);
    if (entry.isDirectory()) {
      count += await precompress(path);
    } else if (/\.(?:js|css|svg|json|html)$/.test(entry.name)) {
      const data = await readFile(path);
      if (data.length < 1024) continue;
      const compressed = gzipSync(data, { level: constants.Z_BEST_COMPRESSION });
      if (compressed.length >= data.length) continue;
      await writeFile(path + '.gz', compressed);
      count++;
    }
  }
  return count;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  console.log('静态资源预压缩完成：' + await precompress('dist') + ' 个文件');
}
