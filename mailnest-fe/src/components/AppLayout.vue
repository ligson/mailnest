<template>
  <a-layout class="app-shell">
    <a-layout-sider
      class="shell-sider"
      :width="220"
      :collapsed-width="64"
      :collapsed="collapsed"
      :trigger="null"
    >
      <div class="shell-brand" :class="{ collapsed }">
        <div class="shell-logo">
          <img class="shell-logo-mark" src="/mailnest-icon.svg" alt="" />
          <span class="shell-logo-text">Mail Nest</span>
        </div>
      </div>
      <a-menu theme="dark" mode="inline" :selected-keys="[selectedKey]" @click="onMenuClick">
        <a-menu-item key="/mail">
          <mail-outlined />
          <span>邮件</span>
        </a-menu-item>
        <a-menu-item key="/attachments">
          <paper-clip-outlined />
          <span>附件中心</span>
        </a-menu-item>
        <a-menu-item key="/accounts">
          <setting-outlined />
          <span>邮箱账号</span>
        </a-menu-item>
        <a-menu-item key="/contacts">
          <team-outlined />
          <span>联系人</span>
        </a-menu-item>
        <a-menu-item key="/rules">
          <branches-outlined />
          <span>规则</span>
        </a-menu-item>
        <a-menu-item key="/settings/profile">
          <user-outlined />
          <span>设置</span>
        </a-menu-item>
        <a-menu-item v-if="auth.user?.isAdmin" key="/admin/users">
          <team-outlined />
          <span>系统管理</span>
        </a-menu-item>
      </a-menu>
      <div class="shell-collapse-panel" :class="{ collapsed }">
        <a-tooltip :title="collapsed ? '展开菜单' : '锁起菜单'" placement="right">
          <a-button
            class="shell-lock-button"
            type="text"
            :aria-label="collapsed ? '展开菜单' : '锁起菜单'"
            @click="toggleCollapsed"
          >
            <template #icon>
              <unlock-outlined v-if="collapsed" />
              <lock-outlined v-else />
            </template>
            <span class="shell-lock-text">锁起菜单</span>
          </a-button>
        </a-tooltip>
      </div>
    </a-layout-sider>
    <a-layout class="shell-main">
      <a-layout-header class="shell-header">
        <div class="shell-header-spacer"></div>
        <a-dropdown :trigger="['click']">
          <a-button class="shell-user-button" type="text">
            <a-avatar :size="28" :src="auth.user?.avatarUrl || undefined">
              {{ avatarFallback }}
            </a-avatar>
            <span class="shell-user-name">{{ displayName }}</span>
            <down-outlined class="shell-user-caret" />
          </a-button>
          <template #overlay>
            <a-menu @click="onUserMenuClick">
              <a-menu-item key="profile">
                <user-outlined />
                <span>个人设置</span>
              </a-menu-item>
              <a-menu-item key="change-password">
                <lock-outlined />
                <span>修改密码</span>
              </a-menu-item>
              <a-menu-item key="logout">
                <logout-outlined />
                <span>退出登录</span>
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </a-layout-header>
      <a-layout-content class="shell-content">
        <slot />
      </a-layout-content>
    </a-layout>
    <a-modal
      v-model:open="passwordModalOpen"
      title="修改密码"
      ok-text="确认修改"
      cancel-text="取消"
      :confirm-loading="changingPassword"
      destroy-on-close
      @ok="submitPasswordForm"
    >
      <a-form ref="passwordFormRef" layout="vertical" :model="passwordForm" :rules="passwordRules">
        <a-form-item label="当前密码" name="currentPassword">
          <a-input-password v-model:value="passwordForm.currentPassword" autocomplete="current-password" />
        </a-form-item>
        <a-form-item label="新密码" name="newPassword">
          <a-input-password v-model:value="passwordForm.newPassword" autocomplete="new-password" />
        </a-form-item>
        <a-form-item label="确认新密码" name="confirmPassword">
          <a-input-password v-model:value="passwordForm.confirmPassword" autocomplete="new-password" />
        </a-form-item>
      </a-form>
    </a-modal>
  </a-layout>
</template>

<script setup lang="ts">
import {
  BranchesOutlined,
  DownOutlined,
  LockOutlined,
  LogoutOutlined,
  MailOutlined,
  PaperClipOutlined,
  SettingOutlined,
  TeamOutlined,
  UnlockOutlined,
  UserOutlined,
} from '@ant-design/icons-vue';
import { computed, reactive, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { message, type FormInstance } from 'ant-design-vue';
import type { MenuInfo } from 'ant-design-vue/es/menu/src/interface';
import { authApi } from '../api/client';
import { useAuthStore } from '../stores/auth';

defineProps<{
  selectedKey: string;
}>();

const router = useRouter();
const auth = useAuthStore();
const collapsedStorageKey = 'mailnest.sidebar.collapsed';
const collapsed = ref(localStorage.getItem(collapsedStorageKey) === 'true');
const passwordModalOpen = ref(false);
const changingPassword = ref(false);
const passwordFormRef = ref<FormInstance>();
const passwordForm = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
});
const displayName = computed(() => auth.user?.nickname || auth.user?.username || '当前用户');
const avatarFallback = computed(() => displayName.value.slice(0, 1).toUpperCase());
const passwordRules = {
  currentPassword: [{ required: true, message: '请输入当前密码' }],
  newPassword: [
    { required: true, message: '请输入新密码' },
    { min: 8, message: '新密码至少 8 位' },
  ],
  confirmPassword: [
    { required: true, message: '请再次输入新密码' },
    {
      validator: async (_rule: unknown, value: string) => {
        if (value !== passwordForm.newPassword) {
          throw new Error('两次输入的新密码不一致');
        }
      },
    },
  ],
};

watch(collapsed, (value) => {
  localStorage.setItem(collapsedStorageKey, String(value));
});

function toggleCollapsed() {
  collapsed.value = !collapsed.value;
}

async function onMenuClick(info: MenuInfo) {
  await router.push(String(info.key));
}

async function onUserMenuClick(info: MenuInfo) {
  if (info.key === 'profile') {
    await router.push('/settings/profile');
    return;
  }
  if (info.key === 'change-password') {
    openPasswordModal();
    return;
  }
  if (info.key === 'logout') {
    await onLogout();
  }
}

function openPasswordModal() {
  resetPasswordForm();
  passwordModalOpen.value = true;
}

function resetPasswordForm() {
  passwordForm.currentPassword = '';
  passwordForm.newPassword = '';
  passwordForm.confirmPassword = '';
  passwordFormRef.value?.clearValidate();
}

async function submitPasswordForm() {
  try {
    await passwordFormRef.value?.validate();
  } catch {
    return;
  }
  changingPassword.value = true;
  try {
    await authApi.changePassword(passwordForm);
    message.success('密码修改成功');
    passwordModalOpen.value = false;
    resetPasswordForm();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '密码修改失败');
  } finally {
    changingPassword.value = false;
  }
}

async function onLogout() {
  await auth.logout();
  await router.push('/login');
}
</script>
