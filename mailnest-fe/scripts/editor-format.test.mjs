import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import ts from 'typescript';

const source = await readFile(new URL('../src/modules/mail/compose/editor-format.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } });
const { normalizeMailLink, normalizeImageWidth, readSelectionFormat } = await import('data:text/javascript;base64,' + Buffer.from(outputText).toString('base64'));

test('兼容历史图片数字宽度，拒绝向尺寸注入其他CSS', () => {
  assert.equal(normalizeImageWidth('120'), '120px');
  assert.equal(normalizeImageWidth('50%'), '50%');
  assert.equal(normalizeImageWidth('120.5px'), '120.5px');
  for (const value of [null, '', '50%;background:red', 'expression(alert(1))', '-1px', 'auto']) assert.equal(normalizeImageWidth(value), null);
});

test('链接只接受安全协议，拒绝脚本、控制字符、相对地址和URL凭据', () => {
  assert.equal(normalizeMailLink(' https://example.com/路径 '), 'https://example.com/%E8%B7%AF%E5%BE%84');
  assert.equal(normalizeMailLink('mailto:a@example.com'), 'mailto:a@example.com');
  assert.equal(normalizeMailLink('tel:+8613800000000'), 'tel:+8613800000000');
  assert.equal(normalizeMailLink('http://example.com'), 'http://example.com/');
  for (const value of ['javascript:alert(1)', 'data:text/html,<script>', '//example.com', '/relative', 'https://user:secret@example.com', 'https://exa\nmple.com', 'mailto:', 'tel:', '']) assert.equal(normalizeMailLink(value), null, value);
});

function mockEditor(attrs, empty = false, current = {}) {
  return {
    state: { selection: {from:1,to:9,empty}, doc: {nodesBetween: (_from,_to,visit) => attrs.forEach(a=>visit({isText:true,marks:a?[{type:{name:'textStyle'},attrs:a}]:[]}))} },
    getAttributes: () => current,
  };
}

test('工具栏准确识别光标输入格式、统一格式和混合选区', () => {
  assert.equal(readSelectionFormat(mockEditor([], true, {fontSize:'24px'})).fontSize, '24px');
  const mixed = readSelectionFormat(mockEditor([{fontSize:'24px',fontFamily:'Arial'}, {fontSize:'18px',fontFamily:'Arial'}]));
  assert.equal(mixed.fontSize, undefined);
  assert.equal(mixed.fontFamily, 'Arial');
  assert.equal(mixed.color, '#1f2937');
  assert.equal(readSelectionFormat(mockEditor([null,{fontSize:'14px'}])).fontSize, '14px');
  assert.equal(readSelectionFormat(mockEditor([{backgroundColor:'#fde68a'},null])).backgroundColor, undefined);
});
