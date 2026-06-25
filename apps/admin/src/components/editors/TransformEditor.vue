<script setup lang="ts">
import { computed, ref, watch } from "vue";
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
const docKey = ref<string | null>(null);

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

const REQUEST_PLACEHOLDER = `// req: { method, path, headers, params, body, raw_body, vars }
// return any subset of { method, headers, params, body }
return {
  body: { ...req.body, source: "gateway" },
  headers: { ...req.headers, "X-Tenant": [req.vars.tenant] }
};`;

const RESPONSE_PLACEHOLDER = `// res: { status, headers, body, raw_body, vars }
// return any subset of { status, headers, body }
if (res.status === 404) return { status: 200, body: { items: [] } };
return { body: res.body.data };`;

interface Field {
  name: string;
  desc: string;
}
interface Contract {
  key: string;
  title: string;
  note: string;
  inputs: Field[];
  outputs: Field[];
  example: string;
}

const CONTRACTS: Contract[] = [
  {
    key: "request",
    title: "Request transform — req → backend",
    note: "Runs after authentication, last before proxying to the backend.",
    inputs: [
      { name: "req.method", desc: "HTTP method, e.g. \"POST\"." },
      { name: "req.path", desc: "Request path, e.g. \"/users/42\"." },
      { name: "req.headers", desc: "Multi-value headers: { name: [values] }." },
      { name: "req.params", desc: "Multi-value query params: { name: [values] }." },
      { name: "req.body", desc: "Parsed JSON body, or undefined if empty / not JSON." },
      { name: "req.raw_body", desc: "Raw request body as a string." },
      { name: "req.vars", desc: "Endpoint variables: { name: value } (strings)." }
    ],
    outputs: [
      { name: "method", desc: "String — overrides the request method." },
      { name: "headers", desc: "Object — replaces ALL headers (spread req.headers to keep). Value may be a list or a bare string." },
      { name: "params", desc: "Object — replaces ALL query params (same value rules)." },
      { name: "body", desc: "Object → JSON-encoded; string → sent as-is; null → empty body." }
    ],
    example: `// Wrap the incoming JSON in the backend's envelope,
// add a header, and drop an internal one.
const headers = { ...req.headers };
delete headers["X-Internal"];

return {
  headers: { ...headers, "X-Tenant": [req.vars.tenant] },
  body: {
    source: "gateway",
    payload: req.body,        // parsed JSON body (undefined if not JSON)
  },
};`
  },
  {
    key: "response",
    title: "Response transform — backend → client",
    note: "Runs on the backend response (after the request reached the backend) before it is returned to the client.",
    inputs: [
      { name: "res.status", desc: "Backend HTTP status code, e.g. 200." },
      { name: "res.headers", desc: "Multi-value headers: { name: [values] }." },
      { name: "res.body", desc: "Parsed JSON body, or undefined if empty / not JSON." },
      { name: "res.raw_body", desc: "Raw response body as a string." },
      { name: "res.vars", desc: "Endpoint variables: { name: value } (strings)." }
    ],
    outputs: [
      { name: "status", desc: "Number — overrides the response status code." },
      { name: "headers", desc: "Object — replaces ALL headers (spread res.headers to keep). Value may be a list or a bare string." },
      { name: "body", desc: "Object → JSON-encoded; string → sent as-is; null → empty body." }
    ],
    example: `// Unwrap the backend envelope, and turn a 404 into an empty list.
if (res.status === 404) {
  return { status: 200, body: { items: [] } };
}
return {
  headers: { ...res.headers, "X-Served-By": ["gateway"] },
  body: res.body.data,
};`
  }
];

const activeContract = computed(() => CONTRACTS.find((c) => c.key === docKey.value) ?? null);

// Self-contained markdown for one contract — copied as a single block to hand to
// an AI agent.
function contractMarkdown(c: Contract): string {
  const failNote =
    c.key === "request"
      ? "fails the request without calling the backend"
      : "fails the response with a 502 (the backend reply was already consumed)";
  return [
    `# ruto gateway — ${c.title}`,
    "",
    c.note,
    "",
    "Per-endpoint JavaScript evaluated by the gateway (goja). The script body runs",
    "with the input object below in scope and must `return` an object describing the",
    `result. A field that is not returned is passed through unchanged. Returning a`,
    `non-object, or throwing, ${failNote}. \`headers\`/\`params\` are multi-value`,
    "`{ name: [values] }`; a returned object REPLACES the whole set (spread the input",
    "to keep existing entries). A returned `body` object is JSON-encoded, a string is",
    "sent as-is, null means empty. `vars` are resolved at request time; the script",
    "source itself is not interpolated.",
    "",
    "## Input",
    ...c.inputs.map((f) => `- \`${f.name}\` — ${f.desc}`),
    "",
    "## Output (return any subset)",
    ...c.outputs.map((f) => `- \`${f.name}\` — ${f.desc}`),
    "",
    "## Example",
    "```js",
    c.example,
    "```",
    ""
  ].join("\n");
}
</script>

<template>
  <div class="transform-editor">
    <label class="field">
      <span class="field__head">
        <span class="field__label">Request script — JavaScript (req → backend)</span>
        <NButton size="tiny" quaternary @click="docKey = 'request'">
          <template #icon><NIcon :component="HelpCircleOutline" /></template>
          Docs
        </NButton>
      </span>
      <NInput
        type="textarea"
        class="transform-editor__code"
        :value="local.request"
        :placeholder="REQUEST_PLACEHOLDER"
        :autosize="{ minRows: 7, maxRows: 22 }"
        spellcheck="false"
        @update:value="(value: string) => patch({ request: value })"
      />
    </label>

    <label class="field">
      <span class="field__head">
        <span class="field__label">Response script — JavaScript (backend → client)</span>
        <NButton size="tiny" quaternary @click="docKey = 'response'">
          <template #icon><NIcon :component="HelpCircleOutline" /></template>
          Docs
        </NButton>
      </span>
      <NInput
        type="textarea"
        class="transform-editor__code"
        :value="local.response"
        :placeholder="RESPONSE_PLACEHOLDER"
        :autosize="{ minRows: 7, maxRows: 22 }"
        spellcheck="false"
        @update:value="(value: string) => patch({ response: value })"
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
      Request runs after auth, last before proxying; response runs on the backend reply
      before the client sees it. Omitted return fields pass through unchanged; throwing
      fails the call without a backend round-trip (request) or with a 502 (response).
      <NButton text size="tiny" type="primary" @click="docKey = 'request'">Request reference</NButton>
      ·
      <NButton text size="tiny" type="primary" @click="docKey = 'response'">Response reference</NButton>
    </p>

    <NModal
      :show="activeContract !== null"
      preset="card"
      :title="activeContract?.title ?? ''"
      class="transform-doc"
      :bordered="false"
      @update:show="(value: boolean) => { if (!value) docKey = null; }"
    >
      <template #header-extra>
        <NButton
          v-if="activeContract"
          size="small"
          secondary
          @click="copy(contractMarkdown(activeContract), 'Documentation copied')"
        >
          <template #icon><NIcon :component="CopyOutline" /></template>
          Copy for AI
        </NButton>
      </template>

      <div v-if="activeContract" class="doc">
        <p class="doc__note muted">{{ activeContract.note }}</p>

        <span class="section-label">Input</span>
        <dl class="doc__list">
          <template v-for="f in activeContract.inputs" :key="f.name">
            <dt><code>{{ f.name }}</code></dt>
            <dd>{{ f.desc }}</dd>
          </template>
        </dl>

        <span class="section-label">Output — <code>return</code> any subset</span>
        <dl class="doc__list">
          <template v-for="f in activeContract.outputs" :key="f.name">
            <dt><code>{{ f.name }}</code></dt>
            <dd>{{ f.desc }}</dd>
          </template>
        </dl>

        <span class="section-label">Example</span>
        <JsonBlock :content="activeContract.example" max-height="280px" />

        <p class="doc__note muted">
          A field that is not returned passes through unchanged. Returning a non-object,
          or throwing, fails the call without sending a partial result.
        </p>
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
  gap: 10px;
}

.doc__list {
  display: grid;
  grid-template-columns: minmax(110px, max-content) 1fr;
  gap: 6px 14px;
  margin: 0;
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
