<template>
  <a-modal :open="open" :title="kind === 'company' ? '批量设置公司' : '批量设置分组'"
    ok-text="应用设置" cancel-text="取消" destroy-on-close :mask-closable="false" :closable="!saving" :keyboard="!saving"
    :confirm-loading="saving" :cancel-button-props="{ disabled: saving }"
    :ok-button-props="{ disabled: !canSubmit }" @cancel="emit('update:open', false)" @ok="submit">
    <p class="batch-summary">已选择 <strong>{{ targetIds.length }}</strong> 位联系人</p>
    <a-alert v-if="limitError" :message="limitError" type="warning" show-icon class="batch-notice" />
    <a-alert v-if="error" :message="error" type="error" show-icon class="batch-notice" />
    <a-form layout="vertical" :disabled="saving">
      <template v-if="kind === 'company'">
        <a-form-item label="操作方式">
          <a-radio-group v-model:value="companyMode" class="batch-modes">
            <a-radio value="fill_empty">仅补全空公司</a-radio>
            <a-radio value="set">统一设置公司</a-radio>
            <a-radio value="clear">清空公司</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item v-if="companyMode !== 'clear'" label="公司名称" required>
          <a-input v-model:value="company" :maxlength="120" placeholder="输入公司名称" allow-clear />
        </a-form-item>
        <a-alert :type="companyMode === 'fill_empty' ? 'info' : 'warning'" show-icon class="batch-notice"
          :message="companyMode === 'fill_empty' ? '已有公司信息保持不变，只补全未填写的联系人。' : companyMode === 'set' ? '所选联系人的原公司信息会被覆盖，姓名、昵称和其他资料保持不变。' : '所选联系人的公司信息将被清空，其他资料保持不变。'" />
        <a-checkbox v-if="companyMode === 'clear'" v-model:checked="clearConfirmed">确认清空所选联系人的公司信息</a-checkbox>
      </template>
      <template v-else>
        <a-form-item label="操作方式">
          <a-radio-group v-model:value="groupMode">
            <a-radio value="add">加入分组</a-radio>
            <a-radio value="remove">移出分组</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item label="联系人分组" required>
          <ContactGroupSelect v-model:values="groupIds" multiple :refresh-key="refreshKey" placeholder="选择一个或多个分组" style="width: 100%" />
        </a-form-item>
        <a-alert type="info" show-icon class="batch-notice"
          :message="groupMode === 'add' ? '加入所选分组，保留已有分组；重复加入会自动去重。' : '仅移出所选分组，联系人及其在其他分组中的关系保留。'" />
      </template>
    </a-form>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { batchUpdateContacts, type CompanyBatchMode, type ContactBatchPayload, type ContactBatchResult, type GroupBatchMode } from '../batch';
import ContactGroupSelect from './ContactGroupSelect.vue';

const props = defineProps<{ open: boolean; kind: 'company' | 'groups'; contactIds: string[]; refreshKey: number }>();
const emit = defineEmits<{ 'update:open': [value: boolean]; saved: [result: ContactBatchResult] }>();
const targetIds = ref<string[]>([]);
const companyMode = ref<CompanyBatchMode>('fill_empty');
const groupMode = ref<GroupBatchMode>('add');
const company = ref('');
const groupIds = ref<string[]>([]);
const clearConfirmed = ref(false);
const saving = ref(false);
const error = ref('');
watch(() => props.open, open => {
  if (!open) return;
  // 固定本次操作的选中快照；列表分页或刷新不能悄悄改变提交范围。
  targetIds.value = [...props.contactIds];
  companyMode.value = 'fill_empty'; groupMode.value = 'add';
  company.value = ''; groupIds.value = []; clearConfirmed.value = false; error.value = '';
});
watch(companyMode, () => { clearConfirmed.value = false; });
const limitError = computed(() => targetIds.value.length > 10000 ? '单次最多设置 10000 位联系人，请减少选择后重试。' :
  props.kind === 'groups' && (groupIds.value.length > 100 || groupIds.value.length * targetIds.value.length > 50000) ? '单次最多选择 100 个分组，联系人与分组组合最多 50000 对，请分次操作。' : '');
const canSubmit = computed(() => !saving.value && !limitError.value && targetIds.value.length > 0 &&
  (props.kind === 'company' ? companyMode.value === 'clear' ? clearConfirmed.value : !!company.value.trim() : groupIds.value.length > 0));
async function submit() {
  if (!canSubmit.value) return;
  const payload: ContactBatchPayload = { contactIds: [...targetIds.value] };
  if (props.kind === 'company') payload.company = { mode: companyMode.value, value: companyMode.value === 'clear' ? '' : company.value.trim() };
  else payload.groups = { mode: groupMode.value, groupIds: [...groupIds.value] };
  saving.value = true; error.value = '';
  try {
    const result = await batchUpdateContacts(payload);
    emit('saved', result); emit('update:open', false);
  } catch (e) { error.value = e instanceof Error ? e.message : '批量设置失败，请重试'; }
  finally { saving.value = false; }
}
</script>

<style scoped>
.batch-summary { margin: 0 0 20px; color: var(--muted-color); }
.batch-summary strong { color: var(--text-color); }
.batch-notice { margin-bottom: 16px; }
.batch-modes { display: flex; flex-wrap: wrap; gap: 10px 0; }
</style>
