<script setup lang="ts">
import { ref, watch } from "vue";
import { NButton, NIcon, NInput, NInputNumber, NModal } from "naive-ui";
import { CopyOutline, HelpCircleOutline } from "@vicons/ionicons5";
import { normalizeTransform } from "@/api/normalize";
import { useClipboard } from "@/composables/useClipboard";
import JsonBlock from "@/components/common/JsonBlock.vue";
import type { Transform } from "@/api/types";

const props = defineProps<{ modelValue: Transform }>();
const emit = defineEmits<{ "update:modelValue": [value: Transform] }>();

const { copy } = useClipboard();

const local = ref<Transform>(normalizeTransform(props.modelValue));
const pushingToParent = ref(false);
const showDoc = ref(false);

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent.value) {
      pushingToParent.value = false;
      return;
    }
    local.value = normalizeTransform(value);
  },
  { deep: true, immediate: true }
);

watch(
  local,
  (value) => {
    pushingToParent.value = true;
    emit("update:modelValue", normalizeTransform(value));
  },
  { deep: true }
);

function patch(part: Partial<Transform>): void {
  local.value = { ...local.value, ...part };
}

const SCRIPT_PLACEHOLDER = `// req: { method, path, headers, params, body, raw_body, vars }
// return any subset of { method, path, headers, params, body }
return {
  body: { ...req.body, source: "gateway" },
  headers: { ...req.headers, "X-Tenant": [req.vars.tenant] }
};`;

const INPUT_FIELDS = [
  { name: "req.method", desc: "HTTP method, e.g. \"POST\"." },
  { name: "req.path", desc: "Request path, e.g. \"/users/42\"." },
  { name: "req.headers", desc: "Multi-value headers: { name: [values] }." },
  { name: "req.params", desc: "Multi-value query params: { name: [values] }." },
  { name: "req.body", desc: "Parsed JSON body, or undefined if empty / not JSON." },
  { name: "req.raw_body", desc: "Raw request body as a string." },
  { name: "req.vars", desc: "Endpoint variables: { name: value } (strings)." }
];

const OUTPUT_FIELDS = [
  { name: "method", desc: "String — overrides the request method." },
  { name: "path", desc: "String — overrides the request path." },
  { name: "headers", desc: "Object — replaces ALL headers (spread req.headers to keep). A value may be a list or a bare string." },
  { name: "params", desc: "Object — replaces ALL query params (same value rules as headers)." },
  { name: "body", desc: "Object → JSON-encoded; string → sent as-is; null → empty body." }
];

const EXAMPLE_CODE = `// Wrap the incoming JSON in the backend's envelope,
// add a header, and drop an internal one.
const headers = { ...req.headers };
delete headers["X-Internal"];

return {
  headers: { ...headers, "X-Tenant": [req.vars.tenant] },
  body: {
    source: "gateway",
    received_at: req.vars.now,
    payload: req.body,        // parsed JSON body (undefined if not JSON)
  },
};`;

// Full reference as markdown — copied as a single block to hand to an AI agent.
const DOC_MARKDOWN = [
  "# ruto gateway — request transform script",
  "",
  "Per-endpoint JavaScript evaluated by the gateway (goja) to reshape the incoming",
  "HTTP request before it is proxied to the backend. The script body runs with a",
  "`req` object in scope and must `return` an object describing the outgoing request.",
  "",
  "## Input: `req`",
  ...INPUT_FIELDS.map((f) => `- \`${f.name}\` — ${f.desc}`),
  "",
  "## Output: return an object with any subset of these fields",
  ...OUTPUT_FIELDS.map((f) => `- \`${f.name}\` — ${f.desc}`),
  "",
  "## Rules",
  "- A field that is not returned is proxied unchanged.",
  "- Returning a non-object, or throwing an error, fails the request without calling the backend.",
  "- The script runs after authentication, last before proxying.",
  "- `req.vars` are resolved at request time; the script source itself is not interpolated.",
  "",
  "## Example",
  "```js",
  EXAMPLE_CODE,
  "```",
  ""
].join("\n");
</script>

<template>
  <div class="transform-editor">
    <label class="field">
      <span class="field__head">
        <span class="field__label">Request script (JavaScript)</span>
        <NButton size="tiny" quaternary @click="showDoc = true">
          <template #icon><NIcon :component="HelpCircleOutline" /></template>
          Docs
        </NButton>
      </span>
      <NInput
        type="textarea"
        class="transform-editor__code"
        :value="local.request"
        :placeholder="SCRIPT_PLACEHOLDER"
        :autosize="{ minRows: 8, maxRows: 24 }"
        spellcheck="false"
        @update:value="(value: string) => patch({ request: value })"
      />
    </label>

    <label class="field transform-editor__workers">
      <span class="field__label">Max workers (0 = inherit default)</span>
      <NInputNumber
        :value="local.max_workers"
        :min="0"
        :step="1"
        @update:value="(value: number | null) => patch({ max_workers: Math.max(0, Math.trunc(value || 0)) })"
      />
    </label>

    <p class="transform-editor__hint muted">
      Runs after auth, last before proxying. Omitted return fields are proxied unchanged.
      Throwing fails the request without calling the backend.
      <NButton text size="tiny" type="primary" @click="showDoc = true">Open reference</NButton>
    </p>

    <NModal
      :show="showDoc"
      preset="card"
      title="Request transform reference"
      class="transform-doc"
      :bordered="false"
      @update:show="(value: boolean) => (showDoc = value)"
    >
      <template #header-extra>
        <NButton size="small" secondary @click="copy(DOC_MARKDOWN, 'Documentation copied')">
          <template #icon><NIcon :component="CopyOutline" /></template>
          Copy for AI
        </NButton>
      </template>

      <div class="doc">
        <section class="doc__section">
          <span class="section-label">Input — <code>req</code></span>
          <dl class="doc__list">
            <template v-for="f in INPUT_FIELDS" :key="f.name">
              <dt><code>{{ f.name }}</code></dt>
              <dd>{{ f.desc }}</dd>
            </template>
          </dl>
        </section>

        <section class="doc__section">
          <span class="section-label">Output — <code>return</code> any subset</span>
          <dl class="doc__list">
            <template v-for="f in OUTPUT_FIELDS" :key="f.name">
              <dt><code>{{ f.name }}</code></dt>
              <dd>{{ f.desc }}</dd>
            </template>
          </dl>
          <p class="doc__note muted">
            A field that is not returned is proxied unchanged. Returning a non-object,
            or throwing, fails the request without calling the backend.
          </p>
        </section>

        <section class="doc__section">
          <span class="section-label">Example</span>
          <JsonBlock :content="EXAMPLE_CODE" max-height="320px" />
        </section>
      </div>
    </NModal>
  </div>
</template>

<style scoped>
.transform-editor {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  background: var(--c-surface-2);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.field__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.transform-editor__code :deep(textarea) {
  font-family: var(--font-mono);
  font-size: 12.5px;
  line-height: 1.5;
}

.transform-editor__workers {
  max-width: 240px;
}

.transform-editor__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
}

:global(.transform-doc) {
  width: min(680px, calc(100vw - 48px));
}

.doc {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.doc__section {
  display: flex;
  flex-direction: column;
  gap: 9px;
}

.doc__list {
  display: grid;
  grid-template-columns: minmax(120px, max-content) 1fr;
  gap: 6px 14px;
  margin: 0;
}

.doc__list dt {
  font-family: var(--font-mono);
}

.doc__list dt code {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--c-text);
}

.doc__list dd {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.45;
  color: var(--c-text-2);
}

.doc__note {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
}

.section-label code {
  font-family: var(--font-mono);
  color: var(--c-text-2);
}

@media (max-width: 560px) {
  .doc__list {
    grid-template-columns: 1fr;
    gap: 2px 0;
  }
  .doc__list dd {
    margin-bottom: 8px;
  }
}
</style>
