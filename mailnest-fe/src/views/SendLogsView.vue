<template>
  <AppLayout selected-key="/send-logs">
    <section class="content-panel send-logs-page">
      <div class="page-toolbar">
        <div>
          <h2 class="page-title">发送记录</h2>
          <p class="page-subtitle">查看 SMTP 投递结果、失败原因和重试状态</p>
        </div>
        <a-button @click="loadSendLogs">刷新</a-button>
      </div>

      <div class="send-log-filters">
        <a-input-search
          v-model:value="filters.keyword"
          allow-clear
          placeholder="搜索主题、收件人、Message-ID 或失败原因"
          @search="onFilterChanged"
          @change="onFilterChanged"
        />
        <a-select v-model:value="filters.accountId" allow-clear placeholder="发件账号" @change="onFilterChanged">
          <a-select-option v-for="account in accounts" :key="account.id" :value="account.id">
            {{ account.displayName || account.email }}
          </a-select-option>
        </a-select>
        <a-select v-model:value="filters.status" allow-clear placeholder="发送状态" @change="onFilterChanged">
          <a-select-option value="success">成功</a-select-option>
          <a-select-option value="failed">失败</a-select-option>
          <a-select-option value="local_save_failed">已发出但本地保存失败</a-select-option>
          <a-select-option value="sending">发送中</a-select-option>
        </a-select>
        <a-select v-model:value="filters.retryStatus" allow-clear placeholder="重试状态" @change="onFilterChanged">
          <a-select-option value="none">无需重试</a-select-option>
          <a-select-option value="retryable">可重试</a-select-option>
          <a-select-option value="retrying">重试中</a-select-option>
          <a-select-option value="exhausted">已耗尽</a-select-option>
        </a-select>
        <a-range-picker v-model:value="dateRange" @change="onDateChanged" />
      </div>

      <a-table
        row-key="id"
        :columns="columns"
        :data-source="sendLogs"
        :loading="loading"
        :pagination="false"
        size="middle"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'subject'">
            <div class="send-log-subject-cell">
              <strong>{{ record.subject || '无主题' }}</strong>
              <small>{{ recipientsText(record) }}</small>
            </div>
          </template>
          <template v-if="column.key === 'account'">
            {{ accountName(record.accountId, record.accountEmail) }}
          </template>
          <template v-if="column.key === 'status'">
            <div class="send-log-status-cell">
              <a-tag :color="statusColor(record.status)">{{ statusLabel(record.status) }}</a-tag>
              <span>{{ retryStatusLabel(record.retryStatus) }}</span>
            </div>
          </template>
          <template v-if="column.key === 'finishedAt'">
            {{ formatTime(record.finishedAt || record.startedAt || record.createdAt) }}
          </template>
          <template v-if="column.key === 'error'">
            <span class="send-log-error">{{ record.errorMessage || '-' }}</span>
          </template>
          <template v-if="column.key === 'actions'">
            <a-space>
              <a-button type="link" size="small" @click="openDetail(record)">详情</a-button>
              <a-button v-if="record.messageId" type="link" size="small" @click="openMessage(record.messageId)">查看邮件</a-button>
            </a-space>
          </template>
        </template>
      </a-table>

      <a-pagination
        v-if="total > pageSize"
        v-model:current="page"
        :page-size="pageSize"
        :total="total"
        class="send-log-pagination"
        @change="loadSendLogs"
      />

      <a-drawer v-model:open="detailOpen" width="640" title="发送记录详情">
        <a-descriptions v-if="selectedLog" bordered size="small" :column="1">
          <a-descriptions-item label="主题">{{ selectedLog.subject || '无主题' }}</a-descriptions-item>
          <a-descriptions-item label="发件账号">{{ accountName(selectedLog.accountId, selectedLog.accountEmail) }}</a-descriptions-item>
          <a-descriptions-item label="收件人">{{ recipientsText(selectedLog) }}</a-descriptions-item>
          <a-descriptions-item label="发送状态">
            <a-tag :color="statusColor(selectedLog.status)">{{ statusLabel(selectedLog.status) }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="重试状态">{{ retryStatusLabel(selectedLog.retryStatus) }}</a-descriptions-item>
          <a-descriptions-item label="SMTP Message-ID">{{ selectedLog.smtpMessageId || '-' }}</a-descriptions-item>
          <a-descriptions-item label="附件数量">{{ selectedLog.attachmentCount }}</a-descriptions-item>
          <a-descriptions-item label="开始时间">{{ formatTime(selectedLog.startedAt || selectedLog.createdAt) }}</a-descriptions-item>
          <a-descriptions-item label="结束时间">{{ formatTime(selectedLog.finishedAt) }}</a-descriptions-item>
          <a-descriptions-item label="失败原因">{{ selectedLog.errorMessage || '-' }}</a-descriptions-item>
        </a-descriptions>
      </a-drawer>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import type { Dayjs } from 'dayjs';
import { useRouter } from 'vue-router';
import { message, type TableColumnsType } from 'ant-design-vue';
import {
  mailAccountApi,
  sendLogApi,
  type MailAccount,
  type MailSendLog,
} from '../api/client';
import AppLayout from '../components/AppLayout.vue';

const router = useRouter();
const loading = ref(false);
const detailOpen = ref(false);
const sendLogs = ref<MailSendLog[]>([]);
const selectedLog = ref<MailSendLog | null>(null);
const accounts = ref<MailAccount[]>([]);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const dateRange = ref<[Dayjs, Dayjs] | null>(null);
const filters = reactive({
  keyword: '',
  accountId: undefined as string | undefined,
  status: undefined as string | undefined,
  retryStatus: undefined as string | undefined,
});

const accountMap = computed(() => new Map(accounts.value.map((item) => [item.id, item.displayName || item.email])));
const columns: TableColumnsType<MailSendLog> = [
  { title: '邮件', key: 'subject' },
  { title: '发件账号', key: 'account', width: 180 },
  { title: '状态', key: 'status', width: 150 },
  { title: '完成时间', key: 'finishedAt', width: 180 },
  { title: '失败原因', key: 'error', width: 240 },
  { title: '操作', key: 'actions', width: 150 },
];

onMounted(async () => {
  await Promise.all([loadAccounts(), loadSendLogs()]);
});

async function loadAccounts() {
  try {
    accounts.value = (await mailAccountApi.list()).items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取账号失败');
  }
}

async function loadSendLogs() {
  loading.value = true;
  try {
    const data = await sendLogApi.list({
      page: page.value,
      pageSize: pageSize.value,
      keyword: filters.keyword.trim() || undefined,
      accountId: filters.accountId,
      status: filters.status,
      retryStatus: filters.retryStatus,
      dateFrom: dateRange.value?.[0]?.format('YYYY-MM-DD'),
      dateTo: dateRange.value?.[1]?.format('YYYY-MM-DD'),
    });
    sendLogs.value = data.items;
    total.value = data.total;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取发送记录失败');
  } finally {
    loading.value = false;
  }
}

function onFilterChanged() {
  page.value = 1;
  void loadSendLogs();
}

function onDateChanged() {
  onFilterChanged();
}

async function openDetail(item: MailSendLog) {
  try {
    selectedLog.value = await sendLogApi.detail(item.id);
    detailOpen.value = true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取发送记录详情失败');
  }
}

async function openMessage(messageId: string) {
  await router.push({ path: '/mail', query: { messageId } });
}

function accountName(id: string, fallback: string | null) {
  return accountMap.value.get(id) || fallback || '-';
}

function recipientsText(item: MailSendLog) {
  const values = [
    ...(item.recipients?.to || []),
    ...(item.recipients?.cc || []),
    ...(item.recipients?.bcc || []),
  ];
  return values.length ? values.join(', ') : '-';
}

function statusColor(status: string) {
  switch (status) {
    case 'success':
      return 'green';
    case 'local_save_failed':
      return 'orange';
    case 'failed':
      return 'red';
    default:
      return 'blue';
  }
}

function statusLabel(status: string) {
  switch (status) {
    case 'success':
      return '发送成功';
    case 'local_save_failed':
      return '已发出未入库';
    case 'failed':
      return '发送失败';
    case 'sending':
      return '发送中';
    default:
      return status || '-';
  }
}

function retryStatusLabel(status: string) {
  switch (status) {
    case 'retryable':
      return '可重试';
    case 'retrying':
      return '重试中';
    case 'exhausted':
      return '已耗尽';
    default:
      return '无需重试';
  }
}

function formatTime(value: string | null) {
  return value ? new Date(value).toLocaleString('zh-CN') : '-';
}
</script>

<style scoped>
.send-logs-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.send-log-filters {
  display: grid;
  grid-template-columns: minmax(260px, 1.5fr) minmax(160px, 1fr) minmax(140px, 0.8fr) minmax(140px, 0.8fr) minmax(220px, 1.1fr);
  gap: 10px;
}

.send-log-subject-cell {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.send-log-subject-cell strong,
.send-log-subject-cell small,
.send-log-error {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.send-log-subject-cell small,
.send-log-status-cell span {
  color: var(--muted-color);
  font-size: 12px;
}

.send-log-status-cell {
  display: grid;
  gap: 3px;
}

.send-log-error {
  display: block;
  max-width: 220px;
  color: var(--muted-color);
}

.send-log-pagination {
  align-self: flex-end;
}

@media (max-width: 1080px) {
  .send-log-filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .send-log-filters {
    grid-template-columns: 1fr;
  }
}
</style>
