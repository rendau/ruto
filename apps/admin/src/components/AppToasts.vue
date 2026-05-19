<script setup lang="ts">
import { useNotices } from "../lib/notify";

const notices = useNotices();
</script>

<template>
  <TransitionGroup name="toast" tag="div" class="toasts">
    <div v-for="item in notices.items" :key="item.id" class="toast" :class="item.kind">
      {{ item.message }}
    </div>
  </TransitionGroup>
</template>
