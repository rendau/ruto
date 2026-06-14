<script setup lang="ts">
import { NTag } from "naive-ui";
import { formatUnixAge } from "@/lib/datetime";
import { formatBytes } from "@/lib/format";
import type { GatewayStateItem } from "@/api/types";

withDefaults(
  defineProps<{
    gateway: GatewayStateItem;
    current: boolean;
    behindText?: string;
    behindType?: "warning" | "default";
    showLastSeen?: boolean;
  }>(),
  { behindText: "behind", behindType: "warning", showLastSeen: false }
);
</script>

<template>
  <div class="gw-card">
    <div class="gw-card__head">
      <div class="gw-card__title">
        <span class="gw-card__id mono">{{ gateway.gateway_id || "—" }}</span>
        <span v-if="gateway.host_name" class="gw-card__host muted">{{ gateway.host_name }}</span>
      </div>
      <NTag
        size="small"
        :bordered="false"
        :type="
          gateway.status === 'online' ? 'success' : gateway.status === 'stale' ? 'warning' : 'error'
        "
      >
        {{ gateway.status }}
      </NTag>
    </div>
    <dl class="gw-card__meta">
      <div class="gw-card__item">
        <dt class="gw-card__term">Applied</dt>
        <dd class="gw-card__desc">
          <NTag size="small" :bordered="false" :type="current ? 'success' : behindType">
            {{ current ? "current" : behindText }}
          </NTag>
        </dd>
      </div>
      <div class="gw-card__item">
        <dt class="gw-card__term">Applied age</dt>
        <dd class="gw-card__desc">{{ formatUnixAge(gateway.last_apply_at_unix) }}</dd>
      </div>
      <div v-if="showLastSeen" class="gw-card__item">
        <dt class="gw-card__term">Last seen</dt>
        <dd class="gw-card__desc">{{ formatUnixAge(gateway.last_seen_at_unix) }}</dd>
      </div>
      <div class="gw-card__item">
        <dt class="gw-card__term">Resources</dt>
        <dd class="gw-card__desc">
          {{ formatBytes(gateway.memory_alloc_bytes) }} · {{ gateway.goroutines_count }} gr
        </dd>
      </div>
    </dl>
  </div>
</template>

<style scoped>
.gw-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 11px;
  background: var(--c-surface);
}

.gw-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.gw-card__title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.gw-card__id {
  font-size: 13.5px;
  overflow-wrap: anywhere;
}

.gw-card__host {
  font-size: 11.5px;
  overflow-wrap: anywhere;
}

.gw-card__meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 14px;
  margin: 0;
}

.gw-card__item {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.gw-card__term {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--c-text-3);
}

.gw-card__desc {
  margin: 0;
  font-size: 13px;
  color: var(--c-text-2);
  overflow-wrap: anywhere;
}
</style>
