<template>
  <a-modal :open="open" :title="groupId ? '编辑联系人分组' : '新建联系人分组'" ok-text="保存" cancel-text="取消"
    :confirm-loading="saving" :ok-button-props="{ disabled: loading || !!loadError }" :mask-closable="false"
    @cancel="emit('update:open', false)" @ok="save">
    <a-alert v-if="error" :message="error" type="error" show-icon class="group-error" />
    <a-spin :spinning="loading">
      <a-form layout="vertical" @submit.prevent>
        <a-form-item label="分组名称" required><a-input v-model:value="name" :maxlength="80" placeholder="例如：项目组、客户、家人" /></a-form-item>
        <a-form-item label="备注"><a-textarea v-model:value="description" :rows="2" :maxlength="500" /></a-form-item>
        <a-form-item :label="`分组成员（${selected.length} 人）`">
          <a-input-search v-model:value="memberKeyword" allow-clear placeholder="搜索姓名、昵称或邮箱" @search="search" @change="memberFilterChanged" />
          <a-table
            class="group-member-table"
            row-key="id"
            size="small"
            :columns="memberColumns"
            :data-source="visibleMembers"
            :loading="searching"
            :row-selection="memberSelection"
            :pagination="{ pageSize: 8, showSizeChanger: false, size: 'small' }"
            :scroll="{ y: 250 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'"><strong>{{ record.nickname || record.displayName || record.name || '未命名联系人' }}</strong></template>
              <template v-else-if="column.key === 'email'"><span class="member-email">{{ record.email }}</span></template>
            </template>
          </a-table>
          <div v-if="searchError" class="member-error">{{ searchError }}</div>
        </a-form-item>
      </a-form>
    </a-spin>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import type { Key } from 'ant-design-vue/es/_util/type';
import type { TableColumnsType } from 'ant-design-vue';
import { contactApi } from '../../../api/client';
import { contactGroupApi } from '../api';
import type { GroupMember } from '../types';
const props = defineProps<{ open: boolean; groupId?: string }>();
const emit = defineEmits<{ 'update:open': [value: boolean]; saved: [] }>();
const name = ref(''); const description = ref(''); const revision = ref(0); const memberKeyword = ref('');
const selected = ref<string[]>([]); const members = ref<GroupMember[]>([]);
const suggestions = ref<GroupMember[]>([]); const loading = ref(false); const searching = ref(false); const saving = ref(false);
const error = ref(''); const loadError = ref(false); const searchError = ref('');
let loadController: AbortController | undefined; let searchController: AbortController | undefined;
let timer: ReturnType<typeof setTimeout> | undefined;
const memberColumns: TableColumnsType<GroupMember> = [
  { title: '联系人', key: 'name', width: 170 },
  { title: '邮箱地址', key: 'email' },
];
const allMembers = computed(() => [...new Map([...members.value, ...suggestions.value].map(c => [c.id, c])).values()]);
const visibleMembers = computed(() => {
  const keyword = memberKeyword.value.trim().toLowerCase();
  if (!keyword) return allMembers.value;
  return allMembers.value.filter(c => [c.name, c.nickname, c.displayName, c.email].some(value => value.toLowerCase().includes(keyword)));
});
const memberSelection = computed(() => ({
  selectedRowKeys: selected.value,
  preserveSelectedRowKeys: true,
  onChange: (keys: Key[]) => { selected.value = keys.map(String); },
}));
watch(() => [props.open, props.groupId] as const, async ([open]) => {
  loadController?.abort(); searchController?.abort(); clearTimeout(timer);
  if (!open) return;
  name.value = ''; description.value = ''; selected.value = []; members.value = []; suggestions.value = []; memberKeyword.value = ''; error.value = ''; loadError.value = false;
  const controller = new AbortController(); loadController = controller;
  loading.value = true;
  try {
    if (props.groupId) {
      const data = await contactGroupApi.detail(props.groupId, controller.signal);
      if (controller.signal.aborted) return;
      name.value = data.group.name; description.value = data.group.description; revision.value = data.group.revision;
      members.value = data.members; selected.value = data.members.map(c => c.id);
    }
    search('');
  } catch (e) { if (!controller.signal.aborted) { error.value = e instanceof Error ? e.message : '读取分组失败'; loadError.value = true; } }
  finally { if (loadController === controller) loading.value = false; }
}, { immediate: true });
function search(keyword: string) {
  clearTimeout(timer); searchController?.abort(); const controller = new AbortController(); searchController = controller;
  searching.value = true; searchError.value = '';
  timer = setTimeout(async () => {
    try {
      const data = await contactApi.list({ keyword, pageSize: 50 }, { signal: controller.signal });
      if (!controller.signal.aborted) {
        // 保留已选选项，连续搜索不会把已选昵称退化成纯 ID。
        members.value = [...new Map([...members.value, ...suggestions.value.filter(c => selected.value.includes(c.id))].map(c => [c.id, c])).values()];
        suggestions.value = data.items.map(c => ({ ...c, nickname: c.nickname || '', displayName: c.displayName || '' }));
      }
    } catch (e) { if (!controller.signal.aborted) searchError.value = e instanceof Error ? e.message : '搜索联系人失败'; }
    finally { if (searchController === controller) searching.value = false; }
  }, 250);
}
function memberFilterChanged() {
  if (memberKeyword.value.trim()) search(memberKeyword.value.trim());
}
async function save() {
  if (saving.value || loading.value || loadError.value) return;
  if (!name.value.trim()) { error.value = '请输入分组名称'; return; }
  saving.value = true; error.value = '';
  try {
    const payload = { name: name.value.trim(), description: description.value.trim(), contactIds: selected.value, revision: revision.value };
    if (props.groupId) await contactGroupApi.update(props.groupId, payload); else await contactGroupApi.create(payload);
    emit('saved'); emit('update:open', false);
  } catch (e) { error.value = e instanceof Error ? e.message : '保存分组失败'; }
  finally { saving.value = false; }
}
onBeforeUnmount(() => { loadController?.abort(); searchController?.abort(); clearTimeout(timer); });
</script>
<style scoped>
.group-error { margin-bottom: 16px; }
.group-member-table { margin-top: 10px; }
.member-email { color: var(--muted-color); overflow-wrap: anywhere; }
.member-error { margin-top: 6px; color: var(--error-color, #d4380d); font-size: 12px; }
</style>
