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
          <template v-if="column.key === 'authType'">
            <a-tag :color="record.authType === 'oauth2' ? 'blue' : 'default'">
              {{ record.authType === 'oauth2' ? 'OAuth2' : '密码' }}
            </a-tag>
          </template>
          <template v-if="column.key === 'action'">
            <a-space>
              <a-button size="small" @click="openEdit(record)">编辑</a-button>
              <a-button size="small" :loading="testingId === record.id" @click="testConnection(record.id)">测试</a-button>
              <a-button size="small" type="primary" :loading="syncingId === record.id" @click="syncAccount(record.id)">收取</a-button>
              <a-popconfirm title="确定删除这个邮箱账号？" ok-text="删除" cancel-text="取消" @confirm="removeAccount(record.id)">
                <a-button danger size="small">删除</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>

      <a-modal v-model:open="modalOpen" :title="modalTitle" :footer="null" destroy-on-close>
        <a-form layout="vertical" :model="form" @finish="saveAccount">
          <a-form-item label="显示名称" name="displayName" :rules="[{ required: true, message: '请输入显示名称' }]">
            <a-input v-model:value="form.displayName" />
          </a-form-item>
          <a-form-item label="邮箱地址" name="email" :rules="[{ required: true, type: 'email', message: '请输入有效邮箱' }]">
            <a-input v-model:value="form.email" />
          </a-form-item>
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
          <a-form-item label="收取间隔（分钟）" name="pollIntervalMinutes">
            <a-input-number v-model:value="form.pollIntervalMinutes" :min="1" :max="1440" style="width: 100%" />
          </a-form-item>
          <a-form-item>
            <a-checkbox v-model:checked="form.imapTls">使用 TLS</a-checkbox>
          </a-form-item>
          <a-form-item>
            <a-checkbox v-model:checked="form.enabled">启用账号</a-checkbox>
          </a-form-item>
          <a-space>
            <a-button type="primary" html-type="submit" :loading="saving">保存</a-button>
            <a-button @click="modalOpen = false">取消</a-button>
          </a-space>
        </a-form>
      </a-modal>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { message, type TableColumnsType } from 'ant-design-vue';
import { mailAccountApi, oauthApi, type CreateMailAccountPayload, type MailAccount } from '../api/client';
import AppLayout from '../components/AppLayout.vue';

const loading = ref(false);
const saving = ref(false);
const testingId = ref('');
const syncingId = ref('');
const oauthLoading = ref(false);
const modalOpen = ref(false);
const editingId = ref('');
const accounts = ref<MailAccount[]>([]);
const columns: TableColumnsType<MailAccount> = [
  { title: '名称', dataIndex: 'displayName', key: 'displayName' },
  { title: '邮箱', dataIndex: 'email', key: 'email' },
  { title: 'IMAP 主机', dataIndex: 'imapHost', key: 'imapHost' },
  { title: '端口', dataIndex: 'imapPort', key: 'imapPort', width: 90 },
  { title: '认证', key: 'authType', width: 90 },
  { title: '状态', key: 'enabled', width: 90 },
  { title: '操作', key: 'action', width: 270 },
];
const form = reactive<CreateMailAccountPayload>({
  displayName: '',
  email: '',
  imapHost: '',
  imapPort: 993,
  imapTls: true,
  imapUsername: '',
  imapPassword: '',
  pollIntervalMinutes: 10,
  enabled: true,
});
const modalTitle = computed(() => editingId.value ? '编辑邮箱账号' : '新增邮箱账号');
const passwordRules = computed(() => editingId.value ? [] : [{ required: true, message: '请输入邮箱密码或授权码' }]);

onMounted(loadAccounts);

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
  Object.assign(form, {
    displayName: '',
    email: '',
    imapHost: '',
    imapPort: 993,
    imapTls: true,
    imapUsername: '',
    imapPassword: '',
    pollIntervalMinutes: 10,
    enabled: true,
  });
  modalOpen.value = true;
}

function openEdit(account: MailAccount) {
  editingId.value = account.id;
  Object.assign(form, {
    displayName: account.displayName,
    email: account.email,
    imapHost: account.imapHost,
    imapPort: account.imapPort,
    imapTls: account.imapTls,
    imapUsername: account.imapUsername,
    imapPassword: '',
    pollIntervalMinutes: account.pollIntervalMinutes,
    enabled: account.enabled,
  });
  modalOpen.value = true;
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

async function syncAccount(id: string) {
  syncingId.value = id;
  try {
    const result = await mailAccountApi.sync(id);
    message.success(`收取完成，新增 ${result.newMessageCount} 封邮件`);
    await loadAccounts();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '收取失败');
  } finally {
    syncingId.value = '';
  }
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
