<template>
  <main class="auth-page">
    <section class="auth-panel">
      <div class="auth-header">
        <h1 class="auth-title">Microsoft 授权</h1>
        <p class="auth-subtitle">{{ statusText }}</p>
      </div>
      <div class="auth-form">
        <a-alert v-if="errorText" type="error" :message="errorText" show-icon />
        <a-button v-if="errorText" block style="margin-top: 16px" @click="router.push('/accounts')">
          返回邮箱账号
        </a-button>
        <a-spin v-else />
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { message } from 'ant-design-vue';
import { useRoute, useRouter } from 'vue-router';
import { oauthApi } from '../api/client';

const route = useRoute();
const router = useRouter();
const statusText = ref('正在完成授权...');
const errorText = ref('');

onMounted(async () => {
  const code = String(route.query.code || '');
  const state = String(route.query.state || '');
  if (!code || !state) {
    errorText.value = 'Microsoft 回调参数不完整';
    statusText.value = '授权失败';
    return;
  }

  try {
    await oauthApi.completeMicrosoft({ code, state });
    message.success('Microsoft 邮箱授权成功');
    await router.push('/accounts');
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : 'Microsoft 授权失败';
    statusText.value = '授权失败';
  }
});
</script>
