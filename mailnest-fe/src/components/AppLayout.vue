<template>
  <a-layout class="app-shell">
    <a-layout-sider class="shell-sider" :width="220">
      <div class="shell-logo">Mail Nest</div>
      <a-menu theme="dark" mode="inline" :selected-keys="[selectedKey]" @click="onMenuClick">
        <a-menu-item key="/mail">
          <mail-outlined />
          <span>邮件</span>
        </a-menu-item>
        <a-menu-item key="/accounts">
          <setting-outlined />
          <span>邮箱账号</span>
        </a-menu-item>
        <a-menu-item key="/rules">
          <branches-outlined />
          <span>规则</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>
    <a-layout>
      <a-layout-header class="shell-header">
        <div>{{ auth.user?.username || '当前用户' }}</div>
        <a-button @click="onLogout">退出登录</a-button>
      </a-layout-header>
      <a-layout-content class="shell-content">
        <slot />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { BranchesOutlined, MailOutlined, SettingOutlined } from '@ant-design/icons-vue';
import { useRouter } from 'vue-router';
import type { MenuInfo } from 'ant-design-vue/es/menu/src/interface';
import { useAuthStore } from '../stores/auth';

defineProps<{
  selectedKey: string;
}>();

const router = useRouter();
const auth = useAuthStore();

async function onMenuClick(info: MenuInfo) {
  await router.push(String(info.key));
}

async function onLogout() {
  await auth.logout();
  await router.push('/login');
}
</script>
