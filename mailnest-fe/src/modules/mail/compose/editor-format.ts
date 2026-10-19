import type { Editor } from '@tiptap/core';

export function normalizeImageWidth(value: unknown): string | null {
  const raw = String(value ?? '').trim();
  if (/^\d+(?:\.\d+)?$/.test(raw)) return raw + 'px';
  return /^\d+(?:\.\d+)?(?:px|%)$/.test(raw) ? raw : null;
}

export function normalizeMailLink(value: string): string | null {
  const raw = value.trim();
  if (!raw || /[\u0000-\u0020\u007f]/.test(raw)) return null;
  try {
    const url = new URL(raw);
    if (!['https:', 'http:', 'mailto:', 'tel:'].includes(url.protocol)) return null;
    if ((url.protocol === 'https:' || url.protocol === 'http:') && (!url.hostname || url.username || url.password)) return null;
    if ((url.protocol === 'mailto:' || url.protocol === 'tel:') && !url.pathname) return null;
    return url.href;
  } catch { return null; }
}

// 混合样式不谎报为第一段的格式；光标状态则读取当前输入标记。
export function readSelectionFormat(editor: Editor) {
  const defaults: Record<string, string> = { fontFamily: 'system-ui', fontSize: '14px', color: '#1f2937', backgroundColor: '' };
  const { from, to, empty } = editor.state.selection;
  const attrs: Record<string, unknown>[] = [];
  if (empty) attrs.push(editor.getAttributes('textStyle'));
  else editor.state.doc.nodesBetween(from, to, node => {
    if (node.isText) attrs.push(node.marks.find(mark => mark.type.name === 'textStyle')?.attrs || {});
  });
  if (!attrs.length) attrs.push(editor.getAttributes('textStyle'));
  return Object.fromEntries(Object.entries(defaults).map(([key, fallback]) => {
    const values = new Set(attrs.map(attr => String(attr[key] || fallback)));
    return [key, values.size === 1 ? [...values][0] : undefined];
  }));
}
