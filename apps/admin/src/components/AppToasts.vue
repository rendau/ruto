<script setup lang="ts">
import { dismissNotice, useNotices } from "../lib/notify";

const notices = useNotices();
</script>

<template>
  <TransitionGroup name="toast" tag="div" class="toasts">
    <div
      v-for="item in notices.items"
      :key="item.id"
      class="toast"
      :class="item.kind"
      role="button"
      tabindex="0"
      @click="dismissNotice(item.id)"
      @keydown.enter="dismissNotice(item.id)"
      @keydown.space.prevent="dismissNotice(item.id)"
    >
      {{ item.message }}
    </div>
  </TransitionGroup>
</template>
