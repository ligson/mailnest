import { Extension, mergeAttributes } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { TextStyleKit } from '@tiptap/extension-text-style';
import TextAlign from '@tiptap/extension-text-align';
import Image from '@tiptap/extension-image';
import { TableKit } from '@tiptap/extension-table';
import { normalizeImageWidth, normalizeMailLink } from './editor-format';
import { MailSignature } from './signature-extension';

// 发出的 HTML 自带基础表格样式，不依赖本系统的 CSS 才能在其他邮件客户端显示。
const tableBorder = 'border:1px solid #d9d9d9;padding:6px 10px;vertical-align:top;';
const MailImage = Image.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      width: {
        default: null,
        parseHTML: element => normalizeImageWidth(element.style.width || element.getAttribute('width')),
        renderHTML: attributes => normalizeImageWidth(attributes.width) ? { style: `width:${normalizeImageWidth(attributes.width)};height:auto;max-width:100%;` } : {},
      },
    };
  },
  renderHTML({ HTMLAttributes }) {
    return ['img', mergeAttributes(this.options.HTMLAttributes, HTMLAttributes, { style: 'max-width:100%;height:auto;' })];
  },
});

const ParagraphLayout = Extension.create({
  name: 'mailParagraphLayout',
  addGlobalAttributes() {
    return [{ types: ['paragraph', 'heading'], attributes: {
      lineHeight: {
        default: null,
        parseHTML: element => element.style.lineHeight || null,
        renderHTML: attributes => attributes.lineHeight ? { style: `line-height:${attributes.lineHeight}` } : {},
      },
    } }];
  },
});

export function createMailEditorExtensions() {
  return [
    StarterKit.configure({
      heading: { levels: [1, 2, 3] },
      link: { openOnClick: false, autolink: true, defaultProtocol: 'https', isAllowedUri: url => normalizeMailLink(url) !== null, HTMLAttributes: { rel: 'noopener noreferrer', target: '_blank' } },
      blockquote: { HTMLAttributes: { style: 'border-left:3px solid #d9d9d9;margin:12px 0;padding-left:12px;color:#5f6b7a;' } },
    }),
    TextStyleKit.configure({ lineHeight: false }),
    MailSignature,
    ParagraphLayout,
    TextAlign.configure({ types: ['heading', 'paragraph'] }),
    MailImage.configure({ inline: true, allowBase64: true }),
    TableKit.configure({
      table: { resizable: true, HTMLAttributes: { style: 'border-collapse:collapse;table-layout:fixed;width:100%;' } },
      tableCell: { HTMLAttributes: { style: tableBorder } },
      tableHeader: { HTMLAttributes: { style: tableBorder + 'background-color:#f5f5f5;font-weight:bold;' } },
    }),
  ];
}
