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
          <a-select v-model:value="selected" mode="multiple" show-search placement="topLeft" :options="options" :filter-option="false"
            :loading="searching" :max-tag-count="5" allow-clear placeholder="搜索姓名或邮箱添加成员" @search="search">
            <template #notFoundContent>{{ searchError || (searching ? '搜索中…' : '暂无匹配联系人') }}</template>
          </a-select>
        </a-form-item>
      </a-form>
    </a-spin>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { contactApi } from '../../../api/client';
import { contactGroupApi } from '../api';
import type { GroupMember } from '../types';
const props = defineProps<{ open: boolean; groupId?: string }>();
const emit = defineEmits<{ 'update:open': [value: boolean]; saved: [] }>();
const name = ref(''); const description = ref(''); const revision = ref(0);
const selected = ref<string[]>([]); const members = ref<GroupMember[]>([]);
const suggestions = ref<GroupMember[]>([]); const loading = ref(false); const searching = ref(false); const saving = ref(false);
const error = ref(''); const loadError = ref(false); const searchError = ref('');
let loadController: AbortController | undefined; let searchController: AbortController | undefined;
let timer: ReturnType<typeof setTimeout> | undefined;
const options = computed(() => [...new Map([...members.value, ...suggestions.value].map(c => [c.id, c])).values()]
  .map(c => ({ value: c.id, label: `${c.nickname || c.displayName || c.name || c.email} <${c.email}>` })));
watch(() => [props.open, props.groupId] as const, async ([open]) => {
  loadController?.abort(); searchController?.abort(); clearTimeout(timer);
  if (!open) return;
  name.value = ''; description.value = ''; selected.value = []; members.value = []; suggestions.value = []; error.value = ''; loadError.value = false;
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
<style scoped>.group-error { margin-bottom: 16px; }</style>
