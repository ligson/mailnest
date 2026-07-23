<template>
  <main class="auth-page">
    <section class="auth-panel">
      <div class="auth-header">
        <h1 class="auth-title">创建信匣账号</h1>
        <p class="auth-subtitle">账号创建后即可配置多个邮箱收取邮件</p>
      </div>
      <a-form class="auth-form" layout="vertical" :model="form" @finish="onSubmit">
        <a-form-item
          label="用户名"
          name="username"
          :rules="[{ required: true, message: '请输入用户名' }]"
        >
          <a-input v-model:value="form.username" autocomplete="username" />
        </a-form-item>
        <a-form-item
          label="邮箱"
          name="email"
          :rules="[{ required: true, type: 'email', message: '请输入有效邮箱' }]"
        >
          <a-input v-model:value="form.email" autocomplete="email" />
        </a-form-item>
        <a-form-item
          label="密码"
          name="password"
          :rules="[{ required: true, min: 8, message: '密码至少 8 位' }]"
        >
          <a-input-password v-model:value="form.password" autocomplete="new-password" />
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
        <a-button type="primary" html-type="submit" block :loading="loading">注册并进入</a-button>
        <a-divider />
        <a-button block @click="router.push('/login')">返回登录</a-button>
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
  username: '',
  email: '',
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
    await auth.register(form);
    message.success('注册成功');
    await router.push('/mail');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '注册失败');
    await loadCaptcha();
  } finally {
    loading.value = false;
  }
}
</script>
