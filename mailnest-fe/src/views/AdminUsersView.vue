<template>
  <AppLayout selected-key="/admin/users">
    <section class="content-panel admin-users-page">
      <div class="page-toolbar">
        <div>
          <h2 class="page-title">系统管理</h2>
          <p class="page-subtitle">查看用户规模、账号状态和邮件附件存储占用</p>
        </div>
        <a-button @click="loadUsers">刷新</a-button>
      </div>

      <div class="admin-metrics">
        <div class="admin-metric">
          <span>用户总数</span>
          <strong>{{ users.length }}</strong>
        </div>
        <div class="admin-metric">
          <span>启用用户</span>
          <strong>{{ enabledCount }}</strong>
        </div>
        <div class="admin-metric">
          <span>邮件总数</span>
          <strong>{{ totalMessages }}</strong>
        </div>
        <div class="admin-metric">
          <span>附件占用</span>
          <strong>{{ formatSize(totalAttachmentBytes) }}</strong>
        </div>
      </div>

      <a-table
        row-key="id"
        :columns="columns"
        :data-source="users"
        :loading="loading"
        :pagination="false"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'user'">
            <div class="admin-user-cell">
              <strong>{{ record.nickname || record.username }}</strong>
              <span>{{ record.email }}</span>
            </div>
          </template>
          <template v-else-if="column.key === 'role'">
            <a-tag :color="record.isAdmin ? 'blue' : 'default'">
              {{ record.isAdmin ? '管理员' : '普通用户' }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'enabled'">
            <a-switch
              :checked="record.enabled"
              :disabled="record.id === auth.user?.id"
              :loading="togglingId === record.id"
              checked-children="启用"
              un-checked-children="停用"
              @change="onEnabledChange(record, $event)"
            />
          </template>
          <template v-else-if="column.key === 'usage'">
            <div class="admin-usage-cell">
              <strong>{{ formatSize(record.attachmentBytes) }}</strong>
              <span>{{ record.attachmentCount }} 个附件</span>
            </div>
          </template>
          <template v-else-if="column.key === 'counts'">
            <div class="admin-count-tags">
              <a-tag>邮箱 {{ record.mailAccountCount }}</a-tag>
              <a-tag>邮件 {{ record.messageCount }}</a-tag>
              <a-tag>联系人 {{ record.contactCount }}</a-tag>
              <a-tag>规则 {{ record.ruleCount }}</a-tag>
            </div>
          </template>
          <template v-else-if="column.key === 'lastActive'">
            <span>{{ formatTime(record.lastMessageAt || record.lastSyncAt) }}</span>
          </template>
        </template>
      </a-table>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { Modal, message } from 'ant-design-vue';
import type { TableColumnsType } from 'ant-design-vue';
import { adminApi, type AdminUserSummary } from '../api/client';
import AppLayout from '../components/AppLayout.vue';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
const loading = ref(false);
const togglingId = ref('');
const users = ref<AdminUserSummary[]>([]);

const columns: TableColumnsType<AdminUserSummary> = [
  { title: '用户', key: 'user', width: 260 },
  { title: '角色', key: 'role', width: 100 },
  { title: '状态', key: 'enabled', width: 110 },
  { title: '存储', key: 'usage', width: 130 },
  { title: '数据概览', key: 'counts' },
  { title: '最近邮件/同步', key: 'lastActive', width: 170 },
];

const enabledCount = computed(() => users.value.filter((item) => item.enabled).length);
const totalMessages = computed(() => users.value.reduce((sum, item) => sum + item.messageCount, 0));
const totalAttachmentBytes = computed(() => users.value.reduce((sum, item) => sum + item.attachmentBytes, 0));

onMounted(loadUsers);

async function loadUsers() {
  loading.value = true;
  try {
    users.value = (await adminApi.users()).items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取用户列表失败');
  } finally {
    loading.value = false;
  }
}

function updateEnabled(record: AdminUserSummary, enabled: boolean) {
  if (record.enabled === enabled || record.id === auth.user?.id) {
    return;
  }
  const actionText = enabled ? '启用' : '停用';
  Modal.confirm({
    title: `${actionText}用户`,
    content: enabled
      ? `确定启用用户「${record.username}」？`
      : `停用后该用户将无法登录，已有登录态也会失效。确定停用「${record.username}」？`,
    okText: actionText,
    cancelText: '取消',
    okButtonProps: { danger: !enabled },
    onOk: () => doUpdateEnabled(record.id, enabled),
  });
}

function onEnabledChange(record: AdminUserSummary, checked: boolean | string | number) {
  updateEnabled(record, Boolean(checked));
}

async function doUpdateEnabled(id: string, enabled: boolean) {
  togglingId.value = id;
  try {
    const updated = await adminApi.updateUserEnabled(id, enabled);
    users.value = users.value.map((item) => (
      item.id === id
        ? { ...item, enabled: updated.enabled, updatedAt: updated.updatedAt }
        : item
    ));
    message.success(enabled ? '用户已启用' : '用户已停用');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '更新用户状态失败');
  } finally {
    togglingId.value = '';
  }
}

function formatSize(value: number) {
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  if (value < 1024 * 1024 * 1024) {
    return `${(value / 1024 / 1024).toFixed(1)} MB`;
  }
  return `${(value / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

function formatTime(value: string | null) {
  return value ? new Date(value).toLocaleString() : '暂无';
}
</script>

<style scoped>
.admin-users-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.admin-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.admin-metric {
  display: grid;
  gap: 6px;
  padding: 14px 16px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--surface-muted);
}

.admin-metric span,
.admin-user-cell span,
.admin-usage-cell span {
  color: var(--muted-color);
  font-size: 12px;
}

.admin-metric strong {
  color: var(--heading-color);
  font-size: 22px;
}

.admin-user-cell,
.admin-usage-cell {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.admin-user-cell strong,
.admin-user-cell span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-count-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

@media (max-width: 900px) {
  .admin-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
