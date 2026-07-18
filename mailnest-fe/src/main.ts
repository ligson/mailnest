import { createApp } from 'vue';
import Antd from 'ant-design-vue';
import { createPinia } from 'pinia';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import 'ant-design-vue/dist/reset.css';
import './styles.css';
import App from './App.vue';
import { router } from './router';

dayjs.locale('zh-cn');

createApp(App).use(createPinia()).use(router).use(Antd).mount('#app');
