<template>
  <a-drawer :open="open" title="联系人分组" width="min(760px, 100vw)" @close="emit('update:open', false)">
    <div class="groups-toolbar">
      <a-input-search v-model:value="keyword" allow-clear placeholder="搜索分组名称" @search="reload" @change="searchChanged" />
      <a-button type="primary" @click="edit()"><template #icon><plus-outlined /></template>新建分组</a-button>
    </div>
    <a-alert v-if="error" :message="error" type="error" show-icon class="groups-error" />
    <a-table row-key="id" :data-source="groups" :columns="columns" :loading="loading" :scroll="{ x: 320 }"
      :pagination="{ current: page, pageSize: 20, total, showSizeChanger: false }" @change="pageChanged">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'name'"><strong class="group-name">{{ record.name }}</strong><div class="group-description">{{ record.description }}</div></template>
        <template v-else-if="column.key === 'actions'">
          <a-space>
            <a-tooltip title="编辑分组与成员"><a-button type="text" aria-label="编辑分组与成员" @click="edit(record.id)"><template #icon><edit-outlined /></template></a-button></a-tooltip>
            <a-tooltip title="删除分组"><a-button type="text" danger aria-label="删除分组" @click="remove(record)"><template #icon><delete-outlined /></template></a-button></a-tooltip>
          </a-space>
        </template>
      </template>
    </a-table>
    <ContactGroupEditor v-model:open="editorOpen" :group-id="editingId" @saved="saved" />
  </a-drawer>
</template>
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';
import { Modal } from 'ant-design-vue';
import type { TablePaginationConfig } from 'ant-design-vue';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons-vue';
import { contactGroupApi } from '../api';
import type { ContactGroup } from '../types';
import ContactGroupEditor from './ContactGroupEditor.vue';
const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ 'update:open': [value: boolean]; changed: [] }>();
const groups = ref<ContactGroup[]>([]); const page = ref(1); const total = ref(0); const keyword = ref('');
const loading = ref(false); const error = ref(''); const editorOpen = ref(false); const editingId = ref<string>();
let controller: AbortController | undefined; let timer: ReturnType<typeof setTimeout> | undefined;
const columns = [{ title: '分组', key: 'name' }, { title: '成员', dataIndex: 'memberCount', width: 70 }, { title: '操作', key: 'actions', width: 100 }];
watch(() => props.open, open => { if (open) reload(); else { controller?.abort(); editorOpen.value = false; clearTimeout(timer); } });
async function load() {
  controller?.abort(); const current = new AbortController(); controller = current; loading.value = true; error.value = '';
  try {
    const data = await contactGroupApi.list({ keyword: keyword.value, page: page.value, pageSize: 20 }, current.signal);
    if (!current.signal.aborted) { groups.value = data.items; total.value = data.total; }
  } catch (e) { if (!current.signal.aborted) error.value = e instanceof Error ? e.message : '读取分组失败'; }
  finally { if (controller === current) loading.value = false; }
}
function reload() { page.value = 1; void load(); }
function pageChanged(p: TablePaginationConfig) { page.value = p.current || 1; void load(); }
function searchChanged() { clearTimeout(timer); controller?.abort(); timer = setTimeout(reload, 250); }
function edit(id?: string) { editingId.value = id; editorOpen.value = true; }
function saved() { reload(); emit('changed'); }
function remove(group: ContactGroup) {
  Modal.confirm({ title: `删除分组「${group.name}」？`, content: '仅删除分组及成员关系，联系人和邮件不会删除。', okText: '删除分组', cancelText: '取消', okType: 'danger',
    async onOk() {
      try { await contactGroupApi.remove(group.id); saved(); }
      catch (e) { error.value = e instanceof Error ? e.message : '删除失败'; throw e; }
    },
  });
}
onBeforeUnmount(() => { controller?.abort(); clearTimeout(timer); });
</script>
<style scoped>
.groups-toolbar { display: flex; gap: 12px; margin-bottom: 16px; }
.groups-toolbar > .ant-input-search { min-width: 0; flex: 1; }
.groups-toolbar > .ant-btn { flex: none; }
.group-description { color: var(--muted-color); font-size: 12px; overflow-wrap: anywhere; }
.group-name { overflow-wrap: anywhere; }
.groups-error { margin-bottom: 12px; }
</style>
