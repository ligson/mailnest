<template>
  <div class="standalone-reader-layout">
    <header class="standalone-reader-toolbar">
      <router-link class="reader-brand" to="/mail"><MailOutlined /> <strong>Mail Nest</strong><span>邮件阅读</span></router-link>
      <a-space>
        <a-tooltip title="刷新邮件"><a-button aria-label="刷新邮件" :loading="loading" @click="$emit('refresh')"><ReloadOutlined /></a-button></a-tooltip>
        <a-button :href="mailboxURL"><template #icon><ArrowLeftOutlined /></template>返回邮箱</a-button>
      </a-space>
    </header>
    <main class="standalone-reader-content"><slot /></main>
  </div>
</template>

<script setup lang="ts">
import { ArrowLeftOutlined, MailOutlined, ReloadOutlined } from '@ant-design/icons-vue';
defineProps<{ mailboxURL: string; loading: boolean }>();
defineEmits<{ refresh: [] }>();
</script>

<style scoped>
.standalone-reader-layout { display: grid; grid-template-rows: auto minmax(0, 1fr); height: 100dvh; background: var(--app-bg); color: var(--text-color); }
.standalone-reader-toolbar { display: flex; min-height: 60px; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 24px; border-bottom: 1px solid var(--border-color); background: var(--surface-bg); }
.reader-brand { display: flex; min-width: 0; align-items: center; gap: 10px; color: var(--heading-color); white-space: nowrap; text-decoration: none; }
.reader-brand > .anticon { color: var(--accent); font-size: 22px; }
.reader-brand span:last-child { padding-left: 12px; border-left: 1px solid var(--border-color); color: var(--muted-color); font-size: 13px; }
.standalone-reader-content { min-width: 0; min-height: 0; padding: 16px; }
@media (max-width: 640px) { .standalone-reader-toolbar { padding: 10px 12px; } .reader-brand span:last-child { display: none; } .standalone-reader-content { padding: 8px; } }
</style>
