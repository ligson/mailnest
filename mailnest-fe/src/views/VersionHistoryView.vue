<template>
  <AppLayout selected-key="/settings/versions">
    <section class="content-panel version-history-page">
      <div class="page-toolbar">
        <div>
          <h2 class="page-title">版本记录</h2>
          <p class="page-subtitle">查看 Mail Nest 每个版本的功能改动和发布时间</p>
        </div>
        <a-tag color="blue">当前版本 v{{ frontend.version }}</a-tag>
      </div>
      <div class="release-list">
        <article v-for="release in releaseNotes" :key="release.version" class="release-card" :class="{ current: release.version === frontend.version }">
          <header class="release-card-header">
            <div class="release-card-title"><a-tag color="blue">v{{ release.version }}</a-tag><strong>{{ release.summary }}</strong></div>
            <time>{{ release.date }}</time>
          </header>
          <ul><li v-for="item in release.items" :key="item">{{ item }}</li></ul>
        </article>
      </div>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '../components/AppLayout.vue';
import { releaseNotes } from '../utils/releaseNotes';

const frontend = __APP_BUILD_INFO__;
</script>

<style scoped>
.version-history-page { display: flex; flex-direction: column; gap: 20px; }
.version-history-page .page-toolbar { align-items: center; }
.release-list { display: grid; gap: 14px; max-width: 1000px; }
.release-card { padding: 20px 22px; border: 1px solid var(--border-color); border-radius: 8px; background: var(--surface-bg); box-shadow: var(--shadow-soft); }
.release-card.current { border-color: var(--accent); box-shadow: 0 0 0 2px var(--accent-soft); }
.release-card-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
.release-card-title { display: flex; min-width: 0; align-items: center; gap: 12px; color: var(--heading-color); font-size: 16px; }
.release-card-title strong { overflow-wrap: anywhere; }
.release-card-header time { flex: none; color: var(--muted-color); font-size: 13px; }
.release-card ul { display: grid; gap: 9px; margin: 0; padding-left: 22px; color: var(--text-color); line-height: 1.65; }
@media (max-width: 640px) { .release-card { padding: 16px; } .release-card-header { display: grid; gap: 8px; } }
</style>
