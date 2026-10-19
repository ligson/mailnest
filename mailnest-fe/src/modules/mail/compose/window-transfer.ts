import type { ComposeForwardAttachment, MailDraft } from '../../../api/client';

export interface ComposeTransfer {
  draft: MailDraft;
  attachments: File[];
  forwardAttachments: ComposeForwardAttachment[];
  missingAttachmentNames: string[];
}
const timeoutMs = 30000;

/** 浏览器内存传递，不在 URL、localStorage 或日志中写正文、附件或认证令牌。 */
export function openComposeTransfer(owner: string, prepare: () => Promise<ComposeTransfer>, commit: () => void) {
  if (!('BroadcastChannel' in window)) throw new Error('当前浏览器不支持转移写信，请继续在当前窗口编辑');
  const key = crypto.randomUUID();
  const channel = new BroadcastChannel('mailnest.compose.' + key);
  // 必须在点击事件内打开，随后立即隔离 opener，避免异步保存被弹窗策略拦截。
  const popup = window.open('about:blank', '_blank');
  if (!popup) { channel.close(); throw new Error('浏览器阻止了新窗口，请允许此站点打开弹窗'); }
  popup.opener = null;
  popup.location.replace('/mail/compose?handoff=' + encodeURIComponent(key));
  return new Promise<void>((resolve, reject) => {
    let payload: Promise<ComposeTransfer> | undefined;
    let payloadSent = false;
    let finished = false;
    const cleanup = () => { finished = true; clearTimeout(timer); channel.close(); };
    const fail = (error: unknown) => {
      if (finished) return;
      channel.postMessage({ type: 'error', owner });
      cleanup();
      reject(error instanceof Error ? error : new Error('转移写信失败，请在原窗口继续编辑'));
    };
    const timer = window.setTimeout(() => fail(new Error('新窗口未准备完成，邮件仍保留在原窗口，可重试')), timeoutMs);
    channel.onmessage = event => {
      if (finished || event.data?.owner !== owner) return;
      if (event.data.type === 'request') {
        payload ||= prepare();
        void payload.then(data => {
          if (!finished) { channel.postMessage({ type: 'payload', owner, data }); payloadSent = true; }
        }).catch(fail);
      } else if (event.data.type === 'ready' && payloadSent) {
        commit();
        channel.postMessage({ type: 'activate', owner });
        cleanup(); resolve();
      } else if (event.data.type === 'abort') {
        fail(new Error('新窗口初始化失败，邮件仍保留在原窗口'));
      }
    };
  });
}

export function receiveComposeTransfer(key: string, owner: string, signal: AbortSignal) {
  return new Promise<{ data: ComposeTransfer; activate: () => Promise<void>; cancel: () => void }>((resolve, reject) => {
    const channel = new BroadcastChannel('mailnest.compose.' + key);
    let received = false;
    let activated = false;
    let finished = false;
    let resolveActivation: (() => void) | undefined;
    let rejectActivation: ((error: Error) => void) | undefined;
    const cleanup = () => { finished = true; clearTimeout(timer); clearInterval(retry); channel.close(); signal.removeEventListener('abort', abort); };
    const fail = (reason: string) => {
      if (finished) return;
      channel.postMessage({ type: 'abort', owner });
      cleanup();
      const error = new Error(reason);
      rejectActivation?.(error); reject(error);
    };
    const abort = () => fail('写信转移已取消');
    const timer = window.setTimeout(() => fail('无法接续原窗口，请返回原窗口继续编辑或从草稿恢复'), timeoutMs);
    const request = () => { if (!received) channel.postMessage({ type: 'request', owner }); };
    const retry = window.setInterval(request, 500);
    channel.onmessage = event => {
      if (finished || event.data?.owner !== owner) return;
      if (event.data.type === 'payload' && !received) {
        received = true; clearInterval(retry);
        resolve({ data: event.data.data, cancel: abort, activate: () => new Promise<void>((done, failed) => {
          if (finished && !activated) { failed(new Error('转移已结束，请返回原窗口继续编辑')); return; }
          resolveActivation = done; rejectActivation = failed;
          channel.postMessage({ type: 'ready', owner });
        }) });
      } else if (event.data.type === 'activate') {
        activated = true; cleanup(); resolveActivation?.();
      } else if (event.data.type === 'error') fail('原窗口未完成转移，请回到原窗口继续编辑');
    };
    signal.addEventListener('abort', abort, { once: true });
    if (signal.aborted) abort(); else request();
  });
}
