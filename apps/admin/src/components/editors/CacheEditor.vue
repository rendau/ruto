<script setup lang="ts">
import { ref, watch } from "vue";
import { NCheckbox, NDynamicTags, NInputNumber, NRadioButton, NRadioGroup } from "naive-ui";
import { normalizeCache } from "@/api/normalize";
import SwitchField from "@/components/common/SwitchField.vue";
import type { CacheBrowser, CacheScope, CacheServer, EndpointCache } from "@/api/types";

const props = defineProps<{ modelValue: EndpointCache }>();

const emit = defineEmits<{ "update:modelValue": [value: EndpointCache] }>();

const local = ref<EndpointCache>(normalizeCache(props.modelValue));
const pushingToParent = ref(false);

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent.value) {
      pushingToParent.value = false;
      return;
    }
    local.value = normalizeCache(value);
  },
  { deep: true, immediate: true }
);

watch(
  local,
  (value) => {
    pushingToParent.value = true;
    emit("update:modelValue", normalizeCache(value));
  },
  { deep: true }
);

function patchServer(part: Partial<CacheServer>): void {
  local.value = { ...local.value, server: { ...local.value.server, ...part } };
}

function patchBrowser(part: Partial<CacheBrowser>): void {
  local.value = { ...local.value, browser: { ...local.value.browser, ...part } };
}

function toInt(value: number | null): number {
  return Math.max(0, Math.trunc(value || 0));
}
</script>

<template>
  <div class="cache-editor">
    <section class="cache-editor__block">
      <SwitchField
        :model-value="local.server.enabled"
        label="Gateway cache"
        @update:model-value="(value: boolean) => patchServer({ enabled: value })"
      />
      <p class="cache-editor__hint muted">
        The gateway keeps <code>200</code> responses to <code>GET</code> requests and answers repeats
        itself. Requests differ by path, query and <code>Accept-Encoding</code>; with authentication on,
        also by the caller's credentials unless the cache is shared. Responses with <code>Set-Cookie</code> or
        <code>Cache-Control: private</code> / <code>no-store</code> are never kept.
      </p>
      <template v-if="local.server.enabled">
        <div class="cache-editor__row">
          <label class="field">
            <span class="field__label">Keep for (seconds)</span>
            <NInputNumber
              :value="local.server.ttl"
              :min="1"
              :step="30"
              @update:value="(value: number | null) => patchServer({ ttl: toInt(value) })"
            />
          </label>
          <label class="field">
            <span class="field__label">Max response size (bytes, 0 = 1 MB)</span>
            <NInputNumber
              :value="local.server.max_body_size"
              :min="0"
              :step="65536"
              @update:value="(value: number | null) => patchServer({ max_body_size: toInt(value) })"
            />
          </label>
        </div>
        <div class="field">
          <NCheckbox
            :checked="local.server.shared"
            @update:checked="(value: boolean) => patchServer({ shared: value })"
          >
            Shared for all users
          </NCheckbox>
          <span class="cache-editor__hint muted">
            One cached response for every caller: credentials don't split the cache. Authentication still
            runs on every request. Use only when the response is the same whoever asks.
          </span>
        </div>
        <div class="field">
          <span class="field__label">Also differ by request headers</span>
          <NDynamicTags
            :value="local.server.vary_headers"
            @update:value="(value: string[]) => patchServer({ vary_headers: value })"
          />
          <span class="cache-editor__hint muted">
            E.g. <code>Accept-Language</code>, or <code>Cookie</code> when the backend tells users apart by
            a cookie.
          </span>
        </div>
      </template>
    </section>

    <section class="cache-editor__block">
      <SwitchField
        :model-value="local.browser.enabled"
        label="Browser cache"
        @update:model-value="(value: boolean) => patchBrowser({ enabled: value })"
      />
      <p class="cache-editor__hint muted">
        The gateway sets <code>Cache-Control</code> on <code>200</code> responses to <code>GET</code> /
        <code>HEAD</code> requests; the client keeps the response and does not ask again until it expires.
      </p>
      <template v-if="local.browser.enabled">
        <div class="cache-editor__row">
          <label class="field">
            <span class="field__label">Who may keep it</span>
            <NRadioGroup
              :value="local.browser.scope"
              @update:value="(value: CacheScope) => patchBrowser({ scope: value })"
            >
              <NRadioButton value="private" title="Only the user's browser">private</NRadioButton>
              <NRadioButton value="public" title="Browsers and shared caches (CDN, proxies)">public</NRadioButton>
              <NRadioButton value="no-store" title="Nobody keeps the response">no-store</NRadioButton>
            </NRadioGroup>
          </label>
          <label v-if="local.browser.scope !== 'no-store'" class="field">
            <span class="field__label">Max age (seconds)</span>
            <NInputNumber
              :value="local.browser.max_age"
              :min="0"
              :step="60"
              @update:value="(value: number | null) => patchBrowser({ max_age: toInt(value) })"
            />
          </label>
        </div>
        <NCheckbox
          :checked="local.browser.override"
          @update:checked="(value: boolean) => patchBrowser({ override: value })"
        >
          Replace the backend's own Cache-Control (otherwise set only when the backend sent none)
        </NCheckbox>
      </template>
    </section>
  </div>
</template>

<style scoped>
.cache-editor {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  background: var(--c-surface-2);
}

.cache-editor__block {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.cache-editor__block + .cache-editor__block {
  padding-top: 14px;
  border-top: 1px solid var(--c-border);
}

.cache-editor__row {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 7px;
  min-width: 200px;
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.cache-editor__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
}

.cache-editor__hint code {
  font-family: var(--font-mono);
  color: var(--c-text-2);
}
</style>
