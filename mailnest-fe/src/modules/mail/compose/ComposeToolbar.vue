<template>
<div class="compose-toolbar">
  <div class="compose-toolbar-group" @mousedown="saveComposeSelection">
    <a-tooltip title="添加附件">
      <a-button type="text" class="compose-tool-button" aria-label="添加附件" @mousedown.prevent @click="chooseComposeFiles">
        <template #icon><paper-clip-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="插入图片">
      <a-button type="text" class="compose-tool-button" aria-label="插入图片" @mousedown.prevent @click="chooseComposeImages">
        <template #icon><file-image-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="插入签名">
      <a-button type="text" class="compose-tool-button" aria-label="插入签名" @mousedown.prevent @click="insertComposeSignature">
        <template #icon><edit-outlined /></template>
      </a-button>
    </a-tooltip>
  </div>
  <span class="compose-toolbar-divider"></span>
  <div class="compose-toolbar-group compose-toolbar-selects" @mousedown="saveComposeSelection">
    <a-select
      :value="composeFontFamily"
      class="compose-font-select"
      size="small"
      @change="applyComposeFontFamily"
    >
      <template #suffixIcon><font-size-outlined /></template>
      <a-select-option v-for="font in composeFontFamilies" :key="font.value" :value="font.value">
        {{ font.label }}
      </a-select-option>
    </a-select>
    <a-select
      :value="composeFontSize"
      class="compose-size-select"
      size="small"
      @change="applyComposeFontSize"
    >
      <a-select-option v-for="size in composeFontSizes" :key="size.value" :value="size.value">
        {{ size.label }}
      </a-select-option>
    </a-select>
  </div>
  <span class="compose-toolbar-divider"></span>
  <div class="compose-toolbar-group" @mousedown="saveComposeSelection">
    <a-tooltip title="加粗">
      <a-button type="text" class="compose-tool-button" aria-label="加粗" @mousedown.prevent @click="runComposeCommand('bold')">
        <template #icon><bold-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="斜体">
      <a-button type="text" class="compose-tool-button" aria-label="斜体" @mousedown.prevent @click="runComposeCommand('italic')">
        <template #icon><italic-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="下划线">
      <a-button type="text" class="compose-tool-button" aria-label="下划线" @mousedown.prevent @click="runComposeCommand('underline')">
        <template #icon><underline-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="删除线">
      <a-button type="text" class="compose-tool-button" aria-label="删除线" @mousedown.prevent @click="runComposeCommand('strikeThrough')">
        <template #icon><strikethrough-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-popover trigger="click" placement="bottomLeft">
      <template #content>
        <div class="compose-color-grid" @mousedown.prevent>
          <button
            v-for="color in composeTextColors"
            :key="color"
            class="compose-color-swatch"
            :class="{ selected: composeTextColor === color }"
            :style="{ '--compose-swatch-color': color }"
            type="button"
            :aria-label="`文字颜色 ${color}`"
            @click="applyComposeTextColor(color)"
          >
            <check-outlined v-if="composeTextColor === color" />
          </button>
        </div>
      </template>
      <a-button type="text" class="compose-tool-button compose-color-button" aria-label="字体颜色" @mousedown.prevent>
        <template #icon><bg-colors-outlined /></template>
        <span class="compose-color-indicator" :style="{ background: composeTextColor }"></span>
      </a-button>
    </a-popover>
    <a-popover trigger="click" placement="bottomLeft">
      <template #content>
        <div class="compose-color-panel" @mousedown.prevent>
          <button class="compose-color-clear" type="button" @click="clearComposeBackgroundColor">
            无背景
          </button>
          <div class="compose-color-grid">
            <button
              v-for="color in composeBackgroundColors"
              :key="color"
              class="compose-color-swatch"
              :class="{ selected: composeBackgroundColor === color }"
              :style="{ '--compose-swatch-color': color }"
              type="button"
              :aria-label="`背景颜色 ${color}`"
              @click="applyComposeBackgroundColor(color)"
            >
              <check-outlined v-if="composeBackgroundColor === color" />
            </button>
          </div>
        </div>
      </template>
      <a-button type="text" class="compose-tool-button compose-color-button" aria-label="背景颜色" @mousedown.prevent>
        <span class="compose-bg-label">A</span>
        <span class="compose-color-indicator" :style="{ background: composeBackgroundColor || 'transparent' }"></span>
      </a-button>
    </a-popover>
  </div>
  <span class="compose-toolbar-divider"></span>
  <div class="compose-toolbar-group" @mousedown="saveComposeSelection">
    <a-tooltip title="项目列表">
      <a-button type="text" class="compose-tool-button" aria-label="项目列表" @mousedown.prevent @click="runComposeCommand('insertUnorderedList')">
        <template #icon><unordered-list-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="编号列表">
      <a-button type="text" class="compose-tool-button" aria-label="编号列表" @mousedown.prevent @click="runComposeCommand('insertOrderedList')">
        <template #icon><ordered-list-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="左对齐">
      <a-button type="text" class="compose-tool-button" aria-label="左对齐" @mousedown.prevent @click="runComposeCommand('justifyLeft')">
        <template #icon><align-left-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="居中">
      <a-button type="text" class="compose-tool-button" aria-label="居中" @mousedown.prevent @click="runComposeCommand('justifyCenter')">
        <template #icon><align-center-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="右对齐">
      <a-button type="text" class="compose-tool-button" aria-label="右对齐" @mousedown.prevent @click="runComposeCommand('justifyRight')">
        <template #icon><align-right-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="插入链接">
      <a-button type="text" class="compose-tool-button" aria-label="插入链接" @mousedown.prevent @click="insertComposeLink">
        <template #icon><link-outlined /></template>
      </a-button>
    </a-tooltip>
    <a-tooltip title="清除格式">
      <a-button type="text" class="compose-tool-button" aria-label="清除格式" @mousedown.prevent @click="runComposeCommand('removeFormat')">
        <template #icon><clear-outlined /></template>
      </a-button>
    </a-tooltip>
  </div>
</div>
</template>

<script setup lang="ts">
import { PaperClipOutlined, FileImageOutlined, EditOutlined, FontSizeOutlined, BoldOutlined, ItalicOutlined, UnderlineOutlined, StrikethroughOutlined, CheckOutlined, BgColorsOutlined, UnorderedListOutlined, OrderedListOutlined, AlignLeftOutlined, AlignCenterOutlined, AlignRightOutlined, LinkOutlined, ClearOutlined } from '@ant-design/icons-vue';

// 首次写信才加载；正文 DOM 与选区仍由父组件管理，避免打断输入。
defineProps<{
  composeFontFamily: string;
  composeFontSize: string;
  composeTextColor: string;
  composeBackgroundColor: string;
  saveComposeSelection: () => void;
  chooseComposeFiles: () => void;
  chooseComposeImages: () => void;
  insertComposeSignature: () => void;
  applyComposeFontFamily: (value: string) => void;
  applyComposeFontSize: (value: string) => void;
  runComposeCommand: (value: string) => void;
  applyComposeTextColor: (value: string) => void;
  clearComposeBackgroundColor: () => void;
  applyComposeBackgroundColor: (value: string) => void;
  insertComposeLink: () => void;
}>();

const composeFontFamilies = [
  { label: '系统默认', value: 'system-ui' },
  { label: '微软雅黑', value: 'Microsoft YaHei' },
  { label: '苹方', value: 'PingFang SC' },
  { label: '宋体', value: 'SimSun' },
  { label: '黑体', value: 'SimHei' },
  { label: 'Georgia', value: 'Georgia' },
  { label: 'Arial', value: 'Arial' },
  { label: 'Courier New', value: 'Courier New' },
];
const composeFontSizes = [
  { label: '12', value: '12px' },
  { label: '14', value: '14px' },
  { label: '16', value: '16px' },
  { label: '18', value: '18px' },
  { label: '24', value: '24px' },
  { label: '32', value: '32px' },
];
const composeTextColors = ['#1f2937', '#111827', '#b91c1c', '#d97706', '#ca8a04', '#047857', '#0369a1', '#7c3aed', '#be185d', '#ffffff'];
const composeBackgroundColors = ['#ffffff', '#fef3c7', '#fde68a', '#d1fae5', '#dbeafe', '#ede9fe', '#fee2e2', '#f3f4f6'];
</script>

<style scoped>
.compose-toolbar {
  display: flex;
  min-height: 44px;
  align-items: center;
  flex-wrap: nowrap;
  gap: 6px;
  padding: 5px 8px;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-muted);
  overflow-x: auto;
  overflow-y: hidden;
}

.compose-toolbar-group {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  flex: none;
  flex-wrap: nowrap;
  gap: 2px;
}

.compose-toolbar-selects {
  gap: 6px;
}

.compose-font-select {
  width: 126px;
}

.compose-size-select {
  width: 72px;
}

.compose-font-select :deep(.ant-select-selector),
.compose-size-select :deep(.ant-select-selector) {
  height: 32px !important;
}

.compose-font-select :deep(.ant-select-selection-item),
.compose-size-select :deep(.ant-select-selection-item) {
  line-height: 30px !important;
}

.compose-file-input {
  display: none;
}

.compose-tool-button {
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  color: var(--muted-color);
}

.compose-tool-button:hover,
.compose-tool-button:focus {
  color: var(--accent);
  background: var(--accent-soft);
}

.compose-toolbar-divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  margin: 5px 2px 0;
  background: var(--border-color);
}

.compose-color-button {
  position: relative;
}

.compose-color-indicator {
  position: absolute;
  right: 8px;
  bottom: 5px;
  left: 8px;
  height: 3px;
  border: 1px solid rgba(31, 41, 55, 0.12);
  border-radius: 999px;
}

.compose-bg-label {
  font-size: 14px;
  font-weight: 800;
  line-height: 1;
}

.compose-color-panel {
  display: grid;
  gap: 8px;
}

.compose-color-clear {
  height: 30px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--surface-bg);
  color: var(--text-color);
  cursor: pointer;
  font-size: 12px;
}

.compose-color-clear:hover {
  border-color: var(--accent);
  color: var(--accent);
  background: var(--accent-tint);
}

.compose-color-grid {
  display: grid;
  grid-template-columns: repeat(5, 26px);
  gap: 7px;
  padding: 2px;
}

.compose-color-swatch {
  display: inline-flex;
  width: 26px;
  height: 26px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--compose-swatch-color);
  color: #ffffff;
  cursor: pointer;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.28);
}

.compose-color-swatch:hover,
.compose-color-swatch.selected {
  border-color: var(--accent);
  outline: 2px solid var(--accent-soft);
}

.compose-color-swatch[style*="#ffffff"],
.compose-color-swatch[style*="255, 255, 255"] {
  color: var(--heading-color);
}

</style>
