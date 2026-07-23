<template>
  <AppLayout selected-key="/rules">
    <section class="content-panel">
      <div class="page-toolbar">
        <div>
          <h2 class="page-title">规则</h2>
          <p class="page-subtitle">按发件人、域名、关键词、正文和附件条件自动归档或移动到垃圾邮件</p>
        </div>
        <div class="toolbar-actions">
          <a-select v-model:value="applyScope" class="apply-scope-select">
            <a-select-option value="unfiled">仅未归档</a-select-option>
            <a-select-option value="filtered">当前筛选</a-select-option>
            <a-select-option value="all">全部邮件</a-select-option>
          </a-select>
          <a-button @click="applyRules">应用到历史邮件</a-button>
          <a-button @click="openSpamTemplate">垃圾规则模板</a-button>
          <a-button type="primary" @click="openCreate">新增规则</a-button>
        </div>
      </div>

      <a-table row-key="id" :columns="columns" :data-source="rules" :loading="loading" :pagination="false">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'enabled'">
            <a-tag :color="record.enabled ? 'green' : 'default'">{{ record.enabled ? '启用' : '停用' }}</a-tag>
          </template>
          <template v-if="column.key === 'targetFolderId'">
            {{ actionLabel(record) }}
          </template>
          <template v-if="column.key === 'conditions'">
            <div class="condition-tags">
              <a-tag v-for="condition in record.conditions" :key="`${condition.field}-${condition.operator}-${condition.value}`">
                {{ conditionLabel(condition) }}
              </a-tag>
            </div>
          </template>
          <template v-if="column.key === 'hits'">
            <div class="rule-hit-cell">
              <strong>{{ record.hitCount || 0 }}</strong>
              <span>{{ record.lastHitAt ? formatShortTime(record.lastHitAt) : '暂无命中' }}</span>
            </div>
          </template>
          <template v-if="column.key === 'actions'">
            <a-space>
              <a-button type="link" size="small" @click="openRuleLogs(record)">记录</a-button>
              <a-button type="link" size="small" @click="openEdit(record)">编辑</a-button>
              <a-button type="link" danger size="small" @click="deleteRule(record)">删除</a-button>
            </a-space>
          </template>
        </template>
      </a-table>

      <a-drawer v-model:open="drawerOpen" width="560" :title="drawerTitle">
        <a-form layout="vertical">
          <a-form-item label="规则名称">
            <a-input v-model:value="form.name" placeholder="例如：安全通知归档" />
          </a-form-item>
          <a-form-item v-if="form.actionType === 'move_folder'" label="目标文件夹">
            <a-select v-model:value="form.targetFolderId" placeholder="选择文件夹">
              <a-select-option v-for="folder in folders" :key="folder.id" :value="folder.id">
                {{ folder.name }}
              </a-select-option>
            </a-select>
          </a-form-item>
          <a-row :gutter="12">
            <a-col :span="12">
              <a-form-item label="优先级">
                <a-input-number v-model:value="form.priority" :min="0" :max="999" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item label="匹配后停止">
                <a-switch v-model:checked="form.stopOnMatch" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item label="动作">
            <a-radio-group v-model:value="form.actionType">
              <a-radio value="move_folder">移动文件夹</a-radio>
              <a-radio value="mark_read">标记已读</a-radio>
              <a-radio value="star">加星标</a-radio>
              <a-radio value="mark_spam">移动到垃圾邮件</a-radio>
            </a-radio-group>
          </a-form-item>
          <a-form-item label="匹配方式">
            <a-radio-group v-model:value="form.matchMode">
              <a-radio value="all">全部条件满足</a-radio>
              <a-radio value="any">任一条件满足</a-radio>
            </a-radio-group>
          </a-form-item>
          <a-form-item label="启用">
            <a-switch v-model:checked="form.enabled" />
          </a-form-item>

          <div class="rule-condition-header">
            <strong>条件</strong>
            <a-space>
              <a-button size="small" @click="previewRule">预览</a-button>
              <a-button size="small" @click="addCondition">添加条件</a-button>
            </a-space>
          </div>
          <div v-for="(condition, index) in form.conditions" :key="index" class="condition-row">
            <a-select v-model:value="condition.field" class="condition-field">
              <a-select-option value="from">发件人</a-select-option>
              <a-select-option value="blacklist_sender">黑名单发件人</a-select-option>
              <a-select-option value="blacklist_domain">黑名单域名</a-select-option>
              <a-select-option value="spam_keyword">垃圾关键词</a-select-option>
              <a-select-option value="subject">主题</a-select-option>
              <a-select-option value="body">正文</a-select-option>
              <a-select-option value="has_attachments">是否有附件</a-select-option>
              <a-select-option value="is_read">是否已读</a-select-option>
              <a-select-option value="starred">是否星标</a-select-option>
              <a-select-option value="attachment_filename">附件名</a-select-option>
              <a-select-option value="attachment_content_type">附件类型</a-select-option>
              <a-select-option value="attachment_size">附件大小</a-select-option>
            </a-select>
            <a-select v-model:value="condition.operator" class="condition-operator">
              <a-select-option value="contains">包含</a-select-option>
              <a-select-option value="equals">等于</a-select-option>
              <a-select-option value="is_true">为真</a-select-option>
              <a-select-option value="is_false">为假</a-select-option>
              <a-select-option value="greater_than">大于</a-select-option>
              <a-select-option value="less_than">小于</a-select-option>
            </a-select>
            <a-input v-model:value="condition.value" class="condition-value" :placeholder="conditionPlaceholder(condition)" />
            <a-button danger @click="removeCondition(index)">删除</a-button>
          </div>
        </a-form>
        <template #footer>
          <a-space>
            <a-button @click="drawerOpen = false">取消</a-button>
            <a-button type="primary" :loading="saving" @click="saveRule">保存</a-button>
          </a-space>
        </template>
      </a-drawer>

      <a-drawer v-model:open="previewOpen" width="720" title="规则预览">
        <a-descriptions bordered size="small" :column="1">
          <a-descriptions-item label="命中">{{ preview.matchedCount }}</a-descriptions-item>
        </a-descriptions>
        <a-list class="preview-list" :data-source="preview.samples" size="small">
          <template #renderItem="{ item }">
            <a-list-item>{{ item.subject || '无主题' }} · {{ item.from || '-' }}</a-list-item>
          </template>
        </a-list>
      </a-drawer>

      <a-drawer v-model:open="logOpen" width="720" title="规则命中记录">
        <a-spin :spinning="logLoading">
          <a-empty v-if="ruleLogs.length === 0" description="暂无规则命中记录" />
          <a-list v-else class="rule-log-list" :data-source="ruleLogs" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <div class="rule-log-row">
                  <div>
                    <strong>{{ item.messageSubject || '无主题' }}</strong>
                    <p>{{ item.resultMessage || '-' }}</p>
                  </div>
                  <div class="rule-log-side">
                    <a-tag :color="ruleLogColor(item.resultStatus)">{{ ruleLogStatusLabel(item.resultStatus) }}</a-tag>
                    <span>{{ formatShortTime(item.createdAt) }}</span>
                  </div>
                </div>
              </a-list-item>
            </template>
          </a-list>
        </a-spin>
      </a-drawer>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { Modal, message, type TableColumnsType } from 'ant-design-vue';
import {
  mailFolderApi,
  mailRuleApi,
  ruleLogApi,
  type MailFolder,
  type MailRule,
  type MailRuleCondition,
  type MailRuleLog,
} from '../api/client';
import AppLayout from '../components/AppLayout.vue';

const loading = ref(false);
const saving = ref(false);
const drawerOpen = ref(false);
const editingId = ref('');
const previewOpen = ref(false);
const folders = ref<MailFolder[]>([]);
const rules = ref<MailRule[]>([]);
const ruleLogs = ref<MailRuleLog[]>([]);
const logOpen = ref(false);
const logLoading = ref(false);
const applyScope = ref<'unfiled' | 'filtered' | 'all'>('unfiled');
const preview = reactive<{ matchedCount: number; samples: Array<{ id: string; subject: string | null; from: string | null }> }>({
  matchedCount: 0,
  samples: [],
});
const form = reactive({
  name: '',
  enabled: true,
  matchMode: 'all' as 'all' | 'any',
  priority: 100,
  stopOnMatch: true,
  actionType: 'move_folder',
  targetFolderId: undefined as string | undefined,
  sortOrder: 10,
  conditions: [] as MailRuleCondition[],
});
const drawerTitle = computed(() => editingId.value ? '编辑规则' : '新增规则');
const columns: TableColumnsType<MailRule> = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '状态', key: 'enabled', width: 90 },
  { title: '动作', key: 'targetFolderId', width: 160 },
  { title: '条件', key: 'conditions' },
  { title: '命中', key: 'hits', width: 120 },
  { title: '排序', dataIndex: 'sortOrder', key: 'sortOrder', width: 80 },
  { title: '操作', key: 'actions', width: 160 },
];

onMounted(refresh);

async function refresh() {
  loading.value = true;
  try {
    const [folderData, ruleData] = await Promise.all([mailFolderApi.list(), mailRuleApi.list()]);
    folders.value = folderData.items;
    rules.value = ruleData.items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取规则失败');
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = '';
  form.name = '';
  form.enabled = true;
  form.matchMode = 'all';
  form.priority = 100;
  form.stopOnMatch = true;
  form.actionType = 'move_folder';
  form.targetFolderId = folders.value[0]?.id;
  form.sortOrder = rules.value.length * 10 + 10;
  form.conditions = [{ field: 'subject', operator: 'contains', value: '' }];
  drawerOpen.value = true;
}

function openSpamTemplate() {
  editingId.value = '';
  form.name = '垃圾邮件黑名单';
  form.enabled = true;
  form.matchMode = 'any';
  form.priority = 10;
  form.stopOnMatch = true;
  form.actionType = 'mark_spam';
  form.targetFolderId = undefined;
  form.sortOrder = rules.value.length * 10 + 10;
  form.conditions = [
    { field: 'blacklist_sender', operator: 'equals', value: '' },
    { field: 'blacklist_domain', operator: 'equals', value: '' },
    { field: 'spam_keyword', operator: 'contains', value: '' },
  ];
  drawerOpen.value = true;
}

function openEdit(rule: MailRule) {
  editingId.value = rule.id;
  form.name = rule.name;
  form.enabled = rule.enabled;
  form.matchMode = rule.matchMode;
  form.priority = rule.priority || 100;
  form.stopOnMatch = rule.stopOnMatch;
  form.actionType = rule.actionType || 'move_folder';
  form.targetFolderId = rule.targetFolderId || undefined;
  form.sortOrder = rule.sortOrder;
  form.conditions = rule.conditions.map((item) => ({
    field: item.field,
    operator: item.operator,
    value: item.value,
  }));
  drawerOpen.value = true;
}

function addCondition() {
  form.conditions.push({ field: 'subject', operator: 'contains', value: '' });
}

function removeCondition(index: number) {
  form.conditions.splice(index, 1);
}

async function saveRule() {
  if (!form.name.trim() || (form.actionType === 'move_folder' && !form.targetFolderId) || form.conditions.length === 0) {
    message.warning('请填写规则名称、条件，以及移动规则的目标文件夹');
    return;
  }
  saving.value = true;
  try {
    const payload = {
      name: form.name.trim(),
      enabled: form.enabled,
      matchMode: form.matchMode,
      priority: form.priority,
      stopOnMatch: form.stopOnMatch,
      actionType: form.actionType,
      targetFolderId: form.actionType === 'move_folder' ? form.targetFolderId || null : null,
      sortOrder: form.sortOrder,
      conditions: form.conditions.map((item) => ({
        field: item.field,
        operator: item.operator,
        value: item.value || '',
      })),
    };
    if (editingId.value) {
      await mailRuleApi.update(editingId.value, payload);
      message.success('规则已更新');
    } else {
      await mailRuleApi.create(payload);
      message.success('规则已创建');
    }
    drawerOpen.value = false;
    await refresh();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存规则失败');
  } finally {
    saving.value = false;
  }
}

async function applyRules() {
  try {
    const result = await mailRuleApi.apply({ scope: applyScope.value });
    message.success(`已归档 ${result.appliedCount} 封邮件`);
    await refresh();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '应用规则失败');
  }
}

async function openRuleLogs(rule: MailRule) {
  logOpen.value = true;
  logLoading.value = true;
  try {
    ruleLogs.value = (await ruleLogApi.list({ ruleId: rule.id, pageSize: 50 })).items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取规则记录失败');
  } finally {
    logLoading.value = false;
  }
}

async function previewRule() {
  if (!form.name.trim() || (form.actionType === 'move_folder' && !form.targetFolderId) || form.conditions.length === 0) {
    message.warning('请先补全规则基础信息');
    return;
  }
  try {
    const data = await mailRuleApi.preview({
      name: form.name.trim(),
      enabled: form.enabled,
      matchMode: form.matchMode,
      priority: form.priority,
      stopOnMatch: form.stopOnMatch,
      actionType: form.actionType,
      targetFolderId: form.actionType === 'move_folder' ? form.targetFolderId || null : null,
      sortOrder: form.sortOrder,
      conditions: form.conditions,
      limit: 10,
    });
    preview.matchedCount = data.matchedCount;
    preview.samples = data.samples;
    previewOpen.value = true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '规则预览失败');
  }
}

function deleteRule(rule: MailRule) {
  Modal.confirm({
    title: `删除规则「${rule.name}」？`,
    content: '删除后不会影响已经归档的邮件，但后续不会再按该规则自动归档。',
    okText: '删除',
    okType: 'danger',
    cancelText: '取消',
    async onOk() {
      await mailRuleApi.remove(rule.id);
      await refresh();
      message.success('规则已删除');
    },
  });
}

function folderName(id: string) {
  return folders.value.find((folder) => folder.id === id)?.name || '-';
}

function actionLabel(rule: MailRule) {
  switch (rule.actionType) {
    case 'mark_read':
      return '标记已读';
    case 'star':
      return '加星标';
    case 'mark_spam':
      return '移动到垃圾邮件';
    default:
      return `移动到 ${folderName(rule.targetFolderId || '')}`;
  }
}

function formatShortTime(value: string | null) {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}

function ruleLogColor(status: string) {
  switch (status) {
    case 'failed':
      return 'red';
    case 'skipped':
      return 'orange';
    default:
      return 'green';
  }
}

function ruleLogStatusLabel(status: string) {
  switch (status) {
    case 'failed':
      return '失败';
    case 'skipped':
      return '跳过';
    default:
      return '已执行';
  }
}

function conditionLabel(condition: MailRuleCondition) {
  const fieldMap: Record<string, string> = {
    from: '发件人',
    blacklist_sender: '黑名单发件人',
    blacklist_domain: '黑名单域名',
    spam_keyword: '垃圾关键词',
    subject: '主题',
    body: '正文',
    has_attachments: '附件',
    is_read: '已读',
    starred: '星标',
    attachment_filename: '附件名',
    attachment_content_type: '附件类型',
    attachment_size: '附件大小',
  };
  const operatorMap: Record<string, string> = {
    contains: '包含',
    equals: '等于',
    is_true: '为真',
    is_false: '为假',
    greater_than: '大于',
    less_than: '小于',
  };
  return `${fieldMap[condition.field] || condition.field} ${operatorMap[condition.operator] || condition.operator} ${condition.value}`;
}

function conditionPlaceholder(condition: MailRuleCondition) {
  switch (condition.field) {
    case 'blacklist_sender':
      return 'bad@example.com, cheat@example.com';
    case 'blacklist_domain':
      return 'promo.example.com, ads.example.net';
    case 'spam_keyword':
      return '优惠, 返利, 免费提现';
    default:
      return '匹配值';
  }
}
</script>

<style scoped>
.toolbar-actions {
  display: flex;
  gap: 8px;
}

.apply-scope-select {
  min-width: 120px;
}

.condition-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.rule-hit-cell {
  display: grid;
  gap: 2px;
}

.rule-hit-cell span {
  color: var(--muted-color);
  font-size: 12px;
}

.rule-log-list :deep(.ant-list-item) {
  padding-right: 0;
  padding-left: 0;
}

.rule-log-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  width: 100%;
}

.rule-log-row strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-log-row p {
  margin: 4px 0 0;
  color: var(--muted-color);
}

.rule-log-side {
  display: grid;
  justify-items: end;
  gap: 4px;
  color: var(--muted-color);
  font-size: 12px;
}

.rule-condition-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 10px 0;
}

.condition-row {
  display: grid;
  grid-template-columns: 110px 110px 1fr 64px;
  gap: 8px;
  margin-bottom: 8px;
}

.condition-field,
.condition-operator,
.condition-value {
  min-width: 0;
}

.preview-list {
  margin-top: 12px;
}
</style>
