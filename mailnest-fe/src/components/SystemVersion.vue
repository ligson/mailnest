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
          <a-button v-if="item.info" type="link" class="version-link" @click="openRelease(item.info.version)">v{{ item.info.version }}</a-button>
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
    <a-modal v-model:open="releaseOpen" :title="releaseTitle" :footer="null" width="680px">
      <div v-if="release" class="release-detail">
        <div class="release-meta"><a-tag color="blue">{{ release.version }}</a-tag><span>{{ release.date }}</span></div>
        <p class="release-summary">{{ release.summary }}</p>
        <a-divider />
        <ul><li v-for="item in release.items" :key="item">{{ item }}</li></ul>
      </div>
    </a-modal>
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
const releaseOpen = ref(false);
const selectedRelease = ref('');
const releaseNotes = [
  { version: '0.6.0', date: '2026-11-27', summary: '再次发送、版本记录、附件加载和联系人分组优化', items: ['邮件详情支持再发一封，保留原收件人、抄送、密送、正文和普通附件', '系统管理版本号可点击查看发布内容', '附件中心首屏跳过大表总数统计，联系人分组成员改为可搜索表格', '邮件操作按钮统一间距，手机端同步支持再次发送'] },
  { version: '0.5.0', date: '2026-10-19', summary: '独立写信与完整签名排版', items: ['写邮件、回复和转发支持转到新窗口继续编辑', '收件人、抄送、密送、正文和附件可在窗口间接续', '签名保留原有颜色、字号、边框、间距和布局表格', '草稿保存串行化，避免迟到请求覆盖最新内容'] },
  { version: '0.4.0', date: '2026-10-19', summary: '联系人批量设置、富文本编辑器和独立邮件阅读', items: ['联系人支持批量设置公司和分组', '写信编辑器支持局部格式、表格、图片和草稿恢复', '邮件详情支持独立阅读窗口和宽表格滚动'] },
  { version: '0.3.0', date: '2026-10-17', summary: '联系人分组与收件人分组选择', items: ['联系人支持多对多分组管理', '写信时可以按分组添加收件人、抄送和密送'] },
];
const release = computed(() => releaseNotes.find(item => item.version === selectedRelease.value));
const releaseTitle = computed(() => release.value ? `${release.value.version} 发布内容` : '版本发布内容');
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

function openRelease(version: string) {
  selectedRelease.value = version;
  releaseOpen.value = true;
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
.version-link { height: auto; padding: 0; color: var(--heading-color); font-size: 16px; font-weight: 700; }
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
.release-meta { display: flex; align-items: center; gap: 10px; color: var(--muted-color); }
.release-summary { margin: 16px 0 0; color: var(--heading-color); font-size: 15px; font-weight: 600; }
.release-detail ul { display: grid; gap: 10px; margin: 0; padding-left: 20px; color: var(--text-color); line-height: 1.65; }
@media (max-width: 640px) {
  .version-grid { grid-template-columns: minmax(0, 1fr); }
  .version-details { grid-template-columns: minmax(0, 1fr); gap: 2px; }
  .version-details dd { margin-bottom: 4px; }
}
</style>
