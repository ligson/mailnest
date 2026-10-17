<template>
  <a-select
    :value="value" :options="options" :loading="loading" :filter-option="false"
    show-search allow-clear :placeholder="placeholder || '全部分组'"
    @change="change" @search="search" @dropdown-visible-change="visible" @popup-scroll="onScroll"
  >
    <template #notFoundContent><span>{{ error || (loading ? '加载中…' : '暂无分组') }}</span></template>
  </a-select>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue';
import { contactGroupApi } from '../api';
import type { ContactGroup } from '../types';
const props = defineProps<{ value?: string; placeholder?: string; refreshKey?: number }>();
const emit = defineEmits<{ 'update:value': [value: string | undefined]; change: [value: string | undefined] }>();
const groups = ref<ContactGroup[]>([]);
const loading = ref(false);
const error = ref('');
const keyword = ref('');
const page = ref(0);
const total = ref(0);
let loadedKey: number | undefined = -1;
let controller: AbortController | undefined;
let timer: ReturnType<typeof setTimeout> | undefined;
const options = computed(() => groups.value.map(g => ({ value: g.id, label: `${g.name}（${g.memberCount} 人）` })));
function change(value?: string) { emit('update:value', value); emit('change', value); }
async function load(reset = false) {
  if (reset) { controller?.abort(); page.value = 0; }
  else if (loading.value) return;
  const current = new AbortController(); controller = current;
  loading.value = true; error.value = '';
  try {
    const data = await contactGroupApi.list({ keyword: keyword.value || undefined, page: page.value + 1, pageSize: 50 }, current.signal);
    if (current.signal.aborted) return;
    groups.value = reset ? data.items : [...groups.value, ...data.items];
    page.value = data.page; total.value = data.total; loadedKey = props.refreshKey;
  } catch (e) { if (!current.signal.aborted) error.value = e instanceof Error ? e.message : '加载分组失败'; }
  finally { if (controller === current) loading.value = false; }
}
function visible(open: boolean) { if (open && (!page.value || loadedKey !== props.refreshKey || error.value)) void load(true); }
function search(value: string) { keyword.value = value; clearTimeout(timer); controller?.abort(); timer = setTimeout(() => void load(true), 250); }
function onScroll(e: Event) {
  const el = e.target as HTMLElement;
  if (el.scrollHeight - el.scrollTop - el.clientHeight < 40 && groups.value.length < total.value) void load();
}
onBeforeUnmount(() => { controller?.abort(); clearTimeout(timer); });
</script>
