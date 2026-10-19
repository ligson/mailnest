<template>
  <div class="compose-window-layout">
    <header class="compose-window-header"><div><MailOutlined /><strong>{{ title || '写邮件' }}</strong></div><router-link to="/mail"><ArrowLeftOutlined /> 返回邮箱</router-link></header>
    <main><slot /></main>
  </div>
</template>
<script setup lang="ts">
import { ArrowLeftOutlined, MailOutlined } from '@ant-design/icons-vue';
defineProps<{ title?: string }>();
</script>
<style scoped>
.compose-window-layout { display: grid; grid-template-rows: auto minmax(0, 1fr); height: 100dvh; background: var(--app-bg); color: var(--text-color); }
.compose-window-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 56px; padding: 10px 20px; border-bottom: 1px solid var(--border-color); background: var(--surface-bg); }
.compose-window-header > div { display: flex; align-items: center; gap: 10px; }
.compose-window-header .anticon { color: var(--accent); }
.compose-window-header a { white-space: nowrap; }
main { min-width: 0; min-height: 0; padding: 12px; }
@media (max-width: 640px) { main { padding: 6px; } .compose-window-header { padding: 10px 12px; } }
</style>
