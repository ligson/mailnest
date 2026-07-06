<template>
  <main class="auth-page">
    <section class="auth-panel">
      <div class="auth-header">
        <h1 class="auth-title">Mail Nest 信匣</h1>
        <p class="auth-subtitle">登录后查看和管理你的邮件收取空间</p>
      </div>
      <a-form class="auth-form" layout="vertical" :model="form" @finish="onSubmit">
        <a-form-item
          label="用户名或邮箱"
          name="account"
          :rules="[{ required: true, message: '请输入用户名或邮箱' }]"
        >
          <a-input v-model:value="form.account" autocomplete="username" />
        </a-form-item>
        <a-form-item
          label="密码"
          name="password"
          :rules="[{ required: true, message: '请输入密码' }]"
        >
          <a-input-password v-model:value="form.password" autocomplete="current-password" />
        </a-form-item>
        <a-button type="primary" html-type="submit" block :loading="loading">登录</a-button>
        <a-divider />
        <a-button block @click="router.push('/register')">创建账号</a-button>
      </a-form>
    </section>
  </main>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import { message } from 'ant-design-vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '../stores/auth';

const router = useRouter();
const auth = useAuthStore();
const loading = ref(false);
const form = reactive({
  account: '',
  password: '',
});

async function onSubmit() {
  loading.value = true;
  try {
    await auth.login(form);
    message.success('登录成功');
    await router.push('/mail');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '登录失败');
  } finally {
    loading.value = false;
  }
}
</script>
