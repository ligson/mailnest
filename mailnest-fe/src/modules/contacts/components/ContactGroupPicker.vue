<template>
  <a-modal :open="open" :title="title || '选择联系人分组'" :width="620" :mask-closable="false"
    :ok-text="confirmText || '添加联系人'" cancel-text="取消" :confirm-loading="confirming"
    :ok-button-props="{ disabled: !selected.length || !!previewError || previewLoading || (preview && !members.length) }"
    @cancel="emit('update:open', false)" @ok="confirm">
    <a-alert v-if="error" :message="error" type="error" show-icon class="picker-alert" />
    <a-input-search v-model:value="keyword" allow-clear placeholder="搜索分组名称" @search="reload" @change="searchChanged" />
    <a-table row-key="id" size="small" :columns="columns" :data-source="groups" :loading="loading" :scroll="{ y: 260 }"
      :row-selection="rowSelection" :pagination="{ current: page, pageSize: 10, total, showSizeChanger: false, size: 'small' }"
      @change="pageChanged" class="picker-table" />
    <div class="picker-summary">
      <strong>已选 {{ selected.length }} 个分组<span v-if="preview"> · {{ members.length }} 位联系人</span></strong>
      <a-spin v-if="previewLoading" size="small" />
    </div>
    <a-alert v-if="previewError" :message="previewError" type="error" show-icon />
    <a-alert v-else-if="preview && selected.length && !previewLoading && !members.length" message="所选分组暂无联系人" type="warning" show-icon />
    <div v-else-if="preview" class="member-preview">
      <a-tooltip v-for="member in members.slice(0, 8)" :key="member.id" :title="member.email"><a-tag>{{ member.name }}</a-tag></a-tooltip>
      <span v-if="members.length > 8" class="remaining">另 {{ members.length - 8 }} 人</span>
    </div>
  </a-modal>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import type { Key } from 'ant-design-vue/es/_util/type';
import type { TablePaginationConfig } from 'ant-design-vue';
import { contactGroupApi } from '../api';
import type { ContactGroup, GroupMember } from '../types';
const props = withDefaults(defineProps<{ open: boolean; title?: string; confirmText?: string; preview?: boolean }>(), { preview: true });
const emit = defineEmits<{ 'update:open': [value: boolean]; selected: [members: GroupMember[], groupIds: string[]] }>();
const groups = ref<ContactGroup[]>([]); const selected = ref<string[]>([]); const members = ref<GroupMember[]>([]);
const page = ref(1); const total = ref(0); const keyword = ref(''); const error = ref(''); const loading = ref(false);
const previewLoading = ref(false); const previewError = ref(''); const confirming = ref(false);
let controller: AbortController | undefined; let previewController: AbortController | undefined;
let timer: ReturnType<typeof setTimeout> | undefined;
const columns = [{ title: '分组名称', dataIndex: 'name', ellipsis: true }, { title: '成员', dataIndex: 'memberCount', width: 75 }];
const rowSelection = computed(() => ({ selectedRowKeys: selected.value, preserveSelectedRowKeys: true,
  onChange: (keys: Key[]) => { selected.value = keys.map(String); },
}));
watch(() => props.open, open => {
  controller?.abort(); previewController?.abort(); clearTimeout(timer);
  if (open) { selected.value = []; members.value = []; keyword.value = ''; previewError.value = ''; error.value = ''; reload(); }
}, { immediate: true });
watch(selected, async ids => {
  previewController?.abort(); members.value = []; previewError.value = '';
  if (!props.open || !ids.length) { previewLoading.value = false; return; }
  const current = new AbortController(); previewController = current; previewLoading.value = true;
  try { const data = await contactGroupApi.resolve(ids, current.signal); if (!current.signal.aborted) members.value = data.items; }
  catch (e) { if (!current.signal.aborted) previewError.value = e instanceof Error ? e.message : '读取分组成员失败'; }
  finally { if (previewController === current) previewLoading.value = false; }
});
async function load() {
  controller?.abort(); const current = new AbortController(); controller = current; loading.value = true; error.value = '';
  try { const data = await contactGroupApi.list({ keyword: keyword.value, page: page.value, pageSize: 10 }, current.signal);
    if (!current.signal.aborted) { groups.value = data.items; total.value = data.total; }
  } catch (e) { if (!current.signal.aborted) error.value = e instanceof Error ? e.message : '读取分组失败'; }
  finally { if (controller === current) loading.value = false; }
}
function reload() { page.value = 1; void load(); }
function pageChanged(p: TablePaginationConfig) { page.value = p.current || 1; void load(); }
function searchChanged() { clearTimeout(timer); controller?.abort(); timer = setTimeout(reload, 250); }
async function confirm() {
  if (confirming.value || !selected.value.length) return;
  confirming.value = true; error.value = '';
  const current = new AbortController(); previewController?.abort(); previewController = current;
  try {
    // 点击确认时再取当前成员，不能用上次预览缓存悄悄发送给已移出分组的人。
    const data = await contactGroupApi.resolve(selected.value, current.signal);
    if (current.signal.aborted || !props.open) return;
    if (props.preview && !data.items.length) { error.value = '所选分组暂无联系人，请先维护成员'; return; }
    emit('selected', data.items, [...selected.value]); emit('update:open', false);
  } catch (e) { if (!current.signal.aborted) error.value = e instanceof Error ? e.message : '获取分组成员失败'; }
  finally { confirming.value = false; }
}
onBeforeUnmount(() => { controller?.abort(); previewController?.abort(); clearTimeout(timer); });
</script>
<style scoped>
.picker-alert { margin-bottom: 12px; }
.picker-table { margin-top: 12px; }
.picker-summary { display: flex; align-items: center; gap: 12px; margin: 12px 0; }
.member-preview { display: flex; flex-wrap: wrap; gap: 6px 0; max-height: 90px; overflow: auto; }
.member-preview .ant-tag { max-width: 180px; overflow: hidden; text-overflow: ellipsis; }
.remaining { color: var(--muted-color); font-size: 12px; }
</style>
