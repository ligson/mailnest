import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import ts from 'typescript';

const source = await readFile(new URL('../src/modules/mail/compose/window-transfer.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } });
const { openComposeTransfer, receiveComposeTransfer } = await import('data:text/javascript;base64,' + Buffer.from(outputText).toString('base64'));

function browserMock({ blocked = false, timeout = 30000 } = {}) {
  const popup = { opener: {}, location: { replace(url) { popup.url = url; } } };
  globalThis.window = {
    BroadcastChannel,
    setTimeout: (fn, ms) => setTimeout(fn, ms === 30000 ? timeout : ms),
    setInterval, clearTimeout, clearInterval,
    open: () => blocked ? null : popup,
  };
  return popup;
}
const sample = () => ({ draft: { id:'sample-draft', htmlBody:'<p>正文</p>', to:['to@example.com'] }, attachments:[new File(['附件内容'],'附件.txt')], forwardAttachments:[{id:'20',selected:true}], missingAttachmentNames:[] });

test('只有新窗口准备完成才交接，保留文件字节且隔离 opener', async () => {
  const popup = browserMock();
  let committed = false, prepared = 0;
  const sending = openComposeTransfer('user-1', async () => { prepared++; return sample(); }, () => { committed = true; });
  const key = new URL(popup.url, 'https://example.com').searchParams.get('handoff');
  assert.equal(popup.opener, null);
  assert.doesNotMatch(popup.url, /正文|to@|token|附件/);
  const received = await receiveComposeTransfer(key, 'user-1', new AbortController().signal);
  assert.equal(committed, false);
  assert.equal(prepared, 1);
  assert.equal(received.data.draft.htmlBody, '<p>正文</p>');
  assert.equal(await received.data.attachments[0].text(), '附件内容');
  assert.equal(received.data.forwardAttachments[0].id, '20');
  await received.activate(); await sending;
  assert.equal(committed, true);
});

test('弹窗被阻止时不保存、不释放原窗口', () => {
  browserMock({blocked:true});
  assert.throws(() => openComposeTransfer('user-1', () => { throw Error('不应保存'); }, () => { throw Error('不应交接'); }), /阻止/);
});

test('用户不匹配不能取得正文或附件，超时不释放原窗口', async () => {
  const popup = browserMock({timeout:50});
  let prepared = false, committed = false;
  const sending = openComposeTransfer('user-1', async () => { prepared=true; return sample(); }, () => { committed=true; });
  const sendingCheck = assert.rejects(sending, /未准备完成/);
  const key = new URL(popup.url, 'https://example.com').searchParams.get('handoff');
  await assert.rejects(receiveComposeTransfer(key, 'user-2', new AbortController().signal), /接续原窗口/);
  await sendingCheck;
  assert.equal(prepared, false); assert.equal(committed, false);
});

test('接收方取消后保留原窗口，迟到的准备结果不会向已关闭通道发送', async () => {
  const popup = browserMock({timeout:1000});
  let committed = false;
  const sending = openComposeTransfer('user-1', async () => { await new Promise(resolve=>setTimeout(resolve,100)); return sample(); }, () => { committed=true; });
  const sendingCheck = assert.rejects(sending, /初始化失败/);
  const abort = new AbortController();
  const key = new URL(popup.url, 'https://example.com').searchParams.get('handoff');
  const receivingCheck = assert.rejects(receiveComposeTransfer(key, 'user-1', abort.signal), /取消/);
  await new Promise(resolve=>setTimeout(resolve,20)); abort.abort();
  await receivingCheck; await sendingCheck;
  await new Promise(resolve=>setTimeout(resolve,120));
  assert.equal(committed, false);
});
