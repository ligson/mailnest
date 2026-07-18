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
        <div v-if="folders.length === 0" class="folder-empty">
          <folder-open-outlined />
          <span>暂无文件夹</span>
        </div>
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
            <p class="mail-count">{{ mailCountText }}</p>
          </div>
          <a-space>
            <a-button type="primary" @click="openCompose">
              <template #icon><send-outlined /></template>
              写邮件
            </a-button>
            <a-button @click="refreshAll">刷新</a-button>
          </a-space>
        </div>

        <div class="mail-filter-bar">
          <div class="mail-search-box">
            <a-select v-model:value="filters.searchField" class="search-field-select" @change="onFilterChanged">
              <a-select-option value="all">全部</a-select-option>
              <a-select-option value="from">发件人</a-select-option>
              <a-select-option value="subject">主题</a-select-option>
              <a-select-option value="body">正文</a-select-option>
            </a-select>
            <a-input
              v-model:value="filters.keyword"
              allow-clear
              class="search-keyword-input"
              :placeholder="searchPlaceholder"
              @change="onFilterChanged"
              @press-enter="loadMessages"
            />
            <a-button class="search-submit-button" aria-label="搜索" @click="loadMessages">
              <template #icon><search-outlined /></template>
            </a-button>
          </div>
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
            <a-range-picker
              v-model:value="dateRange"
              class="date-filter"
              :placeholder="['开始日期', '结束日期']"
              @change="onDateChanged"
            />
            <a-checkbox v-model:checked="filters.hasAttachments" @change="onFilterChanged">有附件</a-checkbox>
          </div>
        </div>

        <a-spin :spinning="loading">
          <div v-if="loading && !hasLoadedMessages" class="mail-list-skeleton">
            <a-skeleton active :paragraph="{ rows: 8 }" />
          </div>
          <div v-else-if="messages.length === 0" class="mail-list-empty">
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
                <strong>{{ displayAddressName(parseContactAddress(item.from || '')) }}</strong>
                <span>{{ formatShortTime(item.sentAt || item.receivedAt) }}</span>
              </div>
              <div class="mail-item-subject">
                <paper-clip-outlined v-if="item.hasAttachments" />
                <span>{{ item.subject || '无主题' }}</span>
              </div>
              <div class="mail-item-meta">{{ addressSummary(item.to) || '无收件人' }}</div>
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
            <div class="reader-time">{{ formatTime(detail.sentAt || detail.receivedAt) }}</div>
            <div class="reader-address-row">
              <span class="reader-address-label">发件人</span>
              <div class="reader-contact-list">
                <a-popover trigger="click" placement="bottomLeft">
                  <template #content>
                    <div class="contact-popover">
                      <div class="contact-popover-header">
                        <strong>{{ displayAddressName(detailFromAddress) }}</strong>
                        <a-tooltip title="编辑联系人">
                          <a-button
                            class="contact-popover-edit"
                            type="text"
                            size="small"
                            aria-label="编辑联系人"
                            @click.stop="editAddressContact(detailFromAddress)"
                          >
                            <template #icon><edit-outlined /></template>
                          </a-button>
                        </a-tooltip>
                      </div>
                      <span>{{ contactEmail(detailFromAddress) || detailFromAddress.raw }}</span>
                      <span v-if="contactInfo(detailFromAddress)?.phone">电话：{{ contactInfo(detailFromAddress)?.phone }}</span>
                      <span v-if="contactInfo(detailFromAddress)?.company">公司：{{ contactInfo(detailFromAddress)?.company }}</span>
                      <span v-if="contactInfo(detailFromAddress)?.notes">备注：{{ contactInfo(detailFromAddress)?.notes }}</span>
                    </div>
                  </template>
                  <button class="reader-contact-chip" type="button">
                    <span class="reader-contact-name">{{ displayAddressName(detailFromAddress) }}</span>
                  </button>
                </a-popover>
              </div>
            </div>
            <div class="reader-address-row">
              <span class="reader-address-label">收件人</span>
              <div class="reader-contact-list">
                <span v-if="!detailToAddresses.length" class="reader-address-empty">-</span>
                <a-popover v-for="(address, index) in detailToAddresses" :key="`${address.raw}-${index}`" trigger="click" placement="bottomLeft">
                  <template #content>
                    <div class="contact-popover">
                      <div class="contact-popover-header">
                        <strong>{{ displayAddressName(address) }}</strong>
                        <a-tooltip title="编辑联系人">
                          <a-button
                            class="contact-popover-edit"
                            type="text"
                            size="small"
                            aria-label="编辑联系人"
                            @click.stop="editAddressContact(address)"
                          >
                            <template #icon><edit-outlined /></template>
                          </a-button>
                        </a-tooltip>
                      </div>
                      <span>{{ contactEmail(address) || address.raw }}</span>
                      <span v-if="contactInfo(address)?.phone">电话：{{ contactInfo(address)?.phone }}</span>
                      <span v-if="contactInfo(address)?.company">公司：{{ contactInfo(address)?.company }}</span>
                      <span v-if="contactInfo(address)?.notes">备注：{{ contactInfo(address)?.notes }}</span>
                    </div>
                  </template>
                  <button class="reader-contact-chip" type="button">
                    <span class="reader-contact-name">{{ displayAddressName(address) }}</span>
                  </button>
                </a-popover>
              </div>
            </div>
            <div v-if="detailCcAddresses.length" class="reader-address-row">
              <span class="reader-address-label">抄送</span>
              <div class="reader-contact-list">
                <a-popover v-for="(address, index) in detailCcAddresses" :key="`${address.raw}-${index}`" trigger="click" placement="bottomLeft">
                  <template #content>
                    <div class="contact-popover">
                      <div class="contact-popover-header">
                        <strong>{{ displayAddressName(address) }}</strong>
                        <a-tooltip title="编辑联系人">
                          <a-button
                            class="contact-popover-edit"
                            type="text"
                            size="small"
                            aria-label="编辑联系人"
                            @click.stop="editAddressContact(address)"
                          >
                            <template #icon><edit-outlined /></template>
                          </a-button>
                        </a-tooltip>
                      </div>
                      <span>{{ contactEmail(address) || address.raw }}</span>
                      <span v-if="contactInfo(address)?.phone">电话：{{ contactInfo(address)?.phone }}</span>
                      <span v-if="contactInfo(address)?.company">公司：{{ contactInfo(address)?.company }}</span>
                      <span v-if="contactInfo(address)?.notes">备注：{{ contactInfo(address)?.notes }}</span>
                    </div>
                  </template>
                  <button class="reader-contact-chip" type="button">
                    <span class="reader-contact-name">{{ displayAddressName(address) }}</span>
                  </button>
                </a-popover>
              </div>
            </div>
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
            <div class="folder-color-picker">
              <button
                v-for="color in folderColorOptions"
                :key="color"
                class="folder-color-swatch"
                :class="{ selected: folderForm.color === color }"
                :style="{ '--swatch-color': color }"
                type="button"
                :aria-label="`选择颜色 ${color}`"
                @click="folderForm.color = color"
              >
                <check-outlined v-if="folderForm.color === color" />
              </button>
            </div>
          </a-form-item>
        </a-form>
      </a-modal>

      <a-drawer
        v-model:open="composeOpen"
        title="写邮件"
        width="620"
        :destroy-on-close="false"
        class="compose-drawer"
      >
        <a-form layout="vertical" :model="composeForm">
          <a-form-item label="发件账号">
            <a-select v-model:value="composeForm.accountId" placeholder="选择发件邮箱" @change="onComposeAccountChanged">
              <a-select-option v-for="account in accounts" :key="account.id" :value="account.id">
                {{ account.displayName }} &lt;{{ account.email }}&gt;
              </a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="收件人">
            <a-select
              v-model:value="composeForm.to"
              mode="tags"
              :options="contactOptions"
              placeholder="输入邮箱后回车"
              :token-separators="[',', ';', '，', '；']"
            />
          </a-form-item>
          <a-row :gutter="12">
            <a-col :span="12">
              <a-form-item label="抄送">
                <a-select
                  v-model:value="composeForm.cc"
                  mode="tags"
                  :options="contactOptions"
                  placeholder="可选"
                  :token-separators="[',', ';', '，', '；']"
                />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item label="密送">
                <a-select
                  v-model:value="composeForm.bcc"
                  mode="tags"
                  :options="contactOptions"
                  placeholder="可选"
                  :token-separators="[',', ';', '，', '；']"
                />
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item label="主题">
            <a-input v-model:value="composeForm.subject" placeholder="邮件主题" />
          </a-form-item>
          <a-form-item label="正文">
            <div class="compose-editor">
              <div class="compose-toolbar">
                <input
                  ref="composeAttachmentInput"
                  class="compose-file-input"
                  type="file"
                  multiple
                  @change="onComposeFilesSelected"
                />
                <a-tooltip title="添加附件">
                  <a-button type="text" class="compose-tool-button" aria-label="添加附件" @click="chooseComposeFiles">
                    <template #icon><paper-clip-outlined /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip title="插入签名">
                  <a-button type="text" class="compose-tool-button" aria-label="插入签名" @click="insertComposeSignature">
                    <template #icon><edit-outlined /></template>
                  </a-button>
                </a-tooltip>
                <span class="compose-toolbar-divider"></span>
                <a-tooltip title="加粗">
                  <a-button type="text" class="compose-tool-button" aria-label="加粗" @click="runComposeCommand('bold')">
                    <template #icon><bold-outlined /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip title="斜体">
                  <a-button type="text" class="compose-tool-button" aria-label="斜体" @click="runComposeCommand('italic')">
                    <template #icon><italic-outlined /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip title="下划线">
                  <a-button type="text" class="compose-tool-button" aria-label="下划线" @click="runComposeCommand('underline')">
                    <template #icon><underline-outlined /></template>
                  </a-button>
                </a-tooltip>
                <span class="compose-toolbar-divider"></span>
                <a-tooltip title="项目列表">
                  <a-button type="text" class="compose-tool-button" aria-label="项目列表" @click="runComposeCommand('insertUnorderedList')">
                    <template #icon><unordered-list-outlined /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip title="编号列表">
                  <a-button type="text" class="compose-tool-button" aria-label="编号列表" @click="runComposeCommand('insertOrderedList')">
                    <template #icon><ordered-list-outlined /></template>
                  </a-button>
                </a-tooltip>
                <span class="compose-toolbar-divider"></span>
                <a-tooltip title="左对齐">
                  <a-button type="text" class="compose-tool-button" aria-label="左对齐" @click="runComposeCommand('justifyLeft')">
                    <template #icon><align-left-outlined /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip title="居中">
                  <a-button type="text" class="compose-tool-button" aria-label="居中" @click="runComposeCommand('justifyCenter')">
                    <template #icon><align-center-outlined /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip title="右对齐">
                  <a-button type="text" class="compose-tool-button" aria-label="右对齐" @click="runComposeCommand('justifyRight')">
                    <template #icon><align-right-outlined /></template>
                  </a-button>
                </a-tooltip>
                <span class="compose-toolbar-divider"></span>
                <a-tooltip title="插入链接">
                  <a-button type="text" class="compose-tool-button" aria-label="插入链接" @click="insertComposeLink">
                    <template #icon><link-outlined /></template>
                  </a-button>
                </a-tooltip>
              </div>
              <div
                ref="composeEditor"
                class="compose-editor-body"
                contenteditable="true"
                data-placeholder="输入邮件正文"
                @input="onComposeEditorInput"
                @blur="onComposeEditorInput"
              ></div>
            </div>
            <div v-if="composeForm.attachments.length" class="compose-attachments">
              <div v-for="(file, index) in composeForm.attachments" :key="`${file.name}-${file.size}-${index}`" class="compose-attachment-item">
                <paper-clip-outlined />
                <span>{{ file.name }}</span>
                <small>{{ formatSize(file.size) }}</small>
                <a-button type="text" size="small" aria-label="移除附件" @click="removeComposeAttachment(index)">
                  移除
                </a-button>
              </div>
            </div>
          </a-form-item>
        </a-form>
        <template #footer>
          <div class="compose-footer">
            <a-button @click="composeOpen = false">取消</a-button>
            <a-button type="primary" :loading="sending" @click="sendMail">
              <template #icon><send-outlined /></template>
              发送
            </a-button>
          </div>
        </template>
      </a-drawer>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, markRaw, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import {
  AlignCenterOutlined,
  AlignLeftOutlined,
  AlignRightOutlined,
  BoldOutlined,
  CheckOutlined,
  EditOutlined,
  FolderOpenOutlined,
  InboxOutlined,
  ItalicOutlined,
  LinkOutlined,
  MailOutlined,
  OrderedListOutlined,
  PaperClipOutlined,
  SearchOutlined,
  SendOutlined,
  UnderlineOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons-vue';
import { Modal, message } from 'ant-design-vue';
import type { Dayjs } from 'dayjs';
import { useRouter } from 'vue-router';
import {
  mailAccountApi,
  contactApi,
  mailFolderApi,
  messageApi,
  type Contact,
  type MailAccount,
  type MailAttachment,
  type MailFolder,
  type MailMessage,
  type MailMessageDetail,
} from '../api/client';
import AppLayout from '../components/AppLayout.vue';

type SystemFolderKey = 'inbox' | 'sent' | 'all' | 'attachments';
type ResizePane = 'folders' | 'list';
type SearchField = 'all' | 'from' | 'subject' | 'body';
type ContactAddress = {
  raw: string;
  name: string;
  email: string;
};

const systemFolders = [
  { key: 'inbox' as const, label: '收件箱', icon: markRaw(InboxOutlined) },
  { key: 'sent' as const, label: '发件箱', icon: markRaw(SendOutlined) },
  { key: 'all' as const, label: '全部邮件', icon: markRaw(MailOutlined) },
  { key: 'attachments' as const, label: '有附件', icon: markRaw(PaperClipOutlined) },
];
const folderColorOptions = ['#1f66d1', '#0f9f6e', '#d97706', '#dc2626', '#7c3aed', '#0891b2', '#64748b', '#be185d'];
const router = useRouter();

const loading = ref(false);
const detailLoading = ref(false);
const accounts = ref<MailAccount[]>([]);
const folders = ref<MailFolder[]>([]);
const messages = ref<MailMessage[]>([]);
const contacts = ref<Contact[]>([]);
const detail = ref<MailMessageDetail | null>(null);
const selectedMessageId = ref<string | null>(null);
const activeSystemFolder = ref<SystemFolderKey>('inbox');
const activeLocalFolderId = ref<string | null>(null);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const hasLoadedMessages = ref(false);
const dateRange = ref<[Dayjs, Dayjs] | null>(null);
const folderCreateOpen = ref(false);
const composeOpen = ref(false);
const sending = ref(false);
const composeEditor = ref<HTMLElement | null>(null);
const composeAttachmentInput = ref<HTMLInputElement | null>(null);
const folderPaneWidth = ref(210);
const listPaneWidth = ref(430);
let composeSignatureInserted = false;
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
const composeForm = reactive({
  accountId: '',
  to: [] as string[],
  cc: [] as string[],
  bcc: [] as string[],
  subject: '',
  textBody: '',
  htmlBody: '',
  attachments: [] as File[],
});
const filters = reactive({
  keyword: '',
  searchField: 'all' as SearchField,
  accountId: undefined as string | undefined,
  hasAttachments: false,
});

const normalAttachments = computed(() => (detail.value?.attachments || []).filter((item) => !item.inline));
const selectedComposeAccount = computed(() => accounts.value.find((account) => account.id === composeForm.accountId));
const detailFromAddress = computed(() => parseContactAddress(detail.value?.from || ''));
const detailToAddresses = computed(() => parseContactAddresses(detail.value?.to || []));
const detailCcAddresses = computed(() => parseContactAddresses(detail.value?.cc || []));
const contactByEmail = computed(() => {
  const map = new Map<string, Contact>();
  for (const contact of contacts.value) {
    map.set(contact.email.toLowerCase(), contact);
  }
  return map;
});
const contactOptions = computed(() => contacts.value.map((contact) => ({
  value: contact.displayName || contact.nickname
    ? `${contact.displayName || contact.nickname} <${contact.email}>`
    : contact.email,
  label: `${contact.name} <${contact.email}>`,
})));
const searchPlaceholder = computed(() => {
  const placeholders: Record<SearchField, string> = {
    all: '搜索主题、发件人、正文',
    from: '搜索发件人',
    subject: '搜索主题',
    body: '搜索正文',
  };
  return placeholders[filters.searchField];
});
const activeFolderKey = computed(() => activeLocalFolderId.value ? `folder:${activeLocalFolderId.value}` : activeSystemFolder.value);
const activeFolderLabel = computed(() => {
  if (activeLocalFolderId.value) {
    return folders.value.find((item) => item.id === activeLocalFolderId.value)?.name || '文件夹';
  }
  return systemFolders.find((item) => item.key === activeSystemFolder.value)?.label || '邮件';
});
const mailCountText = computed(() => (hasLoadedMessages.value ? `${total.value} 封邮件` : '加载中...'));

onMounted(refreshAll);
onBeforeUnmount(stopResize);

async function refreshAll() {
  await Promise.all([loadAccounts(), loadFolders(), loadContacts(), loadMessages()]);
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

async function loadContacts() {
  try {
    contacts.value = (await contactApi.list({ pageSize: 1000 })).items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取联系人失败');
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
      keyword: keywordQuery(),
      from: fieldQuery('from'),
      subject: fieldQuery('subject'),
      body: fieldQuery('body'),
      dateFrom: dateRange.value?.[0]?.format('YYYY-MM-DD'),
      dateTo: dateRange.value?.[1]?.format('YYYY-MM-DD'),
      hasAttachments: filters.hasAttachments || undefined,
    });
    messages.value = data.items;
    total.value = data.total;
    hasLoadedMessages.value = true;
    if (!messages.value.some((item) => item.id === selectedMessageId.value)) {
      selectedMessageId.value = null;
      detail.value = null;
    }
    if (!selectedMessageId.value && messages.value.length > 0) {
      void openDetail(messages.value[0].id);
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

function openCompose() {
  if (accounts.value.length === 0) {
    message.warning('请先新增邮箱账号');
    return;
  }
  resetCompose();
  composeForm.accountId = filters.accountId
    || accounts.value.find((account) => account.smtpConfigured && account.enabled)?.id
    || accounts.value.find((account) => account.enabled)?.id
    || accounts.value[0].id;
  composeOpen.value = true;
  window.setTimeout(() => {
    resetComposeEditor();
    insertComposeSignatureIfEmpty();
  });
}

async function sendMail() {
  if (!composeForm.accountId) {
    message.warning('请选择发件账号');
    return;
  }
  const to = normalizeComposeAddresses(composeForm.to);
  const cc = normalizeComposeAddresses(composeForm.cc);
  const bcc = normalizeComposeAddresses(composeForm.bcc);
  if (!to.length && !cc.length && !bcc.length) {
    message.warning('请填写至少一个收件人');
    return;
  }
  syncComposeEditorContent();
  if (!composeForm.subject.trim() && !composeForm.textBody.trim() && composeForm.attachments.length === 0) {
    message.warning('主题和正文不能同时为空');
    return;
  }
  sending.value = true;
  try {
    const sent = await messageApi.send({
      accountId: composeForm.accountId,
      to,
      cc,
      bcc,
      subject: composeForm.subject.trim(),
      textBody: composeForm.textBody,
      htmlBody: composeForm.htmlBody,
      attachments: composeForm.attachments,
    });
    message.success('邮件已发送');
    composeOpen.value = false;
    resetCompose();
    activeSystemFolder.value = 'sent';
    activeLocalFolderId.value = null;
    page.value = 1;
    await Promise.all([loadContacts(), loadMessages()]);
    await openDetail(sent.id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '发送失败');
  } finally {
    sending.value = false;
  }
}

function resetCompose() {
  Object.assign(composeForm, {
    accountId: filters.accountId || '',
    to: [],
    cc: [],
    bcc: [],
    subject: '',
    textBody: '',
    htmlBody: '',
    attachments: [],
  });
  composeSignatureInserted = false;
  resetComposeEditor();
  if (composeAttachmentInput.value) {
    composeAttachmentInput.value.value = '';
  }
}

function onComposeAccountChanged() {
  insertComposeSignatureIfEmpty();
}

function onComposeEditorInput() {
  syncComposeEditorContent();
}

function syncComposeEditorContent() {
  const editor = composeEditor.value;
  if (!editor) {
    return;
  }
  composeForm.htmlBody = editor.innerHTML.trim();
  composeForm.textBody = editor.innerText.trim();
}

function resetComposeEditor() {
  if (composeEditor.value) {
    composeEditor.value.innerHTML = composeForm.htmlBody;
  }
}

function insertComposeSignatureIfEmpty() {
  if (composeSignatureInserted || !selectedComposeAccount.value?.signatureHtml || !composeEditor.value) {
    return;
  }
  syncComposeEditorContent();
  if (composeForm.textBody || composeForm.htmlBody.replace(/<br\s*\/?>|&nbsp;/gi, '').trim()) {
    return;
  }
  composeEditor.value.innerHTML = `<br><br>${selectedComposeAccount.value.signatureHtml}`;
  composeSignatureInserted = true;
  syncComposeEditorContent();
}

function insertComposeSignature() {
  if (!selectedComposeAccount.value?.signatureHtml) {
    message.info('当前发件账号还没有维护签名模板');
    return;
  }
  insertHTMLAtCursor(`<br>${selectedComposeAccount.value.signatureHtml}`);
  composeSignatureInserted = true;
  syncComposeEditorContent();
}

function runComposeCommand(command: string) {
  composeEditor.value?.focus();
  document.execCommand(command);
  syncComposeEditorContent();
}

function insertComposeLink() {
  const value = window.prompt('链接地址');
  if (!value?.trim()) {
    return;
  }
  composeEditor.value?.focus();
  document.execCommand('createLink', false, value.trim());
  syncComposeEditorContent();
}

function insertHTMLAtCursor(html: string) {
  composeEditor.value?.focus();
  document.execCommand('insertHTML', false, html);
}

function chooseComposeFiles() {
  composeAttachmentInput.value?.click();
}

function onComposeFilesSelected(event: Event) {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files || []);
  const existingKeys = new Set(composeForm.attachments.map((file) => `${file.name}:${file.size}:${file.lastModified}`));
  for (const file of files) {
    const key = `${file.name}:${file.size}:${file.lastModified}`;
    if (!existingKeys.has(key)) {
      composeForm.attachments.push(file);
      existingKeys.add(key);
    }
  }
  input.value = '';
}

function removeComposeAttachment(index: number) {
  composeForm.attachments.splice(index, 1);
}

function normalizeComposeAddresses(values: string[]) {
  return values
    .map((value) => value.trim())
    .filter(Boolean);
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

function keywordQuery() {
  const keyword = filters.keyword.trim();
  if (!keyword) {
    return undefined;
  }
  return filters.searchField === 'all' ? keyword : undefined;
}

function fieldQuery(field: Exclude<SearchField, 'all'>) {
  const keyword = filters.keyword.trim();
  if (!keyword) {
    return undefined;
  }
  return filters.searchField === field ? keyword : undefined;
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

function parseContactAddresses(values: string[]) {
  return values.map(parseContactAddress).filter((item) => item.name || item.email);
}

function addressSummary(values: string[]) {
  return parseContactAddresses(values).map(displayAddressName).join(', ');
}

function parseContactAddress(value: string): ContactAddress {
  const raw = value.trim();
  if (!raw) {
    return { raw: '-', name: '-', email: '' };
  }

  const matched = raw.match(/^(.*?)\s*<([^<>]+)>$/);
  if (!matched) {
    if (looksLikeEmail(raw)) {
      const emailName = raw.split('@')[0] || raw;
      return { raw, name: emailName, email: raw };
    }
    return { raw, name: raw, email: '' };
  }

  const email = matched[2].trim();
  const displayName = matched[1].trim().replace(/^"|"$/g, '');
  const fallbackName = email.split('@')[0] || email;
  const name = displayName && displayName.toLowerCase() !== email.toLowerCase() ? displayName : fallbackName;
  return { raw, name, email };
}

function displayAddressName(address: ContactAddress) {
  const contact = contactInfo(address);
  return contact?.nickname || contact?.displayName || address.name || address.email || '未知联系人';
}

function contactEmail(address: ContactAddress) {
  return address.email || (looksLikeEmail(address.raw) ? address.raw : '');
}

function contactInfo(address: ContactAddress) {
  const email = contactEmail(address).toLowerCase();
  return email ? contactByEmail.value.get(email) : undefined;
}

async function editAddressContact(address: ContactAddress) {
  const email = contactEmail(address).trim();
  if (!email) {
    message.warning('这个联系人没有可编辑的邮箱地址');
    return;
  }
  const displayName = address.name && !looksLikeEmail(address.name) ? address.name : '';
  await router.push({
    path: '/contacts',
    query: {
      email,
      ...(displayName ? { displayName } : {}),
    },
  });
}

function looksLikeEmail(value: string) {
  return /^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(value.trim());
}
</script>

<style scoped>
.mail-workspace {
  display: grid;
  grid-template-columns: var(--folder-pane-width, 210px) 6px var(--list-pane-width, 430px) 6px minmax(420px, 1fr);
  height: 100%;
  min-height: 0;
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
  min-height: 0;
  padding: 16px 10px;
  overflow: auto;
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
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px;
  color: #8a96a8;
  font-size: 13px;
}

.folder-empty .anticon {
  color: #9aa7b8;
  font-size: 15px;
}

.folder-color-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.folder-color-swatch {
  display: inline-flex;
  width: 34px;
  height: 34px;
  align-items: center;
  justify-content: center;
  border: 2px solid transparent;
  border-radius: 8px;
  background: var(--swatch-color);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.36);
  color: #ffffff;
  cursor: pointer;
}

.folder-color-swatch:hover,
.folder-color-swatch.selected {
  border-color: #1f2329;
}

.folder-color-swatch .anticon {
  font-size: 16px;
  filter: drop-shadow(0 1px 1px rgba(31, 35, 41, 0.35));
}

.mail-list-pane {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  background: #ffffff;
}

.mail-list-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex: none;
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
  display: grid;
  gap: 10px;
  flex: none;
  padding: 0 18px 16px;
}

.mail-search-box {
  display: grid;
  grid-template-columns: 102px minmax(0, 1fr) 46px;
  width: 100%;
}

.search-field-select {
  min-width: 0;
}

.mail-search-box :deep(.ant-select-selector) {
  height: 40px !important;
  border-start-end-radius: 0 !important;
  border-end-end-radius: 0 !important;
}

.mail-search-box :deep(.ant-select-selection-item) {
  line-height: 38px !important;
}

.search-keyword-input {
  height: 40px;
  border-radius: 0;
  margin-left: -1px;
}

.search-keyword-input:hover,
.search-keyword-input:focus {
  position: relative;
  z-index: 1;
}

.search-submit-button {
  width: 46px;
  height: 40px;
  border-start-start-radius: 0;
  border-end-start-radius: 0;
  margin-left: -1px;
  color: #667085;
}

.search-submit-button:hover,
.search-submit-button:focus {
  position: relative;
  z-index: 1;
  color: #1f66d1;
}

.filter-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(190px, 1fr) auto;
  align-items: center;
  gap: 10px;
}

.filter-control,
.date-filter {
  width: 100%;
  min-width: 0;
}

.filter-row :deep(.ant-select-selector),
.filter-row :deep(.ant-picker),
.filter-row :deep(.ant-checkbox-wrapper) {
  min-height: 38px;
}

.filter-row :deep(.ant-checkbox-wrapper) {
  display: flex;
  align-items: center;
  white-space: nowrap;
}

@media (max-width: 1280px) {
  .filter-row {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .filter-control {
    grid-column: 1 / -1;
  }
}

@media (max-width: 1080px) {
  .mail-search-box {
    grid-template-columns: 88px minmax(0, 1fr) 42px;
  }

  .filter-row {
    grid-template-columns: 1fr;
  }
}

.mail-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  border-top: 1px solid #eef2f7;
}

.mail-list-empty {
  flex: 1;
  min-height: 0;
  padding: 48px 12px;
  overflow: auto;
}

.mail-list-skeleton {
  flex: 1;
  min-height: 0;
  padding: 16px 18px 0;
  overflow: hidden;
}

.mail-list-pane :deep(.ant-spin-nested-loading),
.mail-list-pane :deep(.ant-spin-container) {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
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
  flex: none;
  margin: 12px 16px 16px;
  align-self: flex-end;
}

.mail-reader-pane {
  min-width: 0;
  min-height: 0;
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

.reader-time {
  margin-bottom: 12px;
  color: #667085;
  font-size: 13px;
}

.reader-address-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-top: 8px;
  color: #667085;
  font-size: 13px;
  line-height: 1.7;
}

.reader-address-label {
  width: 46px;
  flex: none;
  color: #8a96a8;
  font-weight: 600;
  text-align: right;
}

.reader-contact-list {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-wrap: wrap;
  gap: 6px;
}

.reader-contact-chip {
  display: inline-flex;
  max-width: 100%;
  align-items: baseline;
  gap: 5px;
  padding: 2px 8px;
  border: 1px solid #d8e1ea;
  border-radius: 6px;
  background: #f8fafc;
  color: #263445;
  cursor: pointer;
  font: inherit;
  line-height: 1.55;
}

.reader-contact-chip:hover {
  border-color: #9db2c5;
  background: #eef5ff;
}

.reader-contact-name,
.reader-contact-email {
  min-width: 0;
  overflow-wrap: anywhere;
}

.reader-contact-name {
  font-weight: 600;
}

.reader-contact-email {
  color: #667085;
  font-size: 12px;
}

.reader-address-empty {
  color: #8a96a8;
}

.contact-popover {
  display: grid;
  max-width: 280px;
  min-width: 190px;
  gap: 6px;
  color: #263445;
  font-size: 13px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.contact-popover-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 26px;
  align-items: center;
  gap: 10px;
}

.contact-popover strong {
  min-width: 0;
  color: #111827;
  overflow-wrap: anywhere;
}

.contact-popover-edit {
  width: 26px;
  height: 26px;
  color: #64748b;
}

.contact-popover-edit:hover {
  color: #1f66d1;
  background: #eef5ff;
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

.compose-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.compose-drawer :deep(.ant-drawer-body) {
  padding-bottom: 12px;
}

.compose-drawer :deep(.ant-select-selection-overflow) {
  align-items: center;
}

.compose-editor {
  border: 1px solid #d9e2ec;
  border-radius: 8px;
  background: #ffffff;
  overflow: hidden;
}

.compose-toolbar {
  display: flex;
  min-height: 42px;
  align-items: center;
  gap: 2px;
  padding: 5px 8px;
  border-bottom: 1px solid #edf1f7;
  background: #f8fafc;
  overflow-x: auto;
}

.compose-file-input {
  display: none;
}

.compose-tool-button {
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  color: #475569;
}

.compose-toolbar-divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  margin: 0 5px;
  background: #d8e0ea;
}

.compose-editor-body {
  min-height: 280px;
  max-height: 42vh;
  padding: 14px 16px;
  color: #1f2329;
  line-height: 1.7;
  outline: none;
  overflow-y: auto;
  overflow-wrap: anywhere;
}

.compose-editor-body:empty::before {
  color: #9ca3af;
  content: attr(data-placeholder);
}

.compose-editor-body :deep(img) {
  max-width: 100%;
  height: auto;
}

.compose-attachments {
  display: grid;
  gap: 8px;
  margin-top: 10px;
}

.compose-attachment-item {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border: 1px solid #e5ebf3;
  border-radius: 8px;
  background: #f8fafc;
  color: #334155;
  font-size: 13px;
}

.compose-attachment-item span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compose-attachment-item small {
  color: #64748b;
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
