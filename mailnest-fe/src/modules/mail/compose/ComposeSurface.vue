<template>
  <section v-if="standalone" class="standalone-compose-surface">
    <slot v-if="open" />
    <a-skeleton v-else-if="busy" active :paragraph="{ rows: 8 }" class="compose-window-skeleton" />
    <a-result v-else status="info" title="当前邮件已关闭">
      <template #extra><a-button type="primary" @click="$emit('new')">写新邮件</a-button></template>
    </a-result>
  </section>
  <a-modal v-else :open="open" :width="1040" :style="{ top: '16px', paddingBottom: 0 }" :destroy-on-close="false" :mask-closable="false" :footer="null" class="compose-modal" @cancel="$emit('cancel')">
    <template #title>
      <div class="compose-window-title"><span>{{ title }}</span>
        <a-tooltip title="转到新窗口继续写信（保留内容和附件）"><a-button type="text" size="small" aria-label="新窗口写邮件" :disabled="busy" @click="$emit('popout')"><template #icon><ExportOutlined /></template></a-button></a-tooltip>
      </div>
    </template>
    <slot />
  </a-modal>
</template>
<script setup lang="ts">
import { ExportOutlined } from '@ant-design/icons-vue';
defineProps<{ open: boolean; title: string; standalone: boolean; busy: boolean }>();
defineEmits<{ cancel: []; popout: []; new: [] }>();
</script>
<style scoped>
.compose-window-title { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding-right: 28px; }
.standalone-compose-surface { display: flex; flex-direction: column; height: 100%; min-height: 0; min-width: 0; background: var(--surface-bg); border: 1px solid var(--border-color); border-radius: 8px; overflow: hidden; }
.compose-window-skeleton { padding: 24px; }
</style>
