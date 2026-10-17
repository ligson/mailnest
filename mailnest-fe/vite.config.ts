import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import Components from 'unplugin-vue-components/vite';
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers';
import { version } from './package.json';

export default defineConfig({
  define: {
    __APP_BUILD_INFO__: JSON.stringify({
      version: process.env.APP_VERSION?.trim() || version,
      commit: process.env.VCS_REF?.trim() || 'unknown',
      buildTime: process.env.BUILD_TIME?.trim() || new Date().toISOString(),
    }),
  },
  plugins: [
    vue(),
    Components({
      dirs: [],
      dts: false,
      resolvers: [AntDesignVueResolver({ importStyle: false })],
    }),
  ],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
});
