<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NAutoComplete, NButton, NIcon, NInput, NInputNumber, NSelect } from "naive-ui";
import { AddOutline } from "@vicons/ionicons5";
import { emptyRateLimitRule, normalizeRateLimit } from "@/api/normalize";
import { RATE_LIMIT_MAX_REQUESTS, rateLimitRuleErrors } from "@/lib/rateLimit";
import {
  RATE_LIMIT_KEY_LABEL,
  RATE_LIMIT_KEY_OPTIONS,
  RATE_LIMIT_MODE_OPTIONS,
  RATE_LIMIT_PERIOD_PRESETS
} from "@/constants/enums";
import RecordCard from "@/components/common/RecordCard.vue";
import type { RateLimit, RateLimitKey, RateLimitMode, RateLimitRule } from "@/api/types";

const props = defineProps<{
  modelValue: RateLimit;
  // Root has nothing to inherit from: no mode, only its rules.
  root?: boolean;
}>();

const emit = defineEmits<{ "update:modelValue": [value: RateLimit] }>();

const local = ref<RateLimit>(normalizeRateLimit(props.modelValue));
const ruleKeys = ref<string[]>([]);
const pushingToParent = ref(false);
let nextRuleKey = 0;

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent.value) {
      pushingToParent.value = false;
      return;
    }
    local.value = normalizeRateLimit(value);
    syncRuleKeys();
  },
  { deep: true, immediate: true }
);

watch(
  local,
  (value) => {
    pushingToParent.value = true;
    emit("update:modelValue", normalizeRateLimit(value));
  },
  { deep: true }
);

const ruleErrors = computed(() => local.value.rules.map(rateLimitRuleErrors));

const emptyText = computed(() => {
  if (props.root) return "No rules — requests are not limited by default.";
  if (local.value.mode === "replace") {
    return "No rules — with replace no limits apply from this level down.";
  }
  return "No rules of its own — the inherited rules apply.";
});

function syncRuleKeys(): void {
  const count = local.value.rules.length;
  if (ruleKeys.value.length < count) {
    ruleKeys.value = [
      ...ruleKeys.value,
      ...Array.from({ length: count - ruleKeys.value.length }, () => {
        nextRuleKey += 1;
        return `rate-limit-rule-${nextRuleKey}`;
      })
    ];
  } else if (ruleKeys.value.length > count) {
    ruleKeys.value = ruleKeys.value.slice(0, count);
  }
}

function setMode(value: RateLimitMode): void {
  local.value = { ...local.value, mode: value };
}

function addRule(): void {
  local.value = { ...local.value, rules: [...local.value.rules, emptyRateLimitRule()] };
  syncRuleKeys();
}

function removeRule(ruleIndex: number): void {
  local.value = { ...local.value, rules: local.value.rules.filter((_, idx) => idx !== ruleIndex) };
  ruleKeys.value = ruleKeys.value.filter((_, idx) => idx !== ruleIndex);
}

function updateRule(ruleIndex: number, part: Partial<RateLimitRule>): void {
  local.value = {
    ...local.value,
    rules: local.value.rules.map((rule, idx) => (idx === ruleIndex ? { ...rule, ...part } : rule))
  };
}

// All presets while the field is empty or holds one of them, else those starting with the input.
function periodOptions(current: string) {
  const query = current.trim();
  const presets =
    !query || RATE_LIMIT_PERIOD_PRESETS.includes(query)
      ? RATE_LIMIT_PERIOD_PRESETS
      : RATE_LIMIT_PERIOD_PRESETS.filter((item) => item.startsWith(query));
  return presets.map((item) => ({ label: item, value: item }));
}

function ruleErrorText(ruleIndex: number): string {
  const errors = ruleErrors.value[ruleIndex];
  return errors ? [errors.requests, errors.period].filter(Boolean).join("; ") : "";
}

function ruleSummary(rule: RateLimitRule): string {
  return `${rule.requests} / ${rule.period.trim() || "?"} · ${RATE_LIMIT_KEY_LABEL[rule.key]}`;
}
</script>

<template>
  <div class="rate-limit-editor">
    <div v-if="!root" class="rate-limit-editor__head">
      <label class="field">
        <span class="field__label">Mode</span>
        <NSelect
          :value="local.mode"
          :options="RATE_LIMIT_MODE_OPTIONS"
          @update:value="(value: RateLimitMode) => setMode(value)"
        />
      </label>
    </div>

    <p v-if="local.rules.length === 0" class="rate-limit-editor__empty muted">{{ emptyText }}</p>

    <div class="rate-limit-editor__list">
      <RecordCard
        v-for="(rule, ruleIndex) in local.rules"
        :key="ruleKeys[ruleIndex] || ruleIndex"
        remove-label="Remove rule"
        @remove="removeRule(ruleIndex)"
      >
        <template #head>
          <span class="muted">{{ ruleIndex + 1 }}</span>
          <span class="rule__name" :class="{ muted: !rule.name.trim() }">
            {{ rule.name.trim() || "no name" }}
          </span>
          <span class="rule__summary mono">{{ ruleSummary(rule) }}</span>
        </template>
        <div class="rule-row">
          <label class="field">
            <span class="field__label">Name (optional, shown in logs)</span>
            <NInput
              :value="rule.name"
              placeholder="e.g. login"
              @update:value="(value: string) => updateRule(ruleIndex, { name: value })"
            />
          </label>
          <label class="field">
            <span class="field__label">Requests</span>
            <NInputNumber
              :value="rule.requests"
              :min="1"
              :max="RATE_LIMIT_MAX_REQUESTS"
              :status="ruleErrors[ruleIndex]?.requests ? 'error' : undefined"
              @update:value="(value: number | null) => updateRule(ruleIndex, { requests: Math.trunc(value || 0) })"
            />
          </label>
          <label class="field">
            <span class="field__label">Per period</span>
            <NAutoComplete
              :value="rule.period"
              :options="periodOptions(rule.period)"
              :get-show="() => true"
              :status="ruleErrors[ruleIndex]?.period ? 'error' : undefined"
              placeholder="1m"
              @update:value="(value: string | null) => updateRule(ruleIndex, { period: value || '' })"
            />
          </label>
          <label class="field">
            <span class="field__label">Counted</span>
            <NSelect
              :value="rule.key"
              :options="RATE_LIMIT_KEY_OPTIONS"
              @update:value="(value: RateLimitKey) => updateRule(ruleIndex, { key: value })"
            />
          </label>
        </div>
        <p v-if="ruleErrorText(ruleIndex)" class="rule__error">{{ ruleErrorText(ruleIndex) }}</p>
      </RecordCard>
      <NButton dashed block @click="addRule">
        <template #icon><NIcon :component="AddOutline" /></template>
        Add rule
      </NButton>
    </div>

    <p class="rate-limit-editor__hint muted">
      A rule lets through no more than <em>requests</em> per <em>period</em> (whole seconds from
      <code>1s</code> to <code>24h</code>, e.g. <code>10s</code>, <code>1m</code>, <code>1h</code>):
      per client IP, per token (the bearer token or the basic-auth user; a request without one
      counts by IP — a made-up token gets its own counter, so pair it with an IP rule) or for all requests together, which protects the backend as a whole. A rule
      counts on the level it is written on: a Root rule — a client's requests across the whole
      gateway, an application rule — across the application, an endpoint rule — to that endpoint.
      A request passes only if every rule that reaches it allows it; otherwise the client gets
      <code>429</code> with <code>Retry-After</code> (gRPC: <code>RESOURCE_EXHAUSTED</code>).
    </p>
    <p v-if="!root" class="rate-limit-editor__hint muted">
      <strong>extend</strong> adds these rules to the inherited ones; <strong>replace</strong> keeps
      only the rules set here, and replace with no rules turns limits off from this level down
      (e.g. for an internal heavy application).
    </p>
    <p v-else class="rate-limit-editor__hint muted">
      Every gateway copy counts on its own, so with several copies running the real ceiling can be
      up to that many times higher.
    </p>
  </div>
</template>

<style scoped>
/* the editor also sits in narrow drawers, so the rule row follows its own width */
.rate-limit-editor {
  container-type: inline-size;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  background: var(--c-surface-2);
}

.rate-limit-editor__head {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--c-border);
}

.rate-limit-editor__head .field {
  min-width: 280px;
}

.rate-limit-editor__empty {
  margin: 0;
  font-size: 13px;
}

.rate-limit-editor__list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rule__name {
  font-weight: 600;
}

.rule__summary {
  font-size: 12.5px;
  color: var(--c-text-2);
}

.rule-row {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(0, 1.2fr) minmax(0, 1fr) minmax(0, 1.6fr);
  gap: 10px;
}

@container (max-width: 520px) {
  .rule-row {
    grid-template-columns: 1fr 1fr;
  }
}

.rule__error {
  margin: 0;
  font-size: 12px;
  color: var(--c-error);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 7px;
  min-width: 0;
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.rate-limit-editor__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
}

.rate-limit-editor__hint code {
  font-family: var(--font-mono);
  color: var(--c-text-2);
}

@media (max-width: 560px) {
  .rate-limit-editor__head .field {
    min-width: 0;
    flex: 1;
  }
}
</style>
