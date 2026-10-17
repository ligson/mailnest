<template>
  <a-popover v-model:open="open" trigger="click" placement="bottomRight" title="邮件检索">
    <template #content>
      <div class="search-status-panel" aria-live="polite">
        <p class="status-heading">{{ heading }}</p>
        <template v-if="status?.engine === 'bleve'">
          <a-progress v-if="status.phase === 'building'" :percent="percent" size="small" />
          <dl class="status-metrics">
            <dt>当前用户重建进度</dt><dd>{{ status.done.toLocaleString() }} / {{ status.total.toLocaleString() }}</dd>
            <dt>待处理任务</dt><dd>{{ status.pending.toLocaleString() }}</dd>
            <dt>重试中</dt><dd>{{ status.failed.toLocaleString() }}</dd>
            <template v-if="status.shadowChecks !== undefined">
              <dt>影子检查（全局抽样）</dt><dd>{{ status.shadowChecks }}</dd>
              <dt>候选遗漏（全局抽样）</dt><dd>{{ status.shadowMissing }}</dd>
            </template>
          </dl>
          <a-button v-if="auth.user?.isAdmin" size="small" aria-label="重建索引" :loading="rebuilding" :disabled="status.phase === 'building'" @click="confirmRebuild">
            <template #icon><reload-outlined /></template>重建索引
          </a-button>
        </template>
        <a-alert v-if="error || status?.message" class="status-alert" type="warning" show-icon :message="error || status?.message" />
        <p v-if="feedback" class="status-feedback">{{ feedback }}</p>
      </div>
    </template>
    <a-tooltip title="检索状态">
      <a-button type="text" size="small" class="search-status-button" aria-label="检索状态">
        <template #icon>
          <warning-outlined v-if="error || status?.message" />
          <sync-outlined v-else-if="status?.phase === 'building'" spin />
          <database-outlined v-else />
        </template>
      </a-button>
    </a-tooltip>
  </a-popover>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { Modal } from 'ant-design-vue';
import { DatabaseOutlined, ReloadOutlined, SyncOutlined, WarningOutlined } from '@ant-design/icons-vue';
import { isCanceledRequest, searchApi, type MailSearchStatus as SearchStatus } from '../api/client';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
const open = ref(false);
const status = ref<SearchStatus>();
const error = ref('');
const feedback = ref('');
const rebuilding = ref(false);
let timer: ReturnType<typeof setTimeout> | undefined;
let controller: AbortController | undefined;
let disposed = false;
let confirming = false;
let confirmation: ReturnType<typeof Modal.confirm> | undefined;
const heading = computed(() => {
  if (!status.value) return error.value ? '检索状态暂不可用' : '正在读取检索状态';
  if (status.value.engine === 'database') return '数据库搜索';
  const mode = status.value.mode === 'shadow' ? '影子验证' : '索引加速';
  if (status.value.phase === 'building') return `${mode} · 正在后台重建${status.value.available ? '，旧索引继续服务' : '，暂用数据库搜索'}`;
  return `${mode} · 索引就绪`;
});
const percent = computed(() => status.value?.total ? Math.min(100, Math.floor(status.value.done * 100 / status.value.total)) : 0);

async function refresh() {
  clearTimeout(timer);
  controller?.abort();
  const request = new AbortController();
  controller = request;
  try {
    const value = await searchApi.status(request.signal);
    if (controller !== request || disposed) return;
    status.value = value;
    error.value = '';
  } catch (err) {
    if (controller === request && !disposed && !isCanceledRequest(err)) error.value = err instanceof Error ? err.message : '读取检索状态失败';
  } finally {
    if (controller === request && !disposed && open.value) {
      const busy = status.value?.phase === 'building' || Boolean(status.value?.pending);
      timer = setTimeout(() => { if (document.visibilityState === 'visible') void refresh(); else scheduleVisibleRefresh(); }, busy ? 5000 : 30000);
    }
  }
}
function scheduleVisibleRefresh() {
  if (!disposed && open.value) timer = setTimeout(() => { if (document.visibilityState === 'visible') void refresh(); else scheduleVisibleRefresh(); }, 5000);
}
function confirmRebuild() {
  if (confirming || rebuilding.value) return;
  confirming = true;
  confirmation = Modal.confirm({
    title: '重建邮件搜索索引？',
    content: '仅重建检索副本，不修改邮件和附件。重建期间继续使用旧索引或数据库搜索。',
    okText: '重建', cancelText: '取消',
    afterClose() {
      confirming = false;
      if (!disposed) open.value = true;
    },
    async onOk() {
      rebuilding.value = true; feedback.value = '';
      try { await searchApi.rebuild(); feedback.value = '已安排后台重建'; await refresh(); }
      catch (err) { error.value = err instanceof Error ? err.message : '重建请求失败'; }
      finally { rebuilding.value = false; }
    },
  });
}
watch(open, value => {
  if (value) void refresh();
  else { clearTimeout(timer); controller?.abort(); }
});
function closeOnResize() { open.value = false; }
onMounted(() => { window.addEventListener('resize', closeOnResize); void refresh(); });
onBeforeUnmount(() => { disposed = true; window.removeEventListener('resize', closeOnResize); clearTimeout(timer); controller?.abort(); confirmation?.destroy(); });
</script>

<style scoped>
.search-status-button { flex: 0 0 28px; width: 28px; height: 28px; color: var(--muted-color, #666); }
.search-status-panel { width: min(290px, calc(100vw - 64px)); font-size: 13px; }
.status-heading { margin: 0 0 10px; line-height: 1.6; }
.status-metrics { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px 12px; margin: 12px 0; }
.status-metrics dt { color: var(--muted-color, #666); }
.status-metrics dd { margin: 0; font-variant-numeric: tabular-nums; }
.status-alert { margin-top: 12px; }
.status-feedback { margin: 10px 0 0; color: #389e0d; }
</style>
