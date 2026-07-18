<template>
  <AppLayout selected-key="/accounts">
    <section class="content-panel">
      <div class="page-toolbar">
        <div>
          <h2 class="page-title">邮箱账号</h2>
          <p class="page-subtitle">配置多个 IMAP 邮箱账号，后续收取任务会按账号运行</p>
        </div>
        <a-space>
          <a-button :loading="oauthLoading" @click="startMicrosoftOAuth">Microsoft 授权</a-button>
          <a-button type="primary" @click="openCreate">新增邮箱</a-button>
        </a-space>
      </div>

      <a-table
        row-key="id"
        :columns="columns"
        :data-source="accounts"
        :loading="loading"
        :pagination="false"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'enabled'">
            <a-tag :color="record.enabled ? 'green' : 'default'">
              {{ record.enabled ? '启用' : '停用' }}
            </a-tag>
          </template>
          <template v-if="column.key === 'syncStatus'">
            <div class="account-sync-cell">
              <a-tag :color="fullSyncTagColor(record.fullSyncStatus)">
                {{ fullSyncStatusText(record.fullSyncStatus) }}
              </a-tag>
              <a-progress
                v-if="record.fullSyncStatus === 'running' || record.fullSyncTotal > 0"
                :percent="fullSyncPercent(record)"
                size="small"
                :show-info="false"
              />
              <span class="account-sync-hint">
                {{ fullSyncHint(record) }}
              </span>
              <span v-if="pollErrors[record.id]" class="account-sync-error">
                {{ pollErrors[record.id] }}
              </span>
            </div>
          </template>
          <template v-if="column.key === 'authType'">
            <a-tag :color="record.authType === 'oauth2' ? 'blue' : 'default'">
              {{ record.authType === 'oauth2' ? 'OAuth2' : '密码' }}
            </a-tag>
          </template>
          <template v-if="column.key === 'action'">
            <div class="account-actions">
              <a-button size="small" type="primary" :loading="syncingId === record.id" @click="syncAccount(record.id)">收取</a-button>
              <a-button
                v-if="record.fullSyncStatus !== 'running'"
                size="small"
                :loading="fullSyncingId === record.id"
                @click="startFullSync(record.id)"
              >
                同步全部
              </a-button>
              <a-popconfirm
                v-else
                title="确定停止当前全量同步？已同步到本地的邮件会保留。"
                ok-text="停止"
                cancel-text="取消"
                @confirm="stopFullSync(record.id)"
              >
                <a-button size="small" danger :loading="stoppingId === record.id">停止同步</a-button>
              </a-popconfirm>
              <a-dropdown :trigger="['click']">
                <a-button size="small">
                  更多
                  <down-outlined />
                </a-button>
                <template #overlay>
                  <a-menu @click="handleAccountMenuClick(record, $event)">
                    <a-menu-item key="edit">编辑</a-menu-item>
                    <a-menu-item key="test" :disabled="testingId === record.id">测试连接</a-menu-item>
                    <a-menu-item key="sync-log">同步日志</a-menu-item>
                    <a-menu-divider />
                    <a-menu-item key="delete" danger>删除账号</a-menu-item>
                  </a-menu>
                </template>
              </a-dropdown>
            </div>
          </template>
        </template>
      </a-table>

      <a-modal
        v-model:open="modalOpen"
        :title="modalTitle"
        :footer="null"
        :width="760"
        class="account-modal"
        destroy-on-close
      >
        <a-form class="account-form" layout="vertical" :model="form" @finish="saveAccount">
          <a-tabs v-model:active-key="accountFormTab" class="account-form-tabs">
            <a-tab-pane key="basic" tab="基础信息">
              <section class="account-form-section">
                <div class="account-form-grid">
                  <a-form-item label="显示名称" name="displayName" :rules="[{ required: true, message: '请输入显示名称' }]">
                    <a-input v-model:value="form.displayName" />
                  </a-form-item>
                  <a-form-item label="邮箱地址" name="email" :rules="[{ required: true, type: 'email', message: '请输入有效邮箱' }]">
                    <a-input v-model:value="form.email" />
                  </a-form-item>
                </div>
              </section>
            </a-tab-pane>

            <a-tab-pane key="imap" tab="收信 IMAP">
              <section class="account-form-section">
                <div class="account-form-grid">
                  <a-form-item label="IMAP 主机" name="imapHost" :rules="[{ required: true, message: '请输入 IMAP 主机' }]">
                    <a-input v-model:value="form.imapHost" placeholder="imap.example.com" />
                  </a-form-item>
                  <a-form-item label="IMAP 端口" name="imapPort" :rules="[{ required: true, message: '请输入端口' }]">
                    <a-input-number v-model:value="form.imapPort" :min="1" :max="65535" style="width: 100%" />
                  </a-form-item>
                  <a-form-item label="登录用户名" name="imapUsername" :rules="[{ required: true, message: '请输入登录用户名' }]">
                    <a-input v-model:value="form.imapUsername" />
                  </a-form-item>
                  <a-form-item label="邮箱密码或授权码" name="imapPassword" :rules="passwordRules">
                    <a-input-password v-model:value="form.imapPassword" :placeholder="editingId ? '留空则不修改' : ''" />
                  </a-form-item>
                </div>
                <div class="account-toggle-row">
                  <a-checkbox v-model:checked="form.imapTls">使用 TLS</a-checkbox>
                </div>
                <details class="advanced-settings">
                  <summary>高级设置</summary>
                  <a-form-item class="account-form-wide" label="发件箱文件夹名" name="sentFolder">
                    <div class="folder-picker">
                      <a-select
                        v-model:value="form.sentFolder"
                        show-search
                        option-filter-prop="label"
                        :options="sentFolderOptions"
                        :filter-option="filterFolderOption"
                        placeholder="请选择发件箱目录"
                      />
                      <a-button :disabled="!editingId" :loading="folderLoading" @click="loadAccountFolders(editingId, true)">
                        读取目录
                      </a-button>
                    </div>
                    <div class="form-help">
                      用于同步服务器上的已发送邮件。通常保持自动识别结果即可。
                    </div>
                  </a-form-item>
                </details>
              </section>
            </a-tab-pane>

            <a-tab-pane key="smtp" tab="发信 SMTP">
              <section class="account-form-section">
                <div class="account-form-grid">
                  <a-form-item label="SMTP 主机" name="smtpHost">
                    <a-input v-model:value="form.smtpHost" placeholder="smtp.example.com" />
                  </a-form-item>
                  <a-form-item label="SMTP 端口" name="smtpPort">
                    <a-input-number v-model:value="form.smtpPort" :min="1" :max="65535" style="width: 100%" />
                  </a-form-item>
                  <a-form-item label="SMTP 登录用户名" name="smtpUsername">
                    <a-input v-model:value="form.smtpUsername" placeholder="留空默认使用邮箱地址" />
                  </a-form-item>
                  <a-form-item label="SMTP 密码或授权码" name="smtpPassword">
                    <a-input-password
                      v-model:value="form.smtpPassword"
                      :disabled="form.smtpUseImapPassword"
                      :placeholder="editingId ? '留空则不修改' : ''"
                    />
                  </a-form-item>
                </div>
                <div class="account-toggle-row">
                  <a-checkbox v-model:checked="form.smtpUseImapPassword">发信使用同一密码或授权码</a-checkbox>
                  <a-checkbox v-model:checked="form.smtpStartTls" :disabled="form.smtpTls">使用 STARTTLS</a-checkbox>
                  <a-checkbox v-model:checked="form.smtpTls" @change="onSmtpTlsChanged">使用 SSL/TLS</a-checkbox>
                </div>
              </section>
            </a-tab-pane>

            <a-tab-pane key="signature" tab="签名">
              <section class="account-form-section">
                <a-form-item label="签名模板" name="signatureHtml">
                  <a-textarea
                    v-model:value="form.signatureHtml"
                    :auto-size="{ minRows: 8, maxRows: 14 }"
                    placeholder="<p>姓名</p><p>公司 / 电话</p>"
                  />
                  <div class="form-help">
                    支持 HTML；写邮件选择该账号时会自动插入，也可以在写信工具栏点击签名按钮插入。
                  </div>
                </a-form-item>
                <div class="signature-preview">
                  <div class="signature-preview-title">预览</div>
                  <div v-if="form.signatureHtml.trim()" class="signature-preview-body" v-html="form.signatureHtml"></div>
                  <div v-else class="signature-preview-empty">暂无签名内容</div>
                </div>
              </section>
            </a-tab-pane>

            <a-tab-pane key="sync" tab="同步">
              <section class="account-form-section">
                <div class="account-form-grid compact-grid">
                  <a-form-item label="收取间隔（分钟）" name="pollIntervalMinutes">
                    <a-input-number v-model:value="form.pollIntervalMinutes" :min="1" :max="1440" style="width: 100%" />
                  </a-form-item>
                  <div class="account-toggle-row account-enable-row">
                    <a-checkbox v-model:checked="form.enabled">启用账号</a-checkbox>
                  </div>
                </div>
                <div class="danger-settings">
                  <div class="danger-settings-title">服务器旧邮件清理</div>
                  <div class="cleanup-grid">
                    <a-checkbox v-model:checked="form.cleanupEnabled">全量同步成功后删除服务器旧邮件</a-checkbox>
                    <a-form-item label="保留天数" name="cleanupRetentionDays">
                      <a-input-number
                        v-model:value="form.cleanupRetentionDays"
                        :disabled="!form.cleanupEnabled"
                        :min="1"
                        :max="3650"
                        style="width: 100%"
                      />
                    </a-form-item>
                  </div>
                  <a-alert
                    v-if="form.cleanupEnabled"
                    type="warning"
                    show-icon
                    message="开启后，仅在全量同步成功后，删除已保存到本地且早于保留天数的服务器 INBOX 邮件。"
                  />
                </div>
              </section>
            </a-tab-pane>
          </a-tabs>

          <div class="account-form-actions">
            <a-button @click="modalOpen = false">取消</a-button>
            <a-button type="primary" html-type="submit" :loading="saving">保存</a-button>
          </div>
        </a-form>
      </a-modal>

      <a-modal v-model:open="syncDetailOpen" title="同步日志" :footer="null">
        <a-descriptions v-if="syncDetailAccount" bordered size="small" :column="1">
          <a-descriptions-item label="邮箱">{{ syncDetailAccount.email }}</a-descriptions-item>
          <a-descriptions-item label="状态">{{ fullSyncStatusText(syncDetailAccount.fullSyncStatus) }}</a-descriptions-item>
          <a-descriptions-item label="进度">
            {{ syncDetailAccount.fullSyncProcessed }}/{{ syncDetailAccount.fullSyncTotal || 0 }} 封
          </a-descriptions-item>
          <a-descriptions-item label="新增">{{ syncDetailAccount.fullSyncNewCount || 0 }} 封</a-descriptions-item>
          <a-descriptions-item label="开始时间">{{ formatTime(syncDetailAccount.fullSyncStartedAt) }}</a-descriptions-item>
          <a-descriptions-item label="结束时间">{{ formatTime(syncDetailAccount.fullSyncFinishedAt) }}</a-descriptions-item>
          <a-descriptions-item label="说明">{{ syncDetailAccount.fullSyncError || '暂无错误' }}</a-descriptions-item>
        </a-descriptions>
      </a-modal>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { message, Modal, type TableColumnsType } from 'ant-design-vue';
import { DownOutlined } from '@ant-design/icons-vue';
import type { MenuInfo } from 'ant-design-vue/es/menu/src/interface';
import {
  mailAccountApi,
  oauthApi,
  type CreateMailAccountPayload,
  type MailAccount,
  type MailAccountFolder,
} from '../api/client';
import AppLayout from '../components/AppLayout.vue';

const loading = ref(false);
const saving = ref(false);
const testingId = ref('');
const syncingId = ref('');
const folderLoading = ref(false);
const fullSyncingId = ref('');
const stoppingId = ref('');
const oauthLoading = ref(false);
const modalOpen = ref(false);
const editingId = ref('');
const accountFormTab = ref('basic');
const accounts = ref<MailAccount[]>([]);
const accountFolders = ref<MailAccountFolder[]>([]);
const pollErrors = reactive<Record<string, string>>({});
const pollingIds = new Set<string>();
const syncDetailOpen = ref(false);
const syncDetailAccount = ref<MailAccount | null>(null);
let statusTimer: number | undefined;
const columns: TableColumnsType<MailAccount> = [
  { title: '名称', dataIndex: 'displayName', key: 'displayName' },
  { title: '邮箱', dataIndex: 'email', key: 'email' },
  { title: 'IMAP 主机', dataIndex: 'imapHost', key: 'imapHost' },
  { title: 'SMTP 主机', dataIndex: 'smtpHost', key: 'smtpHost' },
  { title: '端口', dataIndex: 'imapPort', key: 'imapPort', width: 90 },
  { title: '认证', key: 'authType', width: 90 },
  { title: '状态', key: 'enabled', width: 90 },
  { title: '同步状态', key: 'syncStatus', width: 170 },
  { title: '操作', key: 'action', width: 250 },
];
const form = reactive<CreateMailAccountPayload>({
  displayName: '',
  email: '',
  imapHost: '',
  imapPort: 993,
  imapTls: true,
  imapUsername: '',
  imapPassword: '',
  smtpHost: '',
  smtpPort: 587,
  smtpTls: false,
  smtpStartTls: true,
  smtpUsername: '',
  smtpPassword: '',
  smtpUseImapPassword: true,
  sentFolder: 'Sent',
  signatureHtml: '',
  pollIntervalMinutes: 10,
  enabled: true,
  cleanupEnabled: false,
  cleanupRetentionDays: 90,
});
const modalTitle = computed(() => editingId.value ? '编辑邮箱账号' : '新增邮箱账号');
const passwordRules = computed(() => editingId.value ? [] : [{ required: true, message: '请输入邮箱密码或授权码' }]);
const defaultFolderOptions: MailAccountFolder[] = [
  { name: 'Sent', delimiter: '/', attributes: ['\\Sent'], sentCandidate: true },
  { name: 'Sent Messages', delimiter: '/', attributes: ['\\Sent'], sentCandidate: true },
  { name: 'Sent Items', delimiter: '/', attributes: ['\\Sent'], sentCandidate: true },
  { name: '已发送邮件', delimiter: '/', attributes: ['\\Sent'], sentCandidate: true },
];
const sentFolderOptions = computed(() => {
  const merged = new Map<string, MailAccountFolder>();
  for (const folder of defaultFolderOptions) {
    merged.set(folder.name, folder);
  }
  for (const folder of accountFolders.value) {
    merged.set(folder.name, folder);
  }
  if (form.sentFolder && !merged.has(form.sentFolder)) {
    merged.set(form.sentFolder, { name: form.sentFolder, delimiter: '/', attributes: [], sentCandidate: false });
  }
  return Array.from(merged.values())
    .sort((left, right) => Number(right.sentCandidate) - Number(left.sentCandidate) || left.name.localeCompare(right.name, 'zh-CN'))
    .map((folder) => ({
      value: folder.name,
      label: folder.sentCandidate ? `${folder.name}（可能是发件箱）` : folder.name,
    }));
});

onMounted(async () => {
  await loadAccounts();
  statusTimer = window.setInterval(refreshRunningAccounts, 3000);
});

onBeforeUnmount(() => {
  if (statusTimer) {
    window.clearInterval(statusTimer);
  }
});

async function loadAccounts() {
  loading.value = true;
  try {
    const data = await mailAccountApi.list();
    accounts.value = data.items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取邮箱账号失败');
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = '';
  accountFormTab.value = 'basic';
  accountFolders.value = [];
  Object.assign(form, {
    displayName: '',
    email: '',
    imapHost: '',
    imapPort: 993,
    imapTls: true,
    imapUsername: '',
    imapPassword: '',
    smtpHost: '',
    smtpPort: 587,
    smtpTls: false,
    smtpStartTls: true,
    smtpUsername: '',
    smtpPassword: '',
    smtpUseImapPassword: true,
    sentFolder: 'Sent',
    signatureHtml: '',
    pollIntervalMinutes: 10,
    enabled: true,
    cleanupEnabled: false,
    cleanupRetentionDays: 90,
  });
  modalOpen.value = true;
}

function openEdit(account: MailAccount) {
  editingId.value = account.id;
  accountFormTab.value = 'basic';
  accountFolders.value = [];
  Object.assign(form, {
    displayName: account.displayName,
    email: account.email,
    imapHost: account.imapHost,
    imapPort: account.imapPort,
    imapTls: account.imapTls,
    imapUsername: account.imapUsername,
    imapPassword: '',
    smtpHost: account.smtpHost || '',
    smtpPort: account.smtpPort || 587,
    smtpTls: account.smtpTls,
    smtpStartTls: account.smtpStartTls,
    smtpUsername: account.smtpUsername || '',
    smtpPassword: '',
    smtpUseImapPassword: false,
    sentFolder: account.sentFolder || 'Sent',
    signatureHtml: account.signatureHtml || '',
    pollIntervalMinutes: account.pollIntervalMinutes,
    enabled: account.enabled,
    cleanupEnabled: account.cleanupEnabled,
    cleanupRetentionDays: account.cleanupRetentionDays || 90,
  });
  modalOpen.value = true;
  void loadAccountFolders(account.id, false);
}

async function saveAccount() {
  saving.value = true;
  try {
    if (editingId.value) {
      await mailAccountApi.update(editingId.value, form);
      message.success('邮箱账号已更新');
    } else {
      await mailAccountApi.create(form);
      message.success('邮箱账号已创建');
    }
    modalOpen.value = false;
    await loadAccounts();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存邮箱账号失败');
  } finally {
    saving.value = false;
  }
}

async function removeAccount(id: string) {
  try {
    await mailAccountApi.remove(id);
    message.success('邮箱账号已删除');
    await loadAccounts();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除邮箱账号失败');
  }
}

function confirmRemoveAccount(id: string) {
  Modal.confirm({
    title: '删除邮箱账号',
    content: '确定删除这个邮箱账号？本地已收取的邮件数据也会失去账号入口。',
    okText: '删除',
    cancelText: '取消',
    okButtonProps: { danger: true },
    onOk: () => removeAccount(id),
  });
}

async function handleAccountAction(account: MailAccount, info: MenuInfo) {
  switch (info.key) {
    case 'edit':
      openEdit(account);
      break;
    case 'test':
      await testConnection(account.id);
      break;
    case 'sync-log':
      openSyncDetail(account);
      break;
    case 'delete':
      confirmRemoveAccount(account.id);
      break;
  }
}

function handleAccountMenuClick(account: MailAccount, info: MenuInfo) {
  void handleAccountAction(account, info);
}

async function testConnection(id: string) {
  testingId.value = id;
  try {
    await mailAccountApi.testConnection(id);
    message.success('连接成功');
    await loadAccounts();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '连接失败');
  } finally {
    testingId.value = '';
  }
}

async function loadAccountFolders(id: string, showToast: boolean) {
  if (!id) {
    return;
  }
  folderLoading.value = true;
  try {
    const data = await mailAccountApi.folders(id);
    accountFolders.value = data.items;
    const candidate = data.items.find((folder) => folder.sentCandidate);
    if (candidate && (!form.sentFolder || form.sentFolder === 'Sent')) {
      form.sentFolder = candidate.name;
    }
    if (showToast) {
      message.success(`已读取 ${data.items.length} 个邮箱目录`);
    }
  } catch (error) {
    if (showToast) {
      message.error(error instanceof Error ? error.message : '读取邮箱目录失败');
    }
  } finally {
    folderLoading.value = false;
  }
}

function filterFolderOption(input: string, option?: { label?: string; value?: string }) {
  const keyword = input.toLowerCase();
  return `${option?.label || ''} ${option?.value || ''}`.toLowerCase().includes(keyword);
}

function onSmtpTlsChanged() {
  if (form.smtpTls) {
    form.smtpStartTls = false;
    if (form.smtpPort === 587) {
      form.smtpPort = 465;
    }
    return;
  }
  form.smtpStartTls = true;
  if (form.smtpPort === 465) {
    form.smtpPort = 587;
  }
}

async function syncAccount(id: string) {
  syncingId.value = id;
  try {
    const result = await mailAccountApi.sync(id);
    if (result.warnings?.length) {
      message.warning(`收取完成，新增 ${result.newMessageCount} 封邮件；${result.warnings.join('；')}`);
    } else {
      message.success(`收取完成，新增 ${result.newMessageCount} 封邮件`);
    }
    await loadAccounts();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '收取失败');
  } finally {
    syncingId.value = '';
  }
}

async function startFullSync(id: string) {
  fullSyncingId.value = id;
  try {
    await mailAccountApi.startFullSync(id);
    message.success('已开始同步全部历史邮件');
    await refreshAccountSyncStatus(id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '启动全量同步失败');
  } finally {
    fullSyncingId.value = '';
  }
}

async function stopFullSync(id: string) {
  stoppingId.value = id;
  try {
    const status = await mailAccountApi.stopFullSync(id);
    mergeAccountSyncStatus(id, status);
    message.success('已停止全量同步');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '停止全量同步失败');
  } finally {
    stoppingId.value = '';
  }
}

async function refreshRunningAccounts() {
  const runningIds = accounts.value
    .filter((account) => account.fullSyncStatus === 'running')
    .map((account) => account.id);
  await Promise.all(runningIds.map((id) => refreshAccountSyncStatus(id)));
}

async function refreshAccountSyncStatus(id: string) {
  if (pollingIds.has(id)) {
    return;
  }
  pollingIds.add(id);
  try {
    const status = await mailAccountApi.syncStatus(id);
    mergeAccountSyncStatus(id, status);
    delete pollErrors[id];
  } catch (error) {
    pollErrors[id] = error instanceof Error ? error.message : '同步状态刷新失败';
  } finally {
    pollingIds.delete(id);
  }
}

function mergeAccountSyncStatus(id: string, status: Awaited<ReturnType<typeof mailAccountApi.syncStatus>>) {
  accounts.value = accounts.value.map((account) => (
    account.id === id
      ? {
          ...account,
          fullSyncStatus: status.fullSyncStatus,
          fullSyncTotal: status.fullSyncTotal,
          fullSyncProcessed: status.fullSyncProcessed,
          fullSyncNewCount: status.fullSyncNewCount,
          fullSyncStartedAt: status.fullSyncStartedAt,
          fullSyncFinishedAt: status.fullSyncFinishedAt,
          fullSyncError: status.fullSyncError,
          cleanupEnabled: status.cleanupEnabled,
          cleanupRetentionDays: status.cleanupRetentionDays,
        }
      : account
  ));
  if (syncDetailAccount.value?.id === id) {
    syncDetailAccount.value = accounts.value.find((account) => account.id === id) || null;
  }
}

function fullSyncPercent(account: MailAccount) {
  if (!account.fullSyncTotal) {
    return account.fullSyncStatus === 'success' ? 100 : 0;
  }
  return Math.min(100, Math.round((account.fullSyncProcessed / account.fullSyncTotal) * 100));
}

function fullSyncStatusText(status: MailAccount['fullSyncStatus']) {
  const texts: Record<MailAccount['fullSyncStatus'], string> = {
    idle: '未同步',
    running: '同步中',
    success: '已完成',
    failed: '失败',
    cancelled: '已停止',
  };
  return texts[status] || '未同步';
}

function fullSyncTagColor(status: MailAccount['fullSyncStatus']) {
  const colors: Record<MailAccount['fullSyncStatus'], string> = {
    idle: 'default',
    running: 'processing',
    success: 'green',
    failed: 'red',
    cancelled: 'orange',
  };
  return colors[status] || 'default';
}

function fullSyncHint(account: MailAccount) {
  if (account.fullSyncStatus === 'running') {
    return `${account.fullSyncProcessed}/${account.fullSyncTotal || 0} 封`;
  }
  if (account.fullSyncStatus === 'success') {
    return `新增 ${account.fullSyncNewCount || 0} 封`;
  }
  if (account.fullSyncStatus === 'failed') {
    return account.fullSyncError || '同步失败';
  }
  if (account.fullSyncStatus === 'cancelled') {
    return account.fullSyncError || '已停止';
  }
  return account.cleanupEnabled ? `清理保留 ${account.cleanupRetentionDays} 天` : '清理关闭';
}

function openSyncDetail(account: MailAccount) {
  syncDetailAccount.value = account;
  syncDetailOpen.value = true;
}

function formatTime(value: string | null) {
  if (!value) {
    return '暂无';
  }
  return new Date(value).toLocaleString('zh-CN');
}

async function startMicrosoftOAuth() {
  oauthLoading.value = true;
  try {
    const data = await oauthApi.startMicrosoft();
    window.location.href = data.authUrl;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '创建 Microsoft 授权链接失败');
    oauthLoading.value = false;
  }
}
</script>

<style scoped>
.account-sync-cell {
  display: grid;
  gap: 5px;
  min-width: 130px;
}

.account-sync-hint {
  color: #6b7280;
  font-size: 12px;
  line-height: 18px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-sync-error {
  color: #dc2626;
  font-size: 12px;
  line-height: 18px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.form-help {
  margin-top: 6px;
  color: #6b7280;
  font-size: 12px;
  line-height: 18px;
}

.account-actions {
  display: flex;
  width: max-content;
  max-width: 100%;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}

.account-modal :deep(.ant-modal-body) {
  max-height: calc(100vh - 170px);
  padding-top: 10px;
  overflow-y: auto;
}

.account-form {
  display: grid;
  gap: 0;
}

.account-form-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 18px;
}

.account-form-tabs :deep(.ant-tabs-tab) {
  padding: 10px 0;
}

.account-form-tabs :deep(.ant-tabs-content-holder) {
  min-height: 250px;
}

.account-form-section {
  display: grid;
  gap: 14px;
}

.account-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 16px;
}

.account-form-grid :deep(.ant-form-item),
.account-form-wide,
.danger-settings :deep(.ant-form-item) {
  margin-bottom: 0;
}

.compact-grid {
  grid-template-columns: minmax(180px, 260px) minmax(0, 1fr);
  align-items: end;
}

.folder-picker {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 110px;
  gap: 10px;
  align-items: center;
}

.folder-picker :deep(.ant-select) {
  width: 100%;
  min-width: 0;
}

.folder-picker :deep(.ant-btn) {
  width: 110px;
}

.account-toggle-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 22px;
  min-height: 32px;
}

.advanced-settings {
  padding: 12px 14px;
  border: 1px solid #e5ebf3;
  border-radius: 8px;
  background: #f8fafc;
}

.advanced-settings summary {
  color: #334155;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  line-height: 20px;
}

.advanced-settings[open] summary {
  margin-bottom: 12px;
}

.account-enable-row {
  padding-bottom: 1px;
}

.danger-settings {
  display: grid;
  gap: 12px;
  margin-top: 2px;
  padding: 14px;
  border: 1px solid #fde3cf;
  border-radius: 8px;
  background: #fff7ed;
}

.danger-settings-title {
  color: #9a3412;
  font-size: 14px;
  font-weight: 600;
}

.signature-preview {
  display: grid;
  gap: 8px;
  padding: 12px 14px;
  border: 1px solid #e5ebf3;
  border-radius: 8px;
  background: #f8fafc;
}

.signature-preview-title {
  color: #334155;
  font-size: 13px;
  font-weight: 700;
}

.signature-preview-body {
  min-height: 70px;
  padding: 12px;
  border: 1px solid #edf1f7;
  border-radius: 8px;
  background: #ffffff;
  color: #1f2329;
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.signature-preview-body :deep(img) {
  max-width: 100%;
  height: auto;
}

.signature-preview-empty {
  padding: 14px 12px;
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  color: #8a96a8;
  background: #ffffff;
}

.cleanup-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 160px;
  align-items: end;
  gap: 14px;
}

.account-form-actions {
  position: sticky;
  bottom: -10px;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 0 4px;
  border-top: 1px solid #edf1f7;
  background: #ffffff;
}

@media (max-width: 760px) {
  .account-form-grid,
  .compact-grid,
  .cleanup-grid {
    grid-template-columns: 1fr;
  }

  .folder-picker {
    grid-template-columns: 1fr;
  }

  .folder-picker :deep(.ant-btn) {
    width: 100%;
  }
}
</style>
