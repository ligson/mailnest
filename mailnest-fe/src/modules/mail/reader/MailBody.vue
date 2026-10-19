<template>
  <div v-if="html" ref="container" class="mail-body-scroll" tabindex="0" aria-label="邮件正文，可横向滚动查看宽表格">
    <iframe
      ref="frame"
      class="mail-body-frame"
      :srcdoc="buildMailBodySrcdoc(html)"
      sandbox="allow-popups allow-popups-to-escape-sandbox allow-same-origin"
      title="邮件正文"
      @load="onLoad"
    ></iframe>
  </div>
  <pre v-else class="mail-text-body">{{ text || '没有正文内容' }}</pre>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue';
import { buildMailBodySrcdoc } from './body-document';

defineProps<{ html?: string | null; text?: string | null }>();
const container = ref<HTMLDivElement>();
const frame = ref<HTMLIFrameElement>();
let containerObserver: ResizeObserver | undefined;
let bodyObserver: ResizeObserver | undefined;
let cleanupImages: (() => void) | undefined;
let animation = 0;
let lastWidth = 0;
let generation = 0;

function updateHeight() {
  const element = frame.value;
  const doc = element?.contentDocument;
  if (!element || !doc?.body) return;
  // 先归零再测量，窗口变宽、内容换行减少时高度才能同步缩小。
  element.style.height = '0px';
  element.style.height = `${Math.max(doc.body.scrollHeight, doc.documentElement.scrollHeight, 180)}px`;
}

function layout() {
  cancelAnimationFrame(animation);
  animation = requestAnimationFrame(() => {
    const element = frame.value;
    const doc = element?.contentDocument;
    if (!element || !container.value || !doc?.body) return;
    element.style.width = '100%';
    const available = container.value.clientWidth;
    const natural = Math.max(doc.body.scrollWidth, doc.documentElement.scrollWidth);
    if (natural > available) element.style.width = `${natural}px`;
    updateHeight();
  });
}

function clearObservers() {
  containerObserver?.disconnect();
  bodyObserver?.disconnect();
  cleanupImages?.();
}

function onLoad() {
  clearObservers();
  const current = ++generation;
  const doc = frame.value?.contentDocument;
  if (!doc?.body || !container.value) return;
  lastWidth = container.value.clientWidth;
  containerObserver = new ResizeObserver(() => {
    const width = container.value?.clientWidth || 0;
    if (width !== lastWidth) { lastWidth = width; layout(); }
  });
  containerObserver.observe(container.value);
  bodyObserver = new ResizeObserver(updateHeight);
  bodyObserver.observe(doc.body);
  const images = Array.from(doc.images);
  for (const image of images) { image.addEventListener('load', layout); image.addEventListener('error', layout); }
  cleanupImages = () => {
    for (const image of images) { image.removeEventListener('load', layout); image.removeEventListener('error', layout); }
  };
  void doc.fonts?.ready.then(() => { if (current === generation) layout(); });
  layout();
}

onBeforeUnmount(() => { generation++; clearObservers(); cancelAnimationFrame(animation); });
</script>

<style scoped>
.mail-body-scroll { width: 100%; min-width: 0; overflow-x: auto; overflow-y: hidden; background: #fff; }
.mail-body-frame { display: block; width: 100%; min-height: 180px; border: 0; background: #fff; }
.mail-text-body { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; font-family: inherit; line-height: 1.7; }
</style>
