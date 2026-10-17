import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, readFile, rm, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { gunzipSync } from 'node:zlib';
import { precompress } from './precompress.mjs';

test('预压缩保留原文件，递归处理公开资源且可重复执行', async () => {
  const root = await mkdtemp(join(tmpdir(), 'mailnest-assets-'));
  try {
    await mkdir(join(root, 'assets'));
    const source = 'const hello = "信匣";\n'.repeat(200);
    const file = join(root, 'assets', 'main-hash.js');
    await writeFile(file, source);
    await writeFile(join(root, 'small.css'), 'body{}');
    await writeFile(join(root, 'private.bin'), source);
    assert.equal(await precompress(root), 1);
    assert.equal(await readFile(file, 'utf8'), source);
    assert.equal(gunzipSync(await readFile(file + '.gz')).toString(), source);
    await assert.rejects(access(join(root, 'small.css.gz')));
    await assert.rejects(access(join(root, 'private.bin.gz')));
    assert.equal(await precompress(root), 1);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
