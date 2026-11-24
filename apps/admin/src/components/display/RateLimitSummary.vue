<script setup lang="ts">
import { computed } from "vue";
import { NTag } from "naive-ui";
import { RATE_LIMIT_KEY_LABEL } from "@/constants/enums";
import type { RateLimit } from "@/api/types";

const props = defineProps<{
  rateLimit: RateLimit;
  // The inherited view: rules carry their scope and the mode no longer matters.
  effective?: boolean;
}>();

const emptyText = computed(() => {
  if (props.effective) return "No limits apply.";
  if (props.rateLimit.mode === "replace") return "No limits from this level down.";
  return "No rules of its own — the inherited rules apply.";
});

function scopeLabel(scope: string): string {
  if (scope === "root") return "root";
  if (scope.startsWith("app:")) return "app";
  if (scope.startsWith("endpoint:")) return "this endpoint";
  return scope;
}
</script>

<template>
  <div class="rate-limit-summary">
    <div v-if="!effective" class="rate-limit-summary__row">
      <span class="rate-limit-summary__label section-label">Mode</span>
      <NTag size="tiny" :bordered="false" round>{{ rateLimit.mode }}</NTag>
    </div>
    <div v-if="rateLimit.rules.length === 0" class="muted rate-limit-summary__empty">
      {{ emptyText }}
    </div>
    <div v-for="(rule, index) in rateLimit.rules" :key="index" class="rate-limit-summary__row">
      <NTag size="tiny" type="primary" :bordered="false">{{ rule.requests }} / {{ rule.period }}</NTag>
      <span class="rate-limit-summary__detail">{{ RATE_LIMIT_KEY_LABEL[rule.key] }}</span>
      <span v-if="rule.name" class="rate-limit-summary__detail mono">{{ rule.name }}</span>
      <NTag v-if="rule.scope" size="tiny" :bordered="false" round>from {{ scopeLabel(rule.scope) }}</NTag>
    </div>
  </div>
</template>

<style scoped>
.rate-limit-summary {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rate-limit-summary__row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.rate-limit-summary__label {
  margin-right: 2px;
}

.rate-limit-summary__detail {
  font-size: 12.5px;
  color: var(--c-text-2);
  overflow-wrap: anywhere;
}

.rate-limit-summary__empty {
  font-size: 13px;
}
</style>
