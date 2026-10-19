<template>
  <div class="compose-rich-editor">
    <ComposeToolbar v-if="editor" :editor="editor" @files="emit('files')" @images="emit('images')" @signature="emit('signature')" />
    <EditorContent :editor="editor" class="compose-editor-content" />
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted } from 'vue';
import { EditorContent, useEditor } from '@tiptap/vue-3';
import { NodeSelection } from '@tiptap/pm/state';
import { createMailEditorExtensions } from './editor-extensions';
import ComposeToolbar from './ComposeToolbar.vue';
import type { ComposeEditorBridge } from './editor-types';

const props = defineProps<{ initialHtml: string }>();
const emit = defineEmits<{ change: []; ready: []; files: []; images: []; signature: []; pasteImages: [files: File[]] }>();
const editor = useEditor({
  extensions: createMailEditorExtensions(),
  content: props.initialHtml,
  editorProps: {
    attributes: { class: 'compose-editor-body', role: 'textbox', 'aria-label': '邮件正文', 'aria-multiline': 'true' },
    handleClickOn: (view, _pos, node, nodePos, _event, direct) => {
      if (!direct || node.type.name !== 'image') return false;
      view.dispatch(view.state.tr.setSelection(NodeSelection.create(view.state.doc, nodePos)));
      return true;
    },
    handlePaste: (_view, event) => {
      const files = Array.from(event.clipboardData?.files || []).filter(file => file.type.startsWith('image/'));
      if (!files.length) return false;
      event.preventDefault(); emit('pasteImages', files); return true;
    },
    handleDrop: (_view, event, _slice, moved) => {
      const files = Array.from(event.dataTransfer?.files || []).filter(file => file.type.startsWith('image/'));
      if (moved || !files.length) return false;
      event.preventDefault(); emit('pasteImages', files); return true;
    },
  },
  onUpdate: () => emit('change'),
});

defineExpose<ComposeEditorBridge>({
  getHTML: () => editor.value?.isEmpty ? '' : editor.value?.getHTML() || '',
  getText: () => editor.value?.getText({ blockSeparator: '\n' }) || '',
  setContent: html => { editor.value?.chain().setContent(html || '', { emitUpdate: false }).setMeta('addToHistory', false).run(); },
  insertHTML: html => { editor.value?.chain().focus().insertContent(html).run(); },
});
onMounted(async () => { await nextTick(); emit('ready'); });
</script>

<style scoped>
.compose-rich-editor { display: flex; flex-direction: column; min-height: 0; height: 100%; }
.compose-editor-content { min-height: 0; flex: 1; overflow: auto; }
:deep(.compose-editor-body) { min-height: 100%; padding: 14px 16px; color: var(--text-color); background: var(--surface-bg); font-size: 14px; line-height: 1.7; outline: none; overflow-wrap: anywhere; }
:deep(.compose-editor-body > :first-child) { margin-top: 0; }
:deep(.compose-editor-body p) { margin: 0 0 8px; }
:deep(.compose-editor-body > p:only-child:has(> br.ProseMirror-trailingBreak)::before) { content: '输入邮件正文'; color: var(--muted-weak); float: left; height: 0; pointer-events: none; }
:deep(.compose-editor-body img) { max-width: 100%; height: auto; }
:deep(.compose-editor-body .ProseMirror-selectednode) { outline: 2px solid var(--accent); }
:deep(.compose-editor-body a) { color: #1677ff; text-decoration: underline; cursor: text; }
:deep(.compose-editor-body table) { border-collapse: collapse; table-layout: fixed; width: 100%; margin: 12px 0; }
:deep(.compose-editor-body th), :deep(.compose-editor-body td) { min-width: 40px; border: 1px solid #d9d9d9; padding: 6px 10px; position: relative; vertical-align: top; }
:deep(.compose-editor-body .tableWrapper) { overflow-x: auto; }
:deep(.compose-editor-body .selectedCell::after) { content: ''; position: absolute; inset: 0; pointer-events: none; background: #1677ff20; }
:deep(.compose-editor-body .column-resize-handle) { position: absolute; right: -2px; top: 0; bottom: -2px; width: 4px; background: var(--accent); pointer-events: none; }
:deep(.compose-editor-body.resize-cursor) { cursor: col-resize; }
:deep(.compose-editor-body pre) { padding: 12px; background: var(--surface-muted); white-space: pre-wrap; }
:deep(.compose-signature) { display: block; max-width: 100%; overflow-x: auto; margin: 8px 0; background: #fff; cursor: default; }
</style>
