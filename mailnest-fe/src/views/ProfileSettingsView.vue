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
          <a-form-item label="界面主题" name="uiTheme">
            <div class="theme-picker" role="radiogroup" aria-label="界面主题">
              <button
                v-for="theme in themeOptions"
                :key="theme.key"
                class="theme-choice"
                :class="{ active: form.uiTheme === theme.key }"
                type="button"
                role="radio"
                :aria-checked="form.uiTheme === theme.key"
                @click="form.uiTheme = theme.key"
              >
                <span class="theme-choice-preview" :style="{ background: theme.preview }">
                  <check-outlined v-if="form.uiTheme === theme.key" />
                </span>
                <span class="theme-choice-copy">
                  <strong>{{ theme.name }}</strong>
                  <span>{{ theme.description }}</span>
                </span>
              </button>
            </div>
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
import { CheckOutlined } from '@ant-design/icons-vue';
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
  uiTheme: 'forest',
});
const themeOptions = [
  {
    key: 'forest',
    name: '松林',
    description: '沉稳绿调，适合长期阅读',
    preview: 'linear-gradient(135deg, #0f3d3a 0%, #24776d 48%, #dfe9df 100%)',
  },
  {
    key: 'sky',
    name: '晴空',
    description: '清爽蓝调，界面更轻快',
    preview: 'linear-gradient(135deg, #153a66 0%, #2563b8 52%, #e2edf8 100%)',
  },
  {
    key: 'grape',
    name: '葡萄',
    description: '柔和紫调，突出层次感',
    preview: 'linear-gradient(135deg, #3d2a55 0%, #7c4d9f 50%, #eee7f3 100%)',
  },
  {
    key: 'ember',
    name: '暖木',
    description: '温暖棕红，适合低刺激办公',
    preview: 'linear-gradient(135deg, #56301f 0%, #b45325 48%, #f4e9dd 100%)',
  },
  {
    key: 'graphite',
    name: '石墨',
    description: '中性灰蓝，更克制耐看',
    preview: 'linear-gradient(135deg, #1f2937 0%, #42546b 48%, #e6edf2 100%)',
  },
  {
    key: 'qinghua',
    name: '青花',
    description: '瓷白青蓝，清雅利落',
    preview: 'linear-gradient(135deg, #122b3d 0%, #1f5f8b 50%, #edf3ee 100%)',
  },
  {
    key: 'cinnabar',
    name: '朱砂',
    description: '朱红宣纸，稳重醒目',
    preview: 'linear-gradient(135deg, #2e211f 0%, #b43b2d 50%, #f3e8de 100%)',
  },
  {
    key: 'ink',
    name: '水墨',
    description: '墨灰米白，安静耐看',
    preview: 'linear-gradient(135deg, #20262a 0%, #3f4b55 50%, #eee9dd 100%)',
  },
  {
    key: 'daishan',
    name: '黛山',
    description: '青黛山色，温润清爽',
    preview: 'linear-gradient(135deg, #182b2d 0%, #2f6f68 50%, #dfece5 100%)',
  },
];

const avatarSrc = computed(() => auth.user?.avatarUrl || undefined);
const displayName = computed(() => auth.user?.nickname || auth.user?.username || '用户');
const avatarFallback = computed(() => displayName.value.slice(0, 1).toUpperCase());

onMounted(resetForm);

function resetForm() {
  form.nickname = auth.user?.nickname || '';
  form.bio = auth.user?.bio || '';
  form.uiTheme = normalizeTheme(auth.user?.uiTheme);
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

function normalizeTheme(value?: string | null) {
  return themeOptions.some((theme) => theme.key === value) ? String(value) : 'forest';
}
</script>
