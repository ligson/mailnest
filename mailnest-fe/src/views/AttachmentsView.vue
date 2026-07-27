<template>
  <AppLayout selected-key="/attachments">
    <section class="content-panel attachments-page">
      <div class="page-toolbar">
        <div>
          <h2 class="page-title">附件中心</h2>
          <p class="page-subtitle">集中查找、下载和定位邮件附件</p>
        </div>
        <a-button @click="loadAttachments">刷新</a-button>
      </div>

      <div class="attachment-filters">
        <a-input-search
          v-model:value="filters.keyword"
          allow-clear
          placeholder="搜索附件名"
          @search="onFilterChanged"
          @change="onFilterChanged"
        />
        <a-select v-model:value="filters.contentType" allow-clear placeholder="类型" @change="onFilterChanged">
          <a-select-option value="image/">图片</a-select-option>
          <a-select-option value="application/pdf">PDF</a-select-option>
          <a-select-option value="application/vnd">Office</a-select-option>
          <a-select-option value="text/">文本</a-select-option>
        </a-select>
        <a-select v-model:value="filters.accountId" allow-clear placeholder="邮箱账号" @change="onFilterChanged">
          <a-select-option v-for="account in accounts" :key="account.id" :value="account.id">
            {{ account.displayName || account.email }}
          </a-select-option>
        </a-select>
        <a-select v-model:value="filters.folderId" allow-clear placeholder="本地文件夹" @change="onFilterChanged">
          <a-select-option v-for="folder in folders" :key="folder.id" :value="folder.id">{{ folder.name }}</a-select-option>
        </a-select>
        <a-range-picker v-model:value="dateRange" @change="onDateChanged" />
      </div>

      <a-table
        row-key="id"
        :columns="columns"
        :data-source="attachments"
        :loading="loading"
        :pagination="false"
        size="middle"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'filename'">
            <div class="attachment-name-cell">
              <paper-clip-outlined />
              <div>
                <strong>{{ record.filename }}</strong>
                <small>{{ record.contentType || '未知类型' }} · {{ formatSize(record.size) }}</small>
              </div>
            </div>
          </template>
          <template v-if="column.key === 'message'">
            <div class="attachment-message-cell">
              <strong>{{ record.messageSubject || '无主题' }}</strong>
              <small>{{ record.messageFrom || '-' }}</small>
            </div>
          </template>
          <template v-if="column.key === 'account'">
            {{ accountName(record.accountId) }}
          </template>
          <template v-if="column.key === 'folder'">
            {{ folderName(record.folderId) }}
          </template>
          <template v-if="column.key === 'messageTime'">
            {{ formatTime(record.messageTime) }}
          </template>
          <template v-if="column.key === 'actions'">
            <a-space>
              <a-button type="link" size="small" @click="preview(record)">预览</a-button>
              <a-button type="link" size="small" @click="download(record)">下载</a-button>
              <a-button type="link" size="small" @click="openSource(record)">查看邮件</a-button>
            </a-space>
          </template>
        </template>
      </a-table>

      <a-pagination
        v-if="total > pageSize"
        v-model:current="page"
        :page-size="pageSize"
        :total="total"
        class="attachment-pagination"
        @change="loadAttachments"
      />

      <a-modal
        v-model:open="previewOpen"
        :title="previewTitle"
        :width="920"
        :footer="null"
        destroy-on-close
        class="attachment-preview-modal"
        @cancel="closePreview"
      >
        <a-spin :spinning="previewLoading">
          <div v-if="previewError" class="attachment-preview-empty">
            <p>{{ previewError }}</p>
            <a-button v-if="previewItem" type="primary" @click="download(previewItem)">下载附件</a-button>
          </div>
          <img v-else-if="previewKind === 'image' && previewUrl" class="attachment-preview-image" :src="previewUrl" :alt="previewItem?.filename || '附件预览'" />
          <iframe v-else-if="previewKind === 'pdf' && previewUrl" class="attachment-preview-frame" :src="previewUrl" title="PDF 预览"></iframe>
          <pre v-else-if="previewKind === 'text'" class="attachment-preview-text">{{ previewText }}</pre>
          <div v-else class="attachment-preview-empty">
            <p>这种附件暂不支持在线预览，可以先下载后用本地应用打开。</p>
            <a-button v-if="previewItem" type="primary" @click="download(previewItem)">下载附件</a-button>
          </div>
        </a-spin>
      </a-modal>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import type { Dayjs } from 'dayjs';
import { useRouter } from 'vue-router';
import { message, type TableColumnsType } from 'ant-design-vue';
import { PaperClipOutlined } from '@ant-design/icons-vue';
import {
  attachmentApi,
  mailAccountApi,
  mailFolderApi,
  messageApi,
  type AttachmentCenterItem,
  type MailAccount,
  type MailFolder,
} from '../api/client';
import AppLayout from '../components/AppLayout.vue';

const router = useRouter();
const loading = ref(false);
const previewOpen = ref(false);
const previewLoading = ref(false);
const previewUrl = ref('');
const previewText = ref('');
const previewError = ref('');
const previewKind = ref<'image' | 'pdf' | 'text' | 'unsupported'>('unsupported');
const previewItem = ref<AttachmentCenterItem | null>(null);
const attachments = ref<AttachmentCenterItem[]>([]);
const accounts = ref<MailAccount[]>([]);
const folders = ref<MailFolder[]>([]);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const dateRange = ref<[Dayjs, Dayjs] | null>(null);
const filters = reactive({
  keyword: '',
  contentType: undefined as string | undefined,
  accountId: undefined as string | undefined,
  folderId: undefined as string | undefined,
});

const accountMap = computed(() => new Map(accounts.value.map((item) => [item.id, item.displayName || item.email])));
const folderMap = computed(() => new Map(folders.value.map((item) => [item.id, item.name])));
const columns: TableColumnsType<AttachmentCenterItem> = [
  { title: '附件', key: 'filename', dataIndex: 'filename' },
  { title: '来源邮件', key: 'message', width: 260 },
  { title: '账号', key: 'account', width: 150 },
  { title: '文件夹', key: 'folder', width: 130 },
  { title: '邮件时间', key: 'messageTime', width: 170 },
  { title: '操作', key: 'actions', width: 190 },
];
const previewTitle = computed(() => previewItem.value?.filename ? `预览：${previewItem.value.filename}` : '附件预览');

onMounted(async () => {
  await Promise.all([loadMeta(), loadAttachments()]);
});

onBeforeUnmount(() => {
  closePreview();
});

async function loadMeta() {
  try {
    const [accountData, folderData] = await Promise.all([mailAccountApi.list(), mailFolderApi.list()]);
    accounts.value = accountData.items;
    folders.value = folderData.items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取筛选数据失败');
  }
}

async function loadAttachments() {
  loading.value = true;
  try {
    const data = await attachmentApi.list({
      page: page.value,
      pageSize: pageSize.value,
      keyword: filters.keyword.trim() || undefined,
      contentType: filters.contentType,
      accountId: filters.accountId,
      folderId: filters.folderId,
      inline: false,
      dateFrom: dateRange.value?.[0]?.format('YYYY-MM-DD'),
      dateTo: dateRange.value?.[1]?.format('YYYY-MM-DD'),
    });
    attachments.value = data.items;
    total.value = data.total;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取附件失败');
  } finally {
    loading.value = false;
  }
}

function onFilterChanged() {
  page.value = 1;
  void loadAttachments();
}

function onDateChanged() {
  onFilterChanged();
}

async function download(item: AttachmentCenterItem) {
  try {
    const blob = await messageApi.downloadAttachment(item);
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = item.filename || 'attachment';
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '下载附件失败');
  }
}

async function preview(item: AttachmentCenterItem) {
  closePreview();
  previewItem.value = item;
  previewKind.value = previewKindFor(item);
  previewOpen.value = true;
  if (previewKind.value === 'unsupported') {
    return;
  }
  previewLoading.value = true;
  try {
    const blob = await attachmentApi.preview(item.id);
    if (previewKind.value === 'text') {
      previewText.value = await blob.text();
      return;
    }
    previewUrl.value = URL.createObjectURL(blob);
  } catch (error) {
    previewError.value = error instanceof Error ? error.message : '预览附件失败';
  } finally {
    previewLoading.value = false;
  }
}

function closePreview() {
  if (previewUrl.value) {
    URL.revokeObjectURL(previewUrl.value);
  }
  previewUrl.value = '';
  previewText.value = '';
  previewError.value = '';
  previewKind.value = 'unsupported';
}

function previewKindFor(item: AttachmentCenterItem): 'image' | 'pdf' | 'text' | 'unsupported' {
  const contentType = (item.contentType || '').toLowerCase();
  const filename = (item.filename || '').toLowerCase();
  if (contentType.startsWith('image/')) {
    return 'image';
  }
  if (contentType === 'application/pdf' || filename.endsWith('.pdf')) {
    return 'pdf';
  }
  if (contentType.startsWith('text/') || contentType.includes('json') || contentType.includes('xml') || /\.(txt|csv|log|json|xml|md|html?)$/.test(filename)) {
    return 'text';
  }
  return 'unsupported';
}

async function openSource(item: AttachmentCenterItem) {
  await router.push({ path: '/mail', query: { messageId: item.messageId } });
}

function accountName(id: string) {
  return accountMap.value.get(id) || '-';
}

function folderName(id: string | null) {
  return id ? folderMap.value.get(id) || '-' : '-';
}

function formatSize(value: number) {
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function formatTime(value: string | null) {
  return value ? new Date(value).toLocaleString('zh-CN') : '-';
}
</script>

<style scoped>
.attachments-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.attachment-filters {
  display: grid;
  grid-template-columns: minmax(220px, 1.4fr) minmax(130px, 0.8fr) minmax(160px, 1fr) minmax(150px, 1fr) minmax(220px, 1.2fr);
  gap: 10px;
}

.attachment-name-cell,
.attachment-message-cell {
  display: flex;
  min-width: 0;
  gap: 10px;
}

.attachment-name-cell > div,
.attachment-message-cell {
  display: grid;
  gap: 3px;
}

.attachment-name-cell strong,
.attachment-message-cell strong,
.attachment-name-cell small,
.attachment-message-cell small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attachment-name-cell small,
.attachment-message-cell small {
  color: var(--muted-color);
}

.attachment-pagination {
  align-self: flex-end;
}

.attachment-preview-modal :deep(.ant-modal-body) {
  min-height: 360px;
}

.attachment-preview-image {
  display: block;
  max-width: 100%;
  max-height: 70vh;
  margin: 0 auto;
  object-fit: contain;
}

.attachment-preview-frame {
  width: 100%;
  height: 70vh;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  background: var(--surface-muted);
}

.attachment-preview-text {
  min-height: 360px;
  max-height: 70vh;
  padding: 14px;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  margin: 0;
  overflow: auto;
  background: var(--surface-muted);
  color: var(--text-color);
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', monospace;
  font-size: 13px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

.attachment-preview-empty {
  display: grid;
  min-height: 300px;
  place-items: center;
  align-content: center;
  gap: 14px;
  color: var(--muted-color);
  text-align: center;
}

@media (max-width: 1100px) {
  .attachment-filters {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 680px) {
  .attachment-filters {
    grid-template-columns: 1fr;
  }
}
</style>
