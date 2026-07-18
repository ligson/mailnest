<template>
  <AppLayout selected-key="/contacts">
    <section class="content-panel contacts-page">
      <div class="page-toolbar">
        <div>
          <h2 class="page-title">联系人</h2>
          <p class="page-subtitle">维护常用联系人，邮件列表和详情会优先显示昵称或姓名。</p>
        </div>
        <a-button type="primary" @click="openCreate">
          <template #icon><plus-outlined /></template>
          新增联系人
        </a-button>
      </div>

      <div class="contacts-toolbar">
        <a-input-search
          v-model:value="keyword"
          allow-clear
          placeholder="搜索姓名、邮箱、电话、公司或备注"
          @search="loadContacts"
          @change="onKeywordChanged"
        />
      </div>

      <a-table
        row-key="id"
        :columns="columns"
        :data-source="contacts"
        :loading="loading"
        :pagination="pagination"
        @change="onTableChanged"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <div class="contact-name-cell">
              <strong>{{ preferredName(record) }}</strong>
              <span>{{ record.email }}</span>
            </div>
          </template>
          <template v-else-if="column.key === 'source'">
            <a-tag :color="record.source === 'manual' ? 'blue' : 'green'">
              {{ record.source === 'manual' ? '手动维护' : '邮件发现' }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'lastSeenAt'">
            {{ formatTime(record.lastSeenAt) }}
          </template>
          <template v-else-if="column.key === 'actions'">
            <a-space>
              <a-button size="small" @click="openEdit(record)">
                <template #icon><edit-outlined /></template>
                编辑
              </a-button>
              <a-button size="small" danger @click="deleteContact(record)">
                <template #icon><delete-outlined /></template>
                删除
              </a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </section>

    <a-modal
      v-model:open="modalOpen"
      :title="editingId ? '编辑联系人' : '新增联系人'"
      ok-text="保存"
      cancel-text="取消"
      :confirm-loading="saving"
      destroy-on-close
      @ok="submitContact"
    >
      <a-form ref="formRef" layout="vertical" :model="form" :rules="rules">
        <a-form-item label="邮箱地址" name="email">
          <a-input v-model:value="form.email" placeholder="name@example.com" />
        </a-form-item>
        <a-form-item label="昵称" name="nickname">
          <a-input v-model:value="form.nickname" placeholder="邮件中优先显示的名称" :maxlength="80" />
        </a-form-item>
        <a-form-item label="姓名" name="displayName">
          <a-input v-model:value="form.displayName" placeholder="联系人真实姓名或显示名" :maxlength="80" />
        </a-form-item>
        <div class="contact-form-grid">
          <a-form-item label="电话" name="phone">
            <a-input v-model:value="form.phone" :maxlength="40" />
          </a-form-item>
          <a-form-item label="公司" name="company">
            <a-input v-model:value="form.company" :maxlength="120" />
          </a-form-item>
        </div>
        <a-form-item label="备注" name="notes">
          <a-textarea v-model:value="form.notes" :rows="4" :maxlength="500" show-count />
        </a-form-item>
      </a-form>
    </a-modal>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons-vue';
import { Modal, message, type FormInstance } from 'ant-design-vue';
import type { TablePaginationConfig } from 'ant-design-vue';
import { useRoute } from 'vue-router';
import { contactApi, type Contact, type ContactPayload } from '../api/client';
import AppLayout from '../components/AppLayout.vue';

const route = useRoute();
const contacts = ref<Contact[]>([]);
const loading = ref(false);
const saving = ref(false);
const keyword = ref('');
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const modalOpen = ref(false);
const editingId = ref<string | null>(null);
const formRef = ref<FormInstance>();
const form = reactive<ContactPayload>({
  email: '',
  displayName: '',
  nickname: '',
  phone: '',
  company: '',
  notes: '',
});

const columns = [
  { title: '联系人', key: 'name', width: 280 },
  { title: '电话', dataIndex: 'phone', key: 'phone', width: 150 },
  { title: '公司', dataIndex: 'company', key: 'company', width: 180 },
  { title: '来源', key: 'source', width: 110 },
  { title: '最近出现', key: 'lastSeenAt', width: 170 },
  { title: '操作', key: 'actions', width: 160, fixed: 'right' as const },
];
const pagination = computed<TablePaginationConfig>(() => ({
  current: page.value,
  pageSize: pageSize.value,
  total: total.value,
  showSizeChanger: true,
  showTotal: (value) => `共 ${value} 位联系人`,
}));
const rules = {
  email: [
    { required: true, message: '请输入邮箱地址' },
    { type: 'email' as const, message: '邮箱格式不正确' },
  ],
  nickname: [{ max: 80, message: '昵称不能超过 80 个字符' }],
  displayName: [{ max: 80, message: '姓名不能超过 80 个字符' }],
  phone: [{ max: 40, message: '电话不能超过 40 个字符' }],
  company: [{ max: 120, message: '公司不能超过 120 个字符' }],
  notes: [{ max: 500, message: '备注不能超过 500 个字符' }],
};

onMounted(async () => {
  await loadContacts();
  await openContactFromRoute();
});

watch(() => route.query.email, () => {
  void openContactFromRoute();
});

async function loadContacts() {
  loading.value = true;
  try {
    const data = await contactApi.list({
      keyword: keyword.value.trim() || undefined,
      page: page.value,
      pageSize: pageSize.value,
    });
    contacts.value = data.items;
    total.value = data.total;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取联系人失败');
  } finally {
    loading.value = false;
  }
}

function onKeywordChanged() {
  page.value = 1;
  void loadContacts();
}

function onTableChanged(nextPagination: TablePaginationConfig) {
  page.value = nextPagination.current || 1;
  pageSize.value = nextPagination.pageSize || 20;
  void loadContacts();
}

function openCreate() {
  editingId.value = null;
  resetForm();
  modalOpen.value = true;
}

function openCreateWithEmail(email: string, displayName: string) {
  editingId.value = null;
  resetForm();
  form.email = email;
  form.displayName = displayName;
  modalOpen.value = true;
}

function openEdit(contact: Contact) {
  editingId.value = contact.id;
  form.email = contact.email;
  form.displayName = contact.displayName || '';
  form.nickname = contact.nickname || '';
  form.phone = contact.phone || '';
  form.company = contact.company || '';
  form.notes = contact.notes || '';
  modalOpen.value = true;
}

async function openContactFromRoute() {
  const email = queryValue(route.query.email).trim();
  if (!email) {
    return;
  }
  const displayName = queryValue(route.query.displayName).trim();
  try {
    keyword.value = email;
    page.value = 1;
    const data = await contactApi.list({ keyword: email, page: 1, pageSize: 20 });
    contacts.value = data.items;
    total.value = data.total;
    const matched = data.items.find((item) => item.email.toLowerCase() === email.toLowerCase());
    if (matched) {
      openEdit(matched);
      return;
    }
    openCreateWithEmail(email, displayName);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '打开联系人失败');
  }
}

function resetForm() {
  form.email = '';
  form.displayName = '';
  form.nickname = '';
  form.phone = '';
  form.company = '';
  form.notes = '';
  formRef.value?.clearValidate();
}

async function submitContact() {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  saving.value = true;
  try {
    const payload = {
      email: form.email.trim(),
      displayName: form.displayName.trim(),
      nickname: form.nickname.trim(),
      phone: form.phone.trim(),
      company: form.company.trim(),
      notes: form.notes.trim(),
    };
    if (editingId.value) {
      await contactApi.update(editingId.value, payload);
      message.success('联系人已更新');
    } else {
      await contactApi.create(payload);
      message.success('联系人已创建');
    }
    modalOpen.value = false;
    await loadContacts();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存联系人失败');
  } finally {
    saving.value = false;
  }
}

function deleteContact(contact: Contact) {
  Modal.confirm({
    title: `删除联系人「${preferredName(contact)}」？`,
    content: '删除后不会影响已经保存的邮件，只是不再用该联系人信息优化显示。',
    okText: '删除',
    okType: 'danger',
    cancelText: '取消',
    async onOk() {
      await contactApi.remove(contact.id);
      message.success('联系人已删除');
      await loadContacts();
    },
  });
}

function preferredName(contact: Contact) {
  return contact.nickname || contact.displayName || contact.email;
}

function formatTime(value: string | null) {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString();
}

function queryValue(value: unknown) {
  if (Array.isArray(value)) {
    return String(value[0] || '');
  }
  return typeof value === 'string' ? value : '';
}
</script>

<style scoped>
.contacts-page {
  overflow: auto;
}

.contacts-toolbar {
  max-width: 520px;
  margin-bottom: 16px;
}

.contact-name-cell {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.contact-name-cell strong,
.contact-name-cell span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.contact-name-cell span {
  color: #64748b;
  font-size: 12px;
}

.contact-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

@media (max-width: 720px) {
  .contact-form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
