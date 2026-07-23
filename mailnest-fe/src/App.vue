<template>
  <a-config-provider :locale="zhCN" :theme="antdTheme">
    <router-view />
  </a-config-provider>
</template>

<script setup lang="ts">
import { computed, watchEffect } from 'vue';
import zhCN from 'ant-design-vue/es/locale/zh_CN';
import { useAuthStore } from './stores/auth';

const auth = useAuthStore();
const themeKey = computed(() => normalizeTheme(auth.user?.uiTheme));
const themeTokens: Record<string, Record<string, string>> = {
  forest: {
    colorPrimary: '#24776d',
    colorInfo: '#24776d',
    colorLink: '#1f766d',
  },
  sky: {
    colorPrimary: '#2563b8',
    colorInfo: '#2563b8',
    colorLink: '#1d5fae',
  },
  grape: {
    colorPrimary: '#7c4d9f',
    colorInfo: '#7c4d9f',
    colorLink: '#76469b',
  },
  ember: {
    colorPrimary: '#b45325',
    colorInfo: '#b45325',
    colorLink: '#a44920',
  },
  graphite: {
    colorPrimary: '#42546b',
    colorInfo: '#42546b',
    colorLink: '#384a60',
  },
  qinghua: {
    colorPrimary: '#1f5f8b',
    colorInfo: '#1f5f8b',
    colorLink: '#1b547b',
  },
  cinnabar: {
    colorPrimary: '#b43b2d',
    colorInfo: '#b43b2d',
    colorLink: '#9f3126',
  },
  ink: {
    colorPrimary: '#3f4b55',
    colorInfo: '#3f4b55',
    colorLink: '#34414b',
  },
  daishan: {
    colorPrimary: '#2f6f68',
    colorInfo: '#2f6f68',
    colorLink: '#285f59',
  },
};
const antdTheme = computed(() => ({
  token: {
    borderRadius: 8,
    colorPrimary: themeTokens[themeKey.value].colorPrimary,
    colorInfo: themeTokens[themeKey.value].colorInfo,
    colorLink: themeTokens[themeKey.value].colorLink,
  },
}));

watchEffect(() => {
  document.documentElement.dataset.theme = themeKey.value;
});

function normalizeTheme(value?: string | null) {
  return ['forest', 'sky', 'grape', 'ember', 'graphite', 'qinghua', 'cinnabar', 'ink', 'daishan'].includes(value || '')
    ? String(value)
    : 'forest';
}
</script>
