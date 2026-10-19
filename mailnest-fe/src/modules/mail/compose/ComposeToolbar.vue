<template>
  <div class="compose-toolbar" role="toolbar" aria-label="正文格式">
    <div class="compose-toolbar-row">
      <div class="compose-toolbar-group">
        <a-tooltip v-for="tool in historyTools" :key="tool.label" :title="tool.label">
          <a-button type="text" class="compose-tool-button" :aria-label="tool.label" :disabled="!tool.enabled" @mousedown.prevent @click="tool.run"><component :is="tool.icon" /></a-button>
        </a-tooltip>
      </div>
      <span class="compose-toolbar-divider" />
      <a-select :value="format.fontFamily" :options="fonts" class="compose-font-select" size="small" aria-label="字体" placeholder="混合字体" :show-search="false" @change="setFont" />
      <a-select :value="format.fontSize" :options="sizes" class="compose-size-select" size="small" aria-label="字号" placeholder="混合" :show-search="false" @change="setSize" />
      <span class="compose-toolbar-divider" />
      <div class="compose-toolbar-group">
        <a-tooltip v-for="tool in inlineTools" :key="tool.label" :title="tool.label">
          <a-button type="text" class="compose-tool-button" :class="{ active: tool.active }" :aria-label="tool.label" :aria-pressed="tool.active" @mousedown.prevent @click="tool.run"><component :is="tool.icon" /></a-button>
        </a-tooltip>
        <a-popover trigger="click" placement="bottomLeft">
          <template #content>
            <div class="compose-color-panel">
              <a-button size="small" @mousedown.prevent @click="editor.chain().focus().unsetColor().run()">默认文字颜色</a-button>
              <div class="compose-color-grid">
                <button v-for="color in textColors" :key="color" type="button" class="compose-color-swatch" :style="{ backgroundColor: color }" :aria-label="'文字颜色 ' + color" @mousedown.prevent @click="editor.chain().focus().setColor(color).run()" />
              </div>
            </div>
          </template>
          <a-button type="text" class="compose-tool-button compose-color-button" aria-label="字体颜色" title="字体颜色" @mousedown.prevent><font-colors-outlined /><i :style="{ backgroundColor: format.color || '#1f2937' }" /></a-button>
        </a-popover>
        <a-popover trigger="click" placement="bottomLeft">
          <template #content>
            <div class="compose-color-panel">
              <a-button size="small" @mousedown.prevent @click="editor.chain().focus().unsetBackgroundColor().run()">无背景</a-button>
              <div class="compose-color-grid">
                <button v-for="color in backgroundColors" :key="color" type="button" class="compose-color-swatch" :style="{ backgroundColor: color }" :aria-label="'背景颜色 ' + color" @mousedown.prevent @click="editor.chain().focus().setBackgroundColor(color).run()" />
              </div>
            </div>
          </template>
          <a-button type="text" class="compose-tool-button compose-color-button" aria-label="背景颜色" title="背景颜色" @mousedown.prevent><bg-colors-outlined /><i :style="{ backgroundColor: format.backgroundColor || 'transparent' }" /></a-button>
        </a-popover>
      </div>
      <span class="compose-toolbar-divider" />
      <div class="compose-toolbar-group">
        <a-tooltip v-for="tool in insertTools" :key="tool.label" :title="tool.label">
          <a-button type="text" class="compose-tool-button" :aria-label="tool.label" @mousedown.prevent @click="tool.run"><component :is="tool.icon" /></a-button>
        </a-tooltip>
        <a-button type="text" class="compose-tool-button" :class="{active: more}" aria-label="更多格式" title="更多格式" :aria-expanded="more" @mousedown.prevent @click="more = !more"><more-outlined /></a-button>
      </div>
    </div>
    <div v-if="more" class="compose-toolbar-row compose-toolbar-extra">
      <a-select :value="blockType" class="compose-paragraph-select" size="small" aria-label="段落样式" :options="blocks" @change="setBlock" />
      <a-select :value="lineHeight" class="compose-line-select" size="small" aria-label="行间距" :options="lineHeights" @change="setLineHeight" />
      <div class="compose-toolbar-group">
        <a-tooltip v-for="tool in paragraphTools" :key="tool.label" :title="tool.label">
          <a-button type="text" class="compose-tool-button" :class="{active: tool.active}" :aria-label="tool.label" :aria-pressed="tool.active" @mousedown.prevent @click="tool.run"><component :is="tool.icon" /></a-button>
        </a-tooltip>
      </div>
      <a-dropdown :trigger="['click']">
        <a-button type="text" class="compose-tool-button" aria-label="表格操作" title="表格操作" @mousedown.prevent><table-outlined /></a-button>
        <template #overlay><a-menu @click="tableAction">
          <a-menu-item key="insert">插入 3 × 3 表格</a-menu-item>
          <a-menu-item key="row" :disabled="!inTable">在下方插入行</a-menu-item>
          <a-menu-item key="column" :disabled="!inTable">在右侧插入列</a-menu-item>
          <a-menu-item key="removeRow" :disabled="!inTable">删除当前行</a-menu-item>
          <a-menu-item key="removeColumn" :disabled="!inTable">删除当前列</a-menu-item>
          <a-menu-item key="merge" :disabled="!editor.can().mergeCells()">合并单元格</a-menu-item>
          <a-menu-item key="split" :disabled="!editor.can().splitCell()">拆分单元格</a-menu-item>
          <a-menu-item key="remove" :disabled="!inTable" danger>删除表格</a-menu-item>
        </a-menu></template>
      </a-dropdown>
      <a-tooltip title="清除格式"><a-button type="text" class="compose-tool-button" aria-label="清除格式" @mousedown.prevent @click="clearFormat"><clear-outlined /></a-button></a-tooltip>
      <a-tooltip title="插入签名"><a-button type="text" class="compose-tool-button" aria-label="插入签名" @mousedown.prevent @click="emit('signature')"><edit-outlined /></a-button></a-tooltip>
    </div>
    <div v-if="imageSelected" class="compose-toolbar-row compose-image-tools">
      <span>图片宽度</span>
      <a-select size="small" aria-label="图片宽度" :value="editor.getAttributes('image').width || '100%'" :options="imageWidths" @change="setImageWidth" />
      <a-tooltip title="删除图片"><a-button type="text" class="compose-tool-button" aria-label="删除图片" @mousedown.prevent @click="editor.chain().focus().deleteSelection().run()"><delete-outlined /></a-button></a-tooltip>
    </div>
  </div>
  <a-modal v-model:open="linkOpen" title="编辑链接" ok-text="应用" cancel-text="取消" :mask-closable="false" @ok="applyLink">
    <a-form layout="vertical">
      <a-form-item label="链接文字"><a-input v-model:value="linkText" placeholder="留空时使用链接地址" /></a-form-item>
      <a-form-item label="链接地址" required :validate-status="linkError ? 'error' : undefined" :help="linkError"><a-input v-model:value="linkURL" placeholder="https:// 或 mailto:" @press-enter="applyLink" /></a-form-item>
    </a-form>
    <a-button v-if="linkExisting" danger @click="removeLink">移除链接</a-button>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue';
import type { Editor } from '@tiptap/core';
import { BoldOutlined, ItalicOutlined, UnderlineOutlined, StrikethroughOutlined, FontColorsOutlined, BgColorsOutlined, UndoOutlined, RedoOutlined, PaperClipOutlined, FileImageOutlined, LinkOutlined, MoreOutlined, TableOutlined, ClearOutlined, EditOutlined, DeleteOutlined, AlignLeftOutlined, AlignCenterOutlined, AlignRightOutlined, UnorderedListOutlined, OrderedListOutlined, BlockOutlined, MinusOutlined, CodeOutlined, MenuUnfoldOutlined, MenuFoldOutlined } from '@ant-design/icons-vue';
import { readSelectionFormat, normalizeMailLink } from './editor-format';

const props = defineProps<{ editor: Editor }>();
const emit = defineEmits<{files: []; images: []; signature: []}>();
const more = ref(false);
// 格式工具栏同时响应光标、选区和文档事务，而不只响应鼠标点击。
const revision = ref(0);
const changed = () => { revision.value++; };
props.editor.on('transaction', changed);
onBeforeUnmount(() => props.editor.off('transaction', changed));
const format = computed(() => { void revision.value; return readSelectionFormat(props.editor); });
const inTable = computed(() => { void revision.value; return props.editor.isActive('table'); });
const imageSelected = computed(() => { void revision.value; return props.editor.isActive('image'); });
const blockType = computed(() => { void revision.value; return props.editor.isActive('heading') ? String(props.editor.getAttributes('heading').level) : 'paragraph'; });
const lineHeight = computed(() => { void revision.value; return props.editor.getAttributes(props.editor.isActive('heading') ? 'heading' : 'paragraph').lineHeight || '1.7'; });
const historyTools = computed(() => { void revision.value; return [
  {label:'撤销',icon:UndoOutlined,enabled:props.editor.can().undo(),run:()=>props.editor.chain().focus().undo().run()},
  {label:'重做',icon:RedoOutlined,enabled:props.editor.can().redo(),run:()=>props.editor.chain().focus().redo().run()},
]; });
const inlineTools = computed(() => { void revision.value; return [
  {label:'加粗',icon:BoldOutlined,active:props.editor.isActive('bold'),run:()=>props.editor.chain().focus().toggleBold().run()},
  {label:'斜体',icon:ItalicOutlined,active:props.editor.isActive('italic'),run:()=>props.editor.chain().focus().toggleItalic().run()},
  {label:'下划线',icon:UnderlineOutlined,active:props.editor.isActive('underline'),run:()=>props.editor.chain().focus().toggleUnderline().run()},
  {label:'删除线',icon:StrikethroughOutlined,active:props.editor.isActive('strike'),run:()=>props.editor.chain().focus().toggleStrike().run()},
]; });
const insertTools = [
  {label:'添加附件',icon:PaperClipOutlined,run:()=>emit('files')},
  {label:'插入图片',icon:FileImageOutlined,run:()=>emit('images')},
  {label:'插入链接',icon:LinkOutlined,run:openLink},
];
const paragraphTools = computed(() => { void revision.value; return [
  ...[{label:'左对齐',value:'left',icon:AlignLeftOutlined},{label:'居中',value:'center',icon:AlignCenterOutlined},{label:'右对齐',value:'right',icon:AlignRightOutlined}].map(t=>({...t,active:props.editor.isActive({textAlign:t.value}),run:()=>props.editor.chain().focus().setTextAlign(t.value).run()})),
  {label:'项目列表',icon:UnorderedListOutlined,active:props.editor.isActive('bulletList'),run:()=>props.editor.chain().focus().toggleBulletList().run()},
  {label:'编号列表',icon:OrderedListOutlined,active:props.editor.isActive('orderedList'),run:()=>props.editor.chain().focus().toggleOrderedList().run()},
  {label:'增加列表缩进',icon:MenuUnfoldOutlined,active:false,run:()=>props.editor.chain().focus().sinkListItem('listItem').run()},
  {label:'减少列表缩进',icon:MenuFoldOutlined,active:false,run:()=>props.editor.chain().focus().liftListItem('listItem').run()},
  {label:'引用',icon:BlockOutlined,active:props.editor.isActive('blockquote'),run:()=>props.editor.chain().focus().toggleBlockquote().run()},
  {label:'代码块',icon:CodeOutlined,active:props.editor.isActive('codeBlock'),run:()=>props.editor.chain().focus().toggleCodeBlock().run()},
  {label:'分隔线',icon:MinusOutlined,active:false,run:()=>props.editor.chain().focus().setHorizontalRule().run()},
]; });
const fonts = [{label:'系统默认',value:'system-ui'},{label:'微软雅黑',value:'Microsoft YaHei'},{label:'苹方',value:'PingFang SC'},{label:'宋体',value:'SimSun'},{label:'黑体',value:'SimHei'},{label:'Arial',value:'Arial'},{label:'Georgia',value:'Georgia'},{label:'Courier New',value:'Courier New'}];
const sizes = [12,14,16,18,20,24,28,32,36,48].map(n=>({label:String(n),value:n+'px'}));
const blocks = [{label:'正文',value:'paragraph'},{label:'标题一',value:'1'},{label:'标题二',value:'2'},{label:'标题三',value:'3'}];
const lineHeights = ['1','1.3','1.5','1.7','2','2.5','3'].map(n=>({label:n+' 倍行距',value:n}));
const imageWidths = ['25%','50%','75%','100%'].map(value=>({label:value,value}));
const textColors = ['#1f2937','#111827','#b91c1c','#d97706','#ca8a04','#047857','#0369a1','#7c3aed','#be185d','#ffffff'];
const backgroundColors = ['#ffffff','#fef3c7','#fde68a','#d1fae5','#dbeafe','#ede9fe','#fee2e2','#f3f4f6'];
function setFont(value: unknown) { props.editor.chain().focus().setFontFamily(String(value)).run(); }
function setImageWidth(value: unknown) { props.editor.chain().focus().updateAttributes('image', {width: String(value)}).run(); }
function setSize(value: unknown) { props.editor.chain().focus().setFontSize(String(value)).run(); }
function setBlock(value: unknown) { const chain=props.editor.chain().focus(); value==='paragraph' ? chain.setParagraph().run() : chain.setHeading({level:Number(value) as 1|2|3}).run(); }
function setLineHeight(value: unknown) { props.editor.chain().focus().updateAttributes(props.editor.isActive('heading')?'heading':'paragraph',{lineHeight:String(value)}).run(); }
function clearFormat() { props.editor.chain().focus().unsetAllMarks().clearNodes().resetAttributes('paragraph',['lineHeight','textAlign']).run(); }
function tableAction({key}: {key: string | number}) {
  const chain=props.editor.chain().focus();
  switch(key) {
    case 'insert': chain.insertTable({rows:3,cols:3,withHeaderRow:true}).run(); break;
    case 'row': chain.addRowAfter().run(); break;
    case 'column': chain.addColumnAfter().run(); break;
    case 'removeRow': chain.deleteRow().run(); break;
    case 'removeColumn': chain.deleteColumn().run(); break;
    case 'merge': chain.mergeCells().run(); break;
    case 'split': chain.splitCell().run(); break;
    case 'remove': chain.deleteTable().run(); break;
  }
}
const linkOpen=ref(false),linkURL=ref(''),linkText=ref(''),linkError=ref(''),linkExisting=ref(false);
let linkRange={from:0,to:0};
let originalLinkText='';
function openLink() {
  const editor=props.editor;
  linkExisting.value=editor.isActive('link');
  if(linkExisting.value) editor.commands.extendMarkRange('link');
  linkRange={from:editor.state.selection.from,to:editor.state.selection.to};
  originalLinkText=editor.state.doc.textBetween(linkRange.from,linkRange.to,' ');
  linkText.value=originalLinkText; linkURL.value=editor.getAttributes('link').href||'';linkError.value='';linkOpen.value=true;
}
function applyLink() {
  const href=normalizeMailLink(linkURL.value);
  if(!href){linkError.value='请输入有效的 https://、http://、mailto: 或 tel: 链接';return;}
  const text=linkText.value.trim()||href;
  const chain=props.editor.chain().focus().setTextSelection(linkRange);
  if(linkRange.from===linkRange.to||text!==originalLinkText) chain.insertContent({type:'text',text,marks:[{type:'link',attrs:{href}}]}).run();
  else chain.setLink({href}).run();
  linkOpen.value=false;
}
function removeLink() { props.editor.chain().focus().setTextSelection(linkRange).unsetLink().run();linkOpen.value=false; }
</script>

<style scoped>
.compose-toolbar { flex: none; padding: 5px 8px; border-bottom: 1px solid var(--border-subtle); background: var(--surface-muted); }
.compose-toolbar-row { display: flex; align-items: center; flex-wrap: wrap; gap: 4px; }
.compose-toolbar-group { display: inline-flex; align-items: center; gap: 1px; flex: none; }
.compose-toolbar-extra { margin-top: 5px; padding-top: 5px; border-top: 1px solid var(--border-subtle); }
.compose-tool-button { display: inline-flex; align-items: center; justify-content: center; width: 30px; height: 30px; padding: 0; flex: 0 0 30px; color: var(--muted-color); }
.compose-tool-button.active { color: var(--accent); background: var(--accent-soft); }
.compose-tool-button:hover { color: var(--accent); }
.compose-toolbar-divider { width: 1px; height: 20px; background: var(--border-color); margin: 0 3px; }
.compose-font-select { width: 122px; }
.compose-size-select { width: 64px; }
.compose-paragraph-select { width: 98px; }
.compose-line-select { width: 112px; }
.compose-color-button { position: relative; }
.compose-color-button i { position: absolute; right: 7px; left: 7px; bottom: 3px; height: 3px; border: 1px solid #00000015; }
.compose-color-panel { display: grid; gap: 10px; }
.compose-color-grid { display: grid; grid-template-columns: repeat(5,26px); gap: 7px; }
.compose-color-swatch { width: 26px; height: 26px; border: 1px solid #d9d9d9; border-radius: 4px; cursor: pointer; }
.compose-color-swatch:hover,.compose-color-swatch:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.compose-image-tools { margin-top: 5px; font-size: 12px; color: var(--muted-color); }
.compose-image-tools .ant-select { width: 90px; }
@media(max-width:600px) { .compose-toolbar { padding: 4px; } .compose-toolbar-row { gap: 3px; } .compose-toolbar-divider { margin: 0; } }
</style>
