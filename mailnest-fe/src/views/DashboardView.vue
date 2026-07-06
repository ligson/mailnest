<template>
  <AppLayout selected-key="/mail">
    <section
      class="mail-workspace"
      :style="{
        '--folder-pane-width': `${folderPaneWidth}px`,
        '--list-pane-width': `${listPaneWidth}px`,
      }"
    >
      <aside class="mail-folders">
        <div class="folder-heading">邮箱</div>
        <button
          v-for="folder in systemFolders"
          :key="folder.key"
          class="folder-item"
          :class="{ active: activeFolderKey === folder.key }"
          @click="selectSystemFolder(folder.key)"
        >
          <component :is="folder.icon" class="folder-icon" />
          <span>{{ folder.label }}</span>
        </button>

        <div class="folder-section-title">
          <span>文件夹</span>
          <a-button type="link" size="small" @click="openFolderCreate">新增</a-button>
        </div>
        <a-empty v-if="folders.length === 0" image="" description="暂无文件夹" class="folder-empty" />
        <button
          v-for="folder in folders"
          :key="folder.id"
          class="folder-item"
          :class="{ active: activeFolderKey === `folder:${folder.id}` }"
          @click="selectLocalFolder(folder.id)"
        >
          <span class="folder-dot" :style="{ background: folder.color || '#64748b' }"></span>
          <span>{{ folder.name }}</span>
          <a-button class="folder-delete" type="link" size="small" danger @click.stop="deleteFolder(folder)">删除</a-button>
        </button>
      </aside>

      <div class="mail-resizer" title="拖拽调整文件夹栏宽度" @mousedown="startResize('folders', $event)"></div>

      <main class="mail-list-pane">
        <div class="mail-list-header">
          <div>
            <h2 class="mail-page-title">{{ activeFolderLabel }}</h2>
            <p class="mail-count">{{ total }} 封邮件</p>
          </div>
          <a-button @click="refreshAll">刷新</a-button>
        </div>

        <div class="mail-filter-bar">
          <a-input-search
            v-model:value="filters.keyword"
            allow-clear
            placeholder="搜索主题、发件人、正文"
            @search="loadMessages"
            @change="onFilterChanged"
          />
          <div class="filter-row">
            <a-select
              v-model:value="filters.accountId"
              allow-clear
              placeholder="邮箱账号"
              class="filter-control"
              @change="onFilterChanged"
            >
              <a-select-option v-for="account in accounts" :key="account.id" :value="account.id">
                {{ account.displayName }}
              </a-select-option>
            </a-select>
            <a-range-picker v-model:value="dateRange" class="date-filter" @change="onDateChanged" />
            <a-checkbox v-model:checked="filters.hasAttachments" @change="onFilterChanged">有附件</a-checkbox>
          </div>
          <div class="filter-row">
            <a-input v-model:value="filters.from" allow-clear placeholder="发件人" @change="onFilterChanged" />
            <a-input v-model:value="filters.subject" allow-clear placeholder="主题" @change="onFilterChanged" />
          </div>
        </div>

        <a-spin :spinning="loading">
          <div v-if="messages.length === 0" class="mail-list-empty">
            <a-empty description="没有符合条件的邮件" />
          </div>
          <div v-else class="mail-list">
            <button
              v-for="item in messages"
              :key="item.id"
              class="mail-list-item"
              :class="{ active: selectedMessageId === item.id }"
              @click="openDetail(item.id)"
            >
              <div class="mail-item-top">
                <strong>{{ item.from || '未知发件人' }}</strong>
                <span>{{ formatShortTime(item.sentAt || item.receivedAt) }}</span>
              </div>
              <div class="mail-item-subject">
                <paper-clip-outlined v-if="item.hasAttachments" />
                <span>{{ item.subject || '无主题' }}</span>
              </div>
              <div class="mail-item-meta">{{ item.to.join(', ') || '无收件人' }}</div>
            </button>
          </div>
        </a-spin>

        <a-pagination
          v-if="total > pageSize"
          v-model:current="page"
          :page-size="pageSize"
          :total="total"
          size="small"
          class="mail-pagination"
          @change="loadMessages"
        />
      </main>

      <div class="mail-resizer" title="拖拽调整邮件列表宽度" @mousedown="startResize('list', $event)"></div>

      <section class="mail-reader-pane">
        <a-skeleton v-if="detailLoading" active />
        <div v-else-if="detail" class="mail-reader">
          <div class="reader-header">
            <h3 class="mail-subject">{{ detail.subject || '无主题' }}</h3>
            <div class="reader-meta">
              <span>{{ detail.from || '-' }}</span>
              <span>{{ formatTime(detail.sentAt || detail.receivedAt) }}</span>
            </div>
            <div class="reader-address">收件人：{{ detail.to.join(', ') || '-' }}</div>
            <div v-if="detail.cc.length" class="reader-address">抄送：{{ detail.cc.join(', ') }}</div>
          </div>
          <div v-if="detail.htmlBody" class="mail-body" v-html="detail.htmlBody"></div>
          <pre v-else class="mail-text-body">{{ detail.textBody || '没有正文内容' }}</pre>
          <section v-if="normalAttachments.length" class="attachments-panel">
            <h4 class="attachments-title">附件</h4>
            <a-list :data-source="normalAttachments" size="small">
              <template #renderItem="{ item }">
                <a-list-item>
                  <template #actions>
                    <a-button type="link" size="small" @click="downloadAttachment(item)">下载</a-button>
                  </template>
                  <a-list-item-meta>
                    <template #title>{{ item.filename }}</template>
                    <template #description>{{ attachmentDescription(item) }}</template>
                  </a-list-item-meta>
                </a-list-item>
              </template>
            </a-list>
          </section>
        </div>
        <div v-else class="reader-empty">
          <mail-outlined />
          <p>选择一封邮件开始阅读</p>
        </div>
      </section>

      <a-modal
        v-model:open="folderCreateOpen"
        title="新增文件夹"
        ok-text="创建"
        cancel-text="取消"
        @ok="createFolder"
      >
        <a-form layout="vertical">
          <a-form-item label="名称">
            <a-input v-model:value="folderForm.name" placeholder="例如：安全通知" />
          </a-form-item>
          <a-form-item label="颜色">
            <a-input v-model:value="folderForm.color" placeholder="#1f66d1" />
          </a-form-item>
        </a-form>
      </a-modal>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, markRaw, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { InboxOutlined, MailOutlined, PaperClipOutlined } from '@ant-design/icons-vue';
import { Modal, message } from 'ant-design-vue';
import type { Dayjs } from 'dayjs';
import {
  mailAccountApi,
  mailFolderApi,
  messageApi,
  type MailAccount,
  type MailAttachment,
  type MailFolder,
  type MailMessage,
  type MailMessageDetail,
} from '../api/client';
import AppLayout from '../components/AppLayout.vue';

type SystemFolderKey = 'inbox' | 'all' | 'attachments';
type ResizePane = 'folders' | 'list';

const systemFolders = [
  { key: 'inbox' as const, label: '收件箱', icon: markRaw(InboxOutlined) },
  { key: 'all' as const, label: '全部邮件', icon: markRaw(MailOutlined) },
  { key: 'attachments' as const, label: '有附件', icon: markRaw(PaperClipOutlined) },
];

const loading = ref(false);
const detailLoading = ref(false);
const accounts = ref<MailAccount[]>([]);
const folders = ref<MailFolder[]>([]);
const messages = ref<MailMessage[]>([]);
const detail = ref<MailMessageDetail | null>(null);
const selectedMessageId = ref<string | null>(null);
const activeSystemFolder = ref<SystemFolderKey>('inbox');
const activeLocalFolderId = ref<string | null>(null);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const dateRange = ref<[Dayjs, Dayjs] | null>(null);
const folderCreateOpen = ref(false);
const folderPaneWidth = ref(210);
const listPaneWidth = ref(430);
let resizeState: {
  pane: ResizePane;
  startX: number;
  startFolderWidth: number;
  startListWidth: number;
} | null = null;
const folderForm = reactive({
  name: '',
  color: '#1f66d1',
});
const filters = reactive({
  keyword: '',
  from: '',
  subject: '',
  accountId: undefined as string | undefined,
  hasAttachments: false,
});

const normalAttachments = computed(() => (detail.value?.attachments || []).filter((item) => !item.inline));
const activeFolderKey = computed(() => activeLocalFolderId.value ? `folder:${activeLocalFolderId.value}` : activeSystemFolder.value);
const activeFolderLabel = computed(() => {
  if (activeLocalFolderId.value) {
    return folders.value.find((item) => item.id === activeLocalFolderId.value)?.name || '文件夹';
  }
  return systemFolders.find((item) => item.key === activeSystemFolder.value)?.label || '邮件';
});

onMounted(refreshAll);
onBeforeUnmount(stopResize);

async function refreshAll() {
  await Promise.all([loadAccounts(), loadFolders()]);
  await loadMessages();
}

async function loadAccounts() {
  try {
    accounts.value = (await mailAccountApi.list()).items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取邮箱账号失败');
  }
}

async function loadFolders() {
  try {
    folders.value = (await mailFolderApi.list()).items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取文件夹失败');
  }
}

async function loadMessages() {
  loading.value = true;
  try {
    const data = await messageApi.list({
      page: page.value,
      pageSize: pageSize.value,
      accountId: filters.accountId,
      folderId: activeLocalFolderId.value || undefined,
      systemFolder: activeLocalFolderId.value ? undefined : activeSystemFolder.value,
      keyword: filters.keyword || undefined,
      from: filters.from || undefined,
      subject: filters.subject || undefined,
      dateFrom: dateRange.value?.[0]?.format('YYYY-MM-DD'),
      dateTo: dateRange.value?.[1]?.format('YYYY-MM-DD'),
      hasAttachments: filters.hasAttachments || undefined,
    });
    messages.value = data.items;
    total.value = data.total;
    if (!messages.value.some((item) => item.id === selectedMessageId.value)) {
      selectedMessageId.value = null;
      detail.value = null;
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取邮件失败');
  } finally {
    loading.value = false;
  }
}

async function openDetail(id: string) {
  selectedMessageId.value = id;
  detailLoading.value = true;
  detail.value = null;
  try {
    detail.value = await messageApi.detail(id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取邮件详情失败');
  } finally {
    detailLoading.value = false;
  }
}

function selectSystemFolder(key: SystemFolderKey) {
  activeSystemFolder.value = key;
  activeLocalFolderId.value = null;
  page.value = 1;
  void loadMessages();
}

function selectLocalFolder(id: string) {
  activeLocalFolderId.value = id;
  page.value = 1;
  void loadMessages();
}

function openFolderCreate() {
  folderForm.name = '';
  folderForm.color = '#1f66d1';
  folderCreateOpen.value = true;
}

function startResize(pane: ResizePane, event: MouseEvent) {
  resizeState = {
    pane,
    startX: event.clientX,
    startFolderWidth: folderPaneWidth.value,
    startListWidth: listPaneWidth.value,
  };
  document.body.classList.add('mail-resizing');
  window.addEventListener('mousemove', onResizeMove);
  window.addEventListener('mouseup', stopResize);
  event.preventDefault();
}

function onResizeMove(event: MouseEvent) {
  if (!resizeState) {
    return;
  }
  const delta = event.clientX - resizeState.startX;
  if (resizeState.pane === 'folders') {
    folderPaneWidth.value = clamp(resizeState.startFolderWidth + delta, 160, 320);
    return;
  }
  const maxListWidth = Math.max(340, window.innerWidth - folderPaneWidth.value - 420);
  listPaneWidth.value = clamp(resizeState.startListWidth + delta, 320, Math.min(720, maxListWidth));
}

function stopResize() {
  if (!resizeState) {
    return;
  }
  resizeState = null;
  document.body.classList.remove('mail-resizing');
  window.removeEventListener('mousemove', onResizeMove);
  window.removeEventListener('mouseup', stopResize);
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

async function createFolder() {
  if (!folderForm.name.trim()) {
    message.warning('请输入文件夹名称');
    return;
  }
  try {
    await mailFolderApi.create({
      name: folderForm.name.trim(),
      color: folderForm.color.trim(),
      sortOrder: folders.value.length * 10 + 10,
    });
    message.success('文件夹已创建');
    folderCreateOpen.value = false;
    await loadFolders();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '创建文件夹失败');
  }
}

function deleteFolder(folder: MailFolder) {
  Modal.confirm({
    title: `删除文件夹「${folder.name}」？`,
    content: '邮件不会被删除，只会移出该本地文件夹。',
    okText: '删除',
    okType: 'danger',
    cancelText: '取消',
    async onOk() {
      await mailFolderApi.remove(folder.id);
      if (activeLocalFolderId.value === folder.id) {
        activeLocalFolderId.value = null;
        activeSystemFolder.value = 'inbox';
      }
      await loadFolders();
      await loadMessages();
      message.success('文件夹已删除');
    },
  });
}

function onFilterChanged() {
  page.value = 1;
  void loadMessages();
}

function onDateChanged() {
  onFilterChanged();
}

async function downloadAttachment(attachment: MailAttachment) {
  try {
    const blob = await messageApi.downloadAttachment(attachment);
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = attachment.filename || 'attachment';
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '下载附件失败');
  }
}

function attachmentDescription(attachment: MailAttachment) {
  const parts: string[] = [];
  if (attachment.contentType) {
    parts.push(attachment.contentType);
  }
  parts.push(formatSize(attachment.size));
  return parts.join(' · ');
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

function formatShortTime(value: string | null) {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}

function formatTime(value: string | null) {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString();
}
</script>

<style scoped>
.mail-workspace {
  display: grid;
  grid-template-columns: var(--folder-pane-width, 210px) 6px var(--list-pane-width, 430px) 6px minmax(420px, 1fr);
  min-height: calc(100vh - 64px);
  border: 1px solid #d8e1ea;
  border-radius: 8px;
  overflow: hidden;
  background: #ffffff;
}

.mail-resizer {
  position: relative;
  z-index: 2;
  background: #f2f5f9;
  cursor: col-resize;
}

.mail-resizer::after {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 2px;
  width: 2px;
  background: transparent;
  content: '';
}

.mail-resizer:hover::after {
  background: #1f66d1;
}

:global(.mail-resizing) {
  cursor: col-resize;
  user-select: none;
}

.mail-folders {
  padding: 16px 10px;
  background: #f8fafc;
}

.folder-heading,
.folder-section-title {
  padding: 8px 10px;
  color: #667085;
  font-size: 12px;
  font-weight: 700;
}

.folder-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
}

.folder-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  padding: 7px 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #263445;
  cursor: pointer;
  text-align: left;
}

.folder-item > span:nth-child(2) {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.folder-delete {
  opacity: 0;
  padding: 0;
}

.folder-item:hover .folder-delete {
  opacity: 1;
}

.folder-item:hover,
.folder-item.active {
  background: #e8f1ff;
  color: #1459bd;
}

.folder-icon {
  font-size: 15px;
}

.folder-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex: none;
}

.folder-empty {
  padding: 8px 0;
}

.mail-list-pane {
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: #ffffff;
}

.mail-list-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 18px 18px 12px;
}

.mail-page-title {
  margin: 0;
  font-size: 20px;
}

.mail-count {
  margin: 4px 0 0;
  color: #667085;
}

.mail-filter-bar {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 18px 14px;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-control,
.date-filter {
  min-width: 130px;
  flex: 1;
}

.mail-list {
  overflow: auto;
  border-top: 1px solid #eef2f7;
}

.mail-list-empty {
  padding: 48px 12px;
}

.mail-list-item {
  display: block;
  width: 100%;
  padding: 13px 16px;
  border: 0;
  border-left: 3px solid transparent;
  border-bottom: 1px solid #edf1f7;
  background: #ffffff;
  color: #1f2a37;
  cursor: pointer;
  text-align: left;
}

.mail-list-item:hover {
  background: #f8fbff;
}

.mail-list-item.active {
  border-left-color: #1f66d1;
  background: #eef5ff;
}

.mail-item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}

.mail-item-top span,
.mail-item-meta {
  color: #667085;
  font-size: 12px;
}

.mail-item-subject {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 7px;
  font-weight: 700;
  line-height: 1.4;
}

.mail-item-meta {
  margin-top: 5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mail-pagination {
  margin: 12px 16px 16px;
  align-self: flex-end;
}

.mail-reader-pane {
  min-width: 0;
  padding: 22px 26px;
  overflow: auto;
  background: #ffffff;
}

.mail-reader {
  max-width: 880px;
}

.reader-header {
  padding-bottom: 18px;
  border-bottom: 1px solid #edf1f7;
  margin-bottom: 18px;
}

.mail-subject {
  margin: 0 0 10px;
  color: #1f2329;
  font-size: 22px;
  font-weight: 700;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.reader-meta,
.reader-address {
  display: flex;
  gap: 12px;
  color: #667085;
  font-size: 13px;
  line-height: 1.7;
}

.mail-body {
  max-width: 100%;
  overflow-x: auto;
  color: #1f2329;
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.mail-body :deep(img) {
  max-width: 100%;
  height: auto;
}

.mail-body :deep(table) {
  max-width: 100%;
}

.mail-text-body {
  margin: 0;
  color: #1f2329;
  font-family: inherit;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.attachments-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 22px;
  padding-top: 18px;
  border-top: 1px solid #edf1f7;
}

.attachments-title {
  margin: 0;
  color: #1f2329;
  font-size: 15px;
  font-weight: 700;
}

.reader-empty {
  display: grid;
  min-height: 55vh;
  place-items: center;
  align-content: center;
  gap: 12px;
  color: #8a96a8;
}

.reader-empty .anticon {
  font-size: 42px;
}

@media (max-width: 1180px) {
  .mail-workspace {
    grid-template-columns: minmax(180px, var(--folder-pane-width, 210px)) 6px minmax(320px, var(--list-pane-width, 390px)) 6px minmax(360px, 1fr);
  }
}

@media (max-width: 920px) {
  .mail-workspace {
    grid-template-columns: 1fr;
  }

  .mail-folders,
  .mail-list-pane {
    border-right: 0;
    border-bottom: 1px solid #e3e9f2;
  }

  .mail-resizer {
    display: none;
  }
}
</style>
