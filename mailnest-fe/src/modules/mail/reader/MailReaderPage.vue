<template>
  <DashboardView :reader-message-id="messageId" />
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import DashboardView from '../../../views/DashboardView.vue';

const route = useRoute();
const messageId = computed(() => String(route.params.messageId || ''));
</script>
