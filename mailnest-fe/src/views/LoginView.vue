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
        <a-form-item
          label="图形验证码"
          name="captchaAnswer"
          :rules="[{ required: true, message: '请输入图形验证码' }]"
        >
          <div class="captcha-row">
            <a-input v-model:value="form.captchaAnswer" autocomplete="off" placeholder="输入验证码" />
            <button class="captcha-image-button" type="button" :disabled="captchaLoading" @click="loadCaptcha">
              <a-spin v-if="captchaLoading" size="small" />
              <img v-else-if="captcha.imageData" :src="captcha.imageData" alt="验证码" />
              <span v-else>刷新</span>
            </button>
          </div>
        </a-form-item>
        <a-button type="primary" html-type="submit" block :loading="loading">登录</a-button>
        <a-divider />
        <a-button block @click="router.push('/register')">创建账号</a-button>
      </a-form>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { message } from 'ant-design-vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import { authApi } from '../api/client';

const router = useRouter();
const auth = useAuthStore();
const loading = ref(false);
const form = reactive({
  account: '',
  password: '',
  captchaId: '',
  captchaAnswer: '',
});
const captcha = reactive({
  imageData: '',
});
const captchaLoading = ref(false);

onMounted(loadCaptcha);

async function loadCaptcha() {
  captchaLoading.value = true;
  try {
    const data = await authApi.captcha();
    form.captchaId = data.id;
    form.captchaAnswer = '';
    captcha.imageData = data.imageData;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取验证码失败');
  } finally {
    captchaLoading.value = false;
  }
}

async function onSubmit() {
  loading.value = true;
  try {
    await auth.login(form);
    message.success('登录成功');
    await router.push('/mail');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '登录失败');
    await loadCaptcha();
  } finally {
    loading.value = false;
  }
}
</script>
