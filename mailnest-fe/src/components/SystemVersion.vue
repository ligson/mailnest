<template>
  <section class="system-version" aria-labelledby="system-version-title">
    <header class="version-header">
      <h3 id="system-version-title">系统版本</h3>
      <a-tooltip title="刷新后端版本信息">
        <a-button type="text" size="small" :loading="loading" aria-label="刷新后端版本信息" @click="loadVersion">
          <template #icon><reload-outlined /></template>
        </a-button>
      </a-tooltip>
    </header>
    <div class="version-grid">
      <div v-for="item in versions" :key="item.name" class="version-component">
        <div class="version-name">
          <span>{{ item.name }}</span>
          <strong v-if="item.info">v{{ item.info.version }}</strong>
          <span v-else>{{ loading ? '正在读取…' : '暂不可用' }}</span>
        </div>
        <dl v-if="item.info" class="version-details">
          <dt>提交号</dt>
          <dd :title="item.info.commit">{{ formatCommit(item.info.commit) }}</dd>
          <dt>构建时间</dt>
          <dd>{{ formatBuildTime(item.info.buildTime) }}</dd>
        </dl>
      </div>
    </div>
    <a-alert v-if="error" type="warning" show-icon :message="error" class="version-error">
      <template #action><a-button size="small" :loading="loading" aria-label="重试读取后端版本" @click="loadVersion">重试</a-button></template>
    </a-alert>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ReloadOutlined } from '@ant-design/icons-vue';
import { adminApi, type AppBuildInfo } from '../api/client';

// 前端取当前页面实际加载的构建信息，后端取运行中的服务，不能互相代替。
const frontend: AppBuildInfo = __APP_BUILD_INFO__;
const backend = ref<AppBuildInfo | null>(null);
const loading = ref(false);
const error = ref('');
const versions = computed(() => [
  { name: '前端', info: frontend },
  { name: '后端', info: backend.value },
]);

onMounted(loadVersion);

async function loadVersion() {
  if (loading.value) return;
  loading.value = true;
  error.value = '';
  try {
    backend.value = (await adminApi.version()).backend;
  } catch (cause) {
    backend.value = null;
    error.value = `后端版本读取失败：${cause instanceof Error ? cause.message : '请稍后重试'}`;
  } finally {
    loading.value = false;
  }
}

function formatCommit(value: string) {
  return !value || value === 'unknown' ? '未提供' : value.slice(0, 12);
}

function formatBuildTime(value: string) {
  if (!value || value === 'unknown') return '未提供';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '未提供' : date.toLocaleString('zh-CN', { hour12: false });
}
</script>

<style scoped>
.system-version {
  min-width: 0;
  padding: 12px 0;
  border-top: 1px solid var(--border-color);
  border-bottom: 1px solid var(--border-color);
}
.version-header {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.version-header h3 {
  margin: 0;
  font-size: 14px;
  color: var(--heading-color);
}
.version-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 24px;
}
.version-component { min-width: 0; }
.version-name { display: flex; align-items: baseline; flex-wrap: wrap; gap: 12px; }
.version-name strong { color: var(--heading-color); font-size: 16px; overflow-wrap: anywhere; }
.version-details {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 4px 12px;
  margin: 8px 0 0;
  font-size: 12px;
}
.version-details dt { color: var(--muted-color); }
.version-details dd { margin: 0; overflow-wrap: anywhere; }
.version-error { margin-top: 12px; }
@media (max-width: 640px) {
  .version-grid { grid-template-columns: minmax(0, 1fr); }
  .version-details { grid-template-columns: minmax(0, 1fr); gap: 2px; }
  .version-details dd { margin-bottom: 4px; }
}
</style>
