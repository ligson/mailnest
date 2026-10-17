import { createRouter, createWebHistory } from 'vue-router';
import { useAuthStore } from '../stores/auth';
// 按路由加载业务页面，打开邮件时不下载账号、规则、管理等页面实现。
const LoginView = () => import('../views/LoginView.vue');
const RegisterView = () => import('../views/RegisterView.vue');
const DashboardView = () => import('../views/DashboardView.vue');
const AttachmentsView = () => import('../views/AttachmentsView.vue');
const MailAccountsView = () => import('../views/MailAccountsView.vue');
const MailRulesView = () => import('../views/MailRulesView.vue');
const SendLogsView = () => import('../views/SendLogsView.vue');
const ContactsView = () => import('../views/ContactsView.vue');
const ProfileSettingsView = () => import('../views/ProfileSettingsView.vue');
const MicrosoftOAuthCallbackView = () => import('../views/MicrosoftOAuthCallbackView.vue');
const AdminUsersView = () => import('../views/AdminUsersView.vue');

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/mail' },
    { path: '/login', component: LoginView, meta: { public: true } },
    { path: '/register', component: RegisterView, meta: { public: true } },
    { path: '/oauth/microsoft/callback', component: MicrosoftOAuthCallbackView },
    { path: '/mail', component: DashboardView },
    { path: '/attachments', component: AttachmentsView },
    { path: '/accounts', component: MailAccountsView },
    { path: '/contacts', component: ContactsView },
    { path: '/rules', component: MailRulesView },
    { path: '/send-logs', component: SendLogsView },
    { path: '/settings/profile', component: ProfileSettingsView },
    { path: '/admin/users', component: AdminUsersView, meta: { admin: true } },
  ],
});

router.beforeEach(async (to) => {
  const auth = useAuthStore();
  if (to.meta.public) {
    return true;
  }
  if (!auth.token) {
    return '/login';
  }
  if (!auth.user) {
    await auth.loadMe().catch(async () => {
      await auth.logout();
    });
  }
  if (to.meta.admin && !auth.user?.isAdmin) {
    return '/mail';
  }
  return auth.token ? true : '/login';
});
