<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(defineProps<{ method: string; grpc?: boolean }>(), { grpc: false });

const label = computed(() => (props.grpc ? "gRPC" : (props.method || "").toUpperCase() || "—"));

const kind = computed(() => {
  if (props.grpc) return "grpc";
  const method = (props.method || "").trim().toUpperCase();
  switch (method) {
    case "GET":
      return "get";
    case "POST":
      return "post";
    case "PUT":
      return "put";
    case "PATCH":
      return "patch";
    case "DELETE":
      return "delete";
    default:
      return "default";
  }
});
</script>

<template>
  <span class="method-badge mono" :class="`method-badge--${kind}`">{{ label }}</span>
</template>

<style scoped>
.method-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 52px;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.03em;
  line-height: 1.5;
}

.method-badge--get {
  background: rgba(67, 201, 139, 0.15);
  color: var(--c-success);
}

.method-badge--post {
  background: rgba(91, 130, 240, 0.16);
  color: var(--c-primary);
}

.method-badge--put {
  background: rgba(232, 178, 58, 0.16);
  color: var(--c-warning);
}

.method-badge--patch {
  background: rgba(34, 211, 197, 0.14);
  color: var(--c-teal);
}

.method-badge--delete {
  background: rgba(239, 111, 114, 0.16);
  color: var(--c-error);
}

.method-badge--grpc {
  background: rgba(34, 211, 197, 0.14);
  color: var(--c-teal);
}

.method-badge--default {
  background: rgba(124, 136, 155, 0.16);
  color: var(--c-text-2);
}
</style>
