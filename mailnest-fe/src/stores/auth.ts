import { defineStore } from 'pinia';
import { authApi, tokenStorageKey, type User } from '../api/client';

interface AuthState {
  token: string;
  user: User | null;
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: localStorage.getItem(tokenStorageKey) || '',
    user: null,
  }),
  getters: {
    isLoggedIn: (state) => Boolean(state.token),
  },
  actions: {
    setSession(token: string, user: User) {
      this.token = token;
      this.user = user;
      localStorage.setItem(tokenStorageKey, token);
    },
    async register(payload: { username: string; email: string; password: string }) {
      const data = await authApi.register(payload);
      this.setSession(data.token, data.user);
    },
    async login(payload: { account: string; password: string }) {
      const data = await authApi.login(payload);
      this.setSession(data.token, data.user);
    },
    async loadMe() {
      if (!this.token) {
        return;
      }
      this.user = await authApi.me();
    },
    async logout() {
      if (this.token) {
        await authApi.logout().catch(() => undefined);
      }
      this.token = '';
      this.user = null;
      localStorage.removeItem(tokenStorageKey);
    },
  },
});
