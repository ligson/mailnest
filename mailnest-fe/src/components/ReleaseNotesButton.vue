<template>
  <a-button class="release-notes-button" type="text" @click="open = true">
    <span>v{{ frontend.version }}</span>
    <span class="release-notes-label">发布内容</span>
  </a-button>
  <a-modal v-model:open="open" title="版本发布内容" :footer="null" width="680px">
    <div v-if="release" class="release-detail">
      <div class="release-meta"><a-tag color="blue">{{ release.version }}</a-tag><span>{{ release.date }}</span></div>
      <p class="release-summary">{{ release.summary }}</p>
      <a-divider />
      <ul><li v-for="item in release.items" :key="item">{{ item }}</li></ul>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

const frontend = __APP_BUILD_INFO__;
const open = ref(false);
const release = computed(() => releaseNotes[0]);
const releaseNotes = [
  { version: '0.6.0', date: '2026-11-27', summary: '再次发送、版本记录、附件加载和联系人分组优化', items: ['邮件详情支持再发一封，保留原收件人、抄送、密送、正文和普通附件', '系统管理版本号可点击查看发布内容', '附件中心首屏跳过大表总数统计，联系人分组成员改为可搜索表格', '邮件操作按钮统一间距，手机端同步支持再次发送'] },
  { version: '0.5.0', date: '2026-10-19', summary: '独立写信与完整签名排版', items: ['写邮件、回复和转发支持转到新窗口继续编辑', '收件人、抄送、密送、正文和附件可在窗口间接续', '签名保留原有颜色、字号、边框、间距和布局表格', '草稿保存串行化，避免迟到请求覆盖最新内容'] },
];
</script>

<style scoped>
.release-notes-button { display: inline-flex; align-items: center; gap: 6px; color: var(--muted-color); font-size: 12px; }
.release-notes-button:hover { color: var(--heading-color); }
.release-notes-label { opacity: .75; }
.release-meta { display: flex; align-items: center; gap: 10px; color: var(--muted-color); }
.release-summary { margin: 16px 0 0; color: var(--heading-color); font-size: 15px; font-weight: 600; }
.release-detail ul { display: grid; gap: 10px; margin: 0; padding-left: 20px; color: var(--text-color); line-height: 1.65; }
</style>
