import { Node } from '@tiptap/core';
import DOMPurify from 'dompurify';

export function sanitizeSignature(html: string) {
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['style', 'form', 'input', 'button', 'textarea', 'select', 'iframe', 'object', 'embed'],
    FORBID_ATTR: ['id', 'name', 'contenteditable'],
  });
}

export function signatureText(html: string) {
  const element = document.createElement('div');
  element.innerHTML = sanitizeSignature(html);
  element.querySelectorAll('br').forEach(br => br.replaceWith('\n'));
  element.querySelectorAll('p, div, tr').forEach(block => block.append('\n'));
  return element.textContent || '';
}

// 签名作为完整模板保存，编辑时与表格/段落的默认 CSS 隔离，发送时仍输出普通 HTML。
export const MailSignature = Node.create({
  name: 'mailSignature', group: 'block', atom: true, selectable: true, draggable: true,
  addAttributes() {
    return { html: { default: '', rendered: false } };
  },
  parseHTML() {
    return [{ tag: 'section[data-mailnest-signature]', getAttrs: element => ({ html: sanitizeSignature((element as HTMLElement).innerHTML) }) }];
  },
  renderHTML({ node }) {
    const section = document.createElement('section');
    section.dataset.mailnestSignature = 'true';
    section.innerHTML = sanitizeSignature(node.attrs.html);
    return section;
  },
  renderText({ node }) { return signatureText(node.attrs.html); },
  addNodeView() {
    return ({ node }) => {
      const dom = document.createElement('div');
      dom.className = 'compose-signature';
      dom.contentEditable = 'false';
      dom.setAttribute('aria-label', '邮件签名（保留模板排版，可在邮箱账号中修改）');
      const frame = document.createElement('iframe');
      frame.title = '邮件签名预览';
      frame.setAttribute('sandbox', 'allow-same-origin');
      frame.setAttribute('referrerpolicy', 'no-referrer');
      frame.style.cssText = 'display:block;width:100%;border:0;pointer-events:none;min-height:24px;';
      const html = sanitizeSignature(node.attrs.html);
      // 双层保护：清理活动内容，并且 iframe 禁止脚本、表单、跳转及外部样式。
      frame.srcdoc = `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src https: http: data:; style-src 'unsafe-inline';"><style>html,body{margin:0;padding:0;background:#fff}body{font:14px/1.5 Arial,'Microsoft YaHei',sans-serif;color:#222}img{max-width:100%;height:auto}table{border-collapse:collapse}</style></head><body>${html}</body></html>`;
      dom.append(frame);
      let observer: ResizeObserver | undefined;
      let containerObserver: ResizeObserver | undefined;
      let animation = 0;
      let lastWidth = 0;
      const resize = () => {
        cancelAnimationFrame(animation);
        animation = requestAnimationFrame(() => {
          const body = frame.contentDocument?.body;
          if (!body) return;
          const width = dom.clientWidth;
          if (width !== lastWidth) {
            lastWidth = width;
            frame.style.width = '100%';
            const naturalWidth = Math.max(width, body.scrollWidth, frame.contentDocument!.documentElement.scrollWidth);
            frame.style.width = `${naturalWidth}px`;
          }
          frame.style.height = '0px';
          frame.style.height = `${Math.max(body.scrollHeight, frame.contentDocument!.documentElement.scrollHeight, 24)}px`;
        });
      };
      frame.onload = () => {
        const body = frame.contentDocument?.body;
        if (!body) return;
        observer = new ResizeObserver(resize);
        observer.observe(body);
        containerObserver = new ResizeObserver(resize);
        containerObserver.observe(dom);
        for (const image of body.querySelectorAll('img')) { image.onload = resize; image.onerror = resize; }
        resize();
      };
      return {
        dom,
        ignoreMutation: () => true,
        selectNode: () => dom.classList.add('ProseMirror-selectednode'),
        deselectNode: () => dom.classList.remove('ProseMirror-selectednode'),
        destroy: () => { frame.onload = null; observer?.disconnect(); containerObserver?.disconnect(); cancelAnimationFrame(animation); },
      };
    };
  },
});
