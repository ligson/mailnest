import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import ts from 'typescript';

const source = await readFile(new URL('../src/modules/mail/reader/body-document.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } });
const { buildMailBodySrcdoc } = await import('data:text/javascript;base64,' + Buffer.from(outputText).toString('base64'));

test('正文保留邮件表格列宽、图片和引用，不再全局压缩及任意断字', () => {
  const html = '<table width="1400"><tr><td width="240">YonDiF 产品盘</td></tr></table><img src="/inline-image"><blockquote>原邮件</blockquote>';
  const document = buildMailBodySrcdoc(html);
  assert.ok(document.includes(html));
  assert.match(document, /<meta charset="utf-8"/);
  assert.match(document, /<base target="_blank"/);
  assert.doesNotMatch(document, /max-width:\s*100%\s*!important|overflow-x:\s*hidden|overflow-wrap:\s*anywhere|word-break:\s*break-word/);
  assert.match(document, /td, th \{ overflow-wrap: normal; word-break: normal;/);
});
