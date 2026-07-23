import { createRouter, createWebHistory } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import LoginView from '../views/LoginView.vue';
import RegisterView from '../views/RegisterView.vue';
import DashboardView from '../views/DashboardView.vue';
import AttachmentsView from '../views/AttachmentsView.vue';
import MailAccountsView from '../views/MailAccountsView.vue';
import MailRulesView from '../views/MailRulesView.vue';
import ContactsView from '../views/ContactsView.vue';
import ProfileSettingsView from '../views/ProfileSettingsView.vue';
import MicrosoftOAuthCallbackView from '../views/MicrosoftOAuthCallbackView.vue';
import AdminUsersView from '../views/AdminUsersView.vue';

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
