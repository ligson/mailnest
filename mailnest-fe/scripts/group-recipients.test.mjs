import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import ts from 'typescript';

const source = await readFile(new URL('../src/modules/mail/compose/groupRecipients.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } });
const { mergeGroupRecipients } = await import('data:text/javascript;base64,' + Buffer.from(outputText).toString('base64'));
const a = { id: '1', email: 'alice@example.com', name: 'Alice', nickname: '阿丽', displayName: 'Alice' };
const b = { id: '2', email: 'bob@example.com', name: 'Bob', nickname: '', displayName: 'Bob' };

test('组重叠及重复选组按邮箱去重，优先昵称并保留已有顺序', () => {
  const original = ['已有 <ALICE@example.com>', 'manual@example.com'];
  const result = mergeGroupRecipients(original, [a, b, a]);
  assert.deepEqual(original, ['已有 <ALICE@example.com>', 'manual@example.com']);
  assert.deepEqual(result, [...original, '"Bob" <bob@example.com>']);
  assert.deepEqual(mergeGroupRecipients([], [a]), ['"阿丽" <alice@example.com>']);
  assert.deepEqual(mergeGroupRecipients(result, [a, b]), result);
});

test('特殊显示名正确转义，空组不修改原地址，不跨栏改动密送', () => {
  const original = ['private@example.com'];
  assert.deepEqual(mergeGroupRecipients(original, []), original);
  const cc = ['cc@example.com'];
  const result = mergeGroupRecipients(original, [{ ...b, nickname: 'Bob "销售", 西区' }]);
  assert.equal(result[1], '"Bob \\"销售\\", 西区" <bob@example.com>');
  assert.deepEqual(cc, ['cc@example.com']);
  assert.deepEqual(original, ['private@example.com']);
});
