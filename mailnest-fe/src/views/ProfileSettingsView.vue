<template>
  <AppLayout selected-key="/settings/profile">
    <section class="content-panel profile-settings">
      <div class="page-toolbar">
        <div>
          <h2 class="page-title">个人设置</h2>
          <p class="page-subtitle">维护你的头像、昵称和个人描述</p>
        </div>
      </div>

      <div class="profile-settings-grid">
        <div class="profile-avatar-panel">
          <a-avatar :size="88" :src="avatarSrc">
            {{ avatarFallback }}
          </a-avatar>
          <div class="profile-avatar-actions">
            <a-upload
              accept="image/png,image/jpeg,image/webp,image/gif"
              :before-upload="uploadAvatar"
              :show-upload-list="false"
            >
              <a-button :loading="uploading">上传头像</a-button>
            </a-upload>
            <span class="profile-avatar-hint">支持 PNG、JPG、WEBP、GIF，最大 2MB</span>
          </div>
        </div>

        <a-form class="profile-form" layout="vertical" :model="form" @finish="saveProfile">
          <div class="profile-readonly-row">
            <a-form-item label="用户名">
              <a-input :value="auth.user?.username" disabled />
            </a-form-item>
            <a-form-item label="邮箱">
              <a-input :value="auth.user?.email" disabled />
            </a-form-item>
          </div>
          <a-form-item label="昵称" name="nickname">
            <a-input v-model:value="form.nickname" maxlength="40" show-count placeholder="用于界面展示" />
          </a-form-item>
          <a-form-item label="个人描述" name="bio">
            <a-textarea
              v-model:value="form.bio"
              :auto-size="{ minRows: 4, maxRows: 6 }"
              maxlength="200"
              show-count
              placeholder="写一句简单的个人说明"
            />
          </a-form-item>
          <a-space>
            <a-button type="primary" html-type="submit" :loading="saving">保存资料</a-button>
            <a-button @click="resetForm">取消</a-button>
          </a-space>
        </a-form>
      </div>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { message } from 'ant-design-vue';
import { profileApi } from '../api/client';
import AppLayout from '../components/AppLayout.vue';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
const saving = ref(false);
const uploading = ref(false);
const form = reactive({
  nickname: '',
  bio: '',
});

const avatarSrc = computed(() => auth.user?.avatarUrl || undefined);
const displayName = computed(() => auth.user?.nickname || auth.user?.username || '用户');
const avatarFallback = computed(() => displayName.value.slice(0, 1).toUpperCase());

onMounted(resetForm);

function resetForm() {
  form.nickname = auth.user?.nickname || '';
  form.bio = auth.user?.bio || '';
}

async function saveProfile() {
  saving.value = true;
  try {
    const user = await profileApi.update(form);
    auth.setUser(user);
    message.success('个人资料已保存');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存个人资料失败');
  } finally {
    saving.value = false;
  }
}

async function uploadAvatar(file: File) {
  uploading.value = true;
  try {
    const user = await profileApi.uploadAvatar(file);
    auth.setUser(user);
    message.success('头像已更新');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '上传头像失败');
  } finally {
    uploading.value = false;
  }
  return false;
}
</script>
