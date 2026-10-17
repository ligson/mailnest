import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import ts from 'typescript';

const source = await readFile(new URL('../src/modules/mail/paging.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } });
const { MailCursorHistory } = await import('data:text/javascript;base64,' + Buffer.from(outputText).toString('base64'));

test('同筛选顺序翻页保留游标，筛选/用户代次切换和返回首页清空', () => {
  const history = new MailCursorHistory();
  assert.equal(history.select('user1/inbox/epoch1', 1), undefined);
  history.remember('user1/inbox/epoch1', 1, 'cursor2');
  assert.equal(history.select('user1/inbox/epoch1', 2), 'cursor2');
  assert.equal(history.select('user1/inbox/epoch1', 9), undefined);
  assert.equal(history.select('user1/sent/epoch1', 2), undefined);
  history.remember('user1/inbox/epoch1', 1, 'stale-response');
  assert.equal(history.select('user1/sent/epoch1', 2), undefined);
  history.remember('user1/sent/epoch1', 1, 'new');
  assert.equal(history.select('user2/sent/epoch1', 2), undefined);
  history.remember('user2/sent/epoch1', 1, 'second');
  history.select('user2/sent/epoch1', 1);
  assert.equal(history.select('user2/sent/epoch1', 2), undefined);
});

test('游标历史有界，分叉翻页删除旧后续页', () => {
  const history = new MailCursorHistory();
  history.select('scope', 1);
  for (let page = 1; page <= 140; page++) history.remember('scope', page, 'cursor-' + (page + 1));
  assert.equal(history.select('scope', 2), undefined);
  assert.equal(history.select('scope', 141), 'cursor-141');
  history.remember('scope', 20, 'changed');
  assert.equal(history.select('scope', 21), 'changed');
  assert.equal(history.select('scope', 141), undefined);
  history.remember('scope', 20);
  assert.equal(history.select('scope', 21), undefined);
});
