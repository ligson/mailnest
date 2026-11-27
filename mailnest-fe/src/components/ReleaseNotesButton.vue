<template>
  <a-button class="release-notes-button" type="text" @click="router.push('/settings/versions')">
    <span>v{{ frontend.version }}</span>
    <span class="release-notes-label">发布内容</span>
  </a-button>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';

const frontend = __APP_BUILD_INFO__;
const router = useRouter();
</script>

<style scoped>
.release-notes-button { display: inline-flex; align-items: center; gap: 6px; color: var(--muted-color); font-size: 12px; }
.release-notes-button:hover { color: var(--heading-color); }
.release-notes-label { opacity: .75; }
</style>
