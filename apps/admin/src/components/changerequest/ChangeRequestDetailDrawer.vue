<script setup lang="ts">
import { computed, ref } from "vue";
import {
  NAlert,
  NButton,
  NCheckbox,
  NDrawer,
  NDrawerContent,
  NIcon,
  NInput,
  NModal,
  NSpin,
  NTag,
  useMessage
} from "naive-ui";
import {
  CheckmarkOutline,
  CloseOutline,
  LockClosedOutline,
  TrashOutline
} from "@vicons/ionicons5";
import {
  applyChangeRequest,
  getChangeRequest,
  rejectChangeRequest
} from "@/api/changeRequest";
import { apiErrorMessage } from "@/api/http";
import { useDrawerResource } from "@/composables/useDrawerResource";
import { useConfirm } from "@/composables/useConfirm";
import { useIsMobile } from "@/composables/useIsMobile";
import { useSnapshotStore } from "@/stores/snapshot";
import { useAppsStore } from "@/stores/apps";
import { formatUnixDateTime } from "@/lib/datetime";
import {
  changeRequestOpLabel,
  changeRequestOpTone,
  changeRequestStatusLabel,
  changeRequestStatusTag
} from "@/lib/changeRequest";
import DefList from "@/components/common/DefList.vue";
import DefRow from "@/components/common/DefRow.vue";
import MethodBadge from "@/components/common/MethodBadge.vue";
import type { ChangeRequestDetail, ChangeRequestOp } from "@/api/types";
import type { TagType } from "@/constants/enums";

const props = defineProps<{ show: boolean; requestId: number | null }>();
const emit = defineEmits<{ "update:show": [value: boolean]; changed: [] }>();

const message = useMessage();
const { confirmAction } = useConfirm();
const isMobile = useIsMobile();
const snapshotStore = useSnapshotStore();
const appsStore = useAppsStore();

const publish = ref(false);
const applying = ref(false);
const showReject = ref(false);
const rejecting = ref(false);
const rejectComment = ref("");

const { loading, item, reload } = useDrawerResource<ChangeRequestDetail, number>({
  show: () => props.show,
  id: () => props.requestId,
  fetch: getChangeRequest,
  onLoaded: (detail) => {
    publish.value = detail.main.publish;
  },
  onError: () => emit("update:show", false)
});

const isPending = computed(() => item.value?.main.status === "pending");
const isOutdated = computed(() => isPending.value && Boolean(item.value?.outdated));
const busy = computed(() => applying.value || rejecting.value);
const deleteCount = computed(
  () => item.value?.ops.filter((op) => changeRequestOpTone(op.op) === "delete").length ?? 0
);

const author = computed(() => {
  const detail = item.value;
  if (!detail) return "";
  const name = detail.author_name || "Unknown";
  return detail.main.api_key_name ? `${name} via key «${detail.main.api_key_name}»` : name;
});

const OP_TAG: Record<string, TagType> = { create: "success", update: "info", delete: "error" };

function opTagType(op: ChangeRequestOp): TagType {
  return OP_TAG[changeRequestOpTone(op.op)] ?? "default";
}

// Create ops have no "before" side; hiding the column keeps their diff compact.
function hasBefore(op: ChangeRequestOp): boolean {
  return op.changes.some((change) => change.before !== "");
}

// Rows are stored without the leading slash; "(other)" is the overflow bucket.
function seenPathLabel(path: string): string {
  return path === "(other)" ? path : `/${path}`;
}

function plural(count: number, word: string): string {
  return `${count} ${word}${count === 1 ? "" : "s"}`;
}

// Changes made by an action reload both the drawer and the list/badge, also on
// failure: the request may have turned outdated or expired meanwhile.
async function afterAction(): Promise<void> {
  await reload();
  emit("changed");
}

function apply(): void {
  const detail = item.value;
  if (!detail || isOutdated.value) return;
  const count = detail.ops.length;
  const parts = [`Apply ${plural(count, "change")}?`];
  if (deleteCount.value) {
    parts.push(`This includes ${plural(deleteCount.value, "deletion")}.`);
  }
  parts.push(
    publish.value
      ? "The new snapshot is published to the gateways right away."
      : "The snapshot is not published; deploy it later."
  );
  const shouldPublish = publish.value;
  confirmAction({
    title: `Apply change request #${detail.main.id}`,
    content: parts.join(" "),
    positiveText: "Apply",
    onConfirm: async () => {
      applying.value = true;
      let applied = false;
      try {
        await applyChangeRequest(detail.main.id, shouldPublish);
        applied = true;
        message.success("Change request applied");
      } catch (error) {
        message.error(apiErrorMessage(error, "Failed to apply change request"));
      } finally {
        applying.value = false;
      }
      await afterAction();
      // applied ops may have created or deleted apps shown in the sidebar
      void appsStore.load();
      if (item.value?.main.publish_error) {
        message.warning(`Applied, but publishing failed: ${item.value.main.publish_error}`);
      }
      if (shouldPublish) {
        void snapshotStore.loadVersion();
      }
      // a failed apply stays open: the drawer shows why (outdated, expired, publish error)
      if (applied) {
        emit("update:show", false);
      }
    }
  });
}

function openReject(): void {
  rejectComment.value = "";
  showReject.value = true;
}

async function submitReject(): Promise<void> {
  const detail = item.value;
  if (!detail) return;
  rejecting.value = true;
  try {
    await rejectChangeRequest(detail.main.id, rejectComment.value.trim());
    message.success("Change request rejected");
    showReject.value = false;
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to reject change request"));
  } finally {
    rejecting.value = false;
  }
  await afterAction();
}
</script>

<template>
  <NDrawer
    :show="show"
    :width="isMobile ? '100%' : 720"
    placement="right"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NDrawerContent :title="item ? `Change request #${item.main.id}` : 'Change request'" closable>
      <NSpin :show="loading">
        <div v-if="item" class="cr">
          <div class="cr__head">
            <h3 class="cr__title">{{ item.main.title || "Untitled" }}</h3>
            <NTag
              size="small"
              :bordered="false"
              :type="changeRequestStatusTag(item.main.status)"
            >
              {{ changeRequestStatusLabel(item.main.status) }}
            </NTag>
          </div>

          <NAlert v-if="isOutdated" type="warning" title="Outdated" :bordered="false">
            {{ item.outdated_reason || "An object this request touches changed after it was proposed." }}
            It can no longer be applied — reject it and ask the agent to propose it again.
          </NAlert>

          <p v-if="item.main.description" class="cr__desc">{{ item.main.description }}</p>

          <DefList>
            <DefRow label="Author">{{ author }}</DefRow>
            <DefRow label="Created">{{ formatUnixDateTime(item.main.created_at_unix, "—") }}</DefRow>
            <template v-if="!isPending">
              <DefRow label="Decided">
                {{ formatUnixDateTime(item.main.decided_at_unix, "—") }}
              </DefRow>
              <DefRow v-if="item.decided_name" label="Decided by">{{ item.decided_name }}</DefRow>
            </template>
            <DefRow v-if="item.main.comment" label="Comment">
              <span class="cr__pre">{{ item.main.comment }}</span>
            </DefRow>
            <DefRow v-if="item.main.error" label="Error">
              <span class="cr__pre cr__error">{{ item.main.error }}</span>
            </DefRow>
            <DefRow v-if="item.main.status === 'applied'" label="Published">
              <span v-if="item.main.published_at_unix > 0">
                {{ formatUnixDateTime(item.main.published_at_unix) }}
              </span>
              <span v-else-if="item.main.publish_error" class="cr__pre cr__error">{{ item.main.publish_error }}</span>
              <span v-else class="muted">Not published — deploy the snapshot to roll it out</span>
            </DefRow>
          </DefList>

          <section class="cr__ops">
            <span class="section-label">Operations ({{ item.ops.length }})</span>
            <div
              v-for="(op, index) in item.ops"
              :key="index"
              class="op"
              :class="`op--${changeRequestOpTone(op.op)}`"
            >
              <div class="op__head">
                <NTag size="small" :bordered="false" :type="opTagType(op)">
                  <template v-if="changeRequestOpTone(op.op) === 'delete'" #icon>
                    <NIcon :component="TrashOutline" />
                  </template>
                  {{ changeRequestOpLabel(op.op) }}
                </NTag>
                <code v-if="op.title" class="op__title">{{ op.title }}</code>
                <span v-if="op.app_name" class="op__app muted">
                  app <span class="mono">{{ op.app_name }}</span>
                </span>
              </div>

              <NAlert
                v-for="(note, noteIndex) in op.notes"
                :key="noteIndex"
                type="warning"
                :bordered="false"
                class="op__note"
              >
                {{ note }}
              </NAlert>

              <div v-if="op.changes.length" class="diff" :class="{ 'diff--no-before': !hasBefore(op) }">
                <div class="diff__row diff__row--head">
                  <span>Field</span>
                  <span v-if="hasBefore(op)">Before</span>
                  <span>After</span>
                </div>
                <div v-for="change in op.changes" :key="change.path" class="diff__row">
                  <div class="diff__path">
                    <code>{{ change.path }}</code>
                    <NTag v-if="change.secret" size="tiny" :bordered="false" type="warning">
                      <template #icon><NIcon :component="LockClosedOutline" /></template>
                      secret
                    </NTag>
                  </div>
                  <div v-if="hasBefore(op)" class="diff__cell">
                    <span class="diff__label muted">Before</span>
                    <div v-if="change.before" class="diff__value diff__value--before">{{ change.before }}</div>
                    <span v-else class="muted">—</span>
                  </div>
                  <div class="diff__cell">
                    <span class="diff__label muted">After</span>
                    <div v-if="change.after" class="diff__value">{{ change.after }}</div>
                    <span v-else class="muted">—</span>
                  </div>
                </div>
              </div>
              <div v-else-if="op.seen_paths.length" class="seen">
                <div
                  v-for="row in op.seen_paths"
                  :key="`${row.method} ${row.path}`"
                  class="seen__row"
                >
                  <MethodBadge :method="row.method" />
                  <div class="seen__main">
                    <code class="seen__path">{{ seenPathLabel(row.path) }}</code>
                    <span v-if="row.sample" class="muted seen__sample" :title="row.sample">
                      e.g. {{ row.sample }}
                    </span>
                  </div>
                  <span
                    class="seen__hits mono"
                    :title="`${row.hits.toLocaleString()} hits, ${row.hits_not_found.toLocaleString()} of them returned 404`"
                  >
                    {{ row.hits.toLocaleString() }}
                  </span>
                </div>
              </div>
              <span v-else-if="changeRequestOpTone(op.op) !== 'delete'" class="muted op__empty">
                No field changes
              </span>
            </div>
          </section>
        </div>
      </NSpin>

      <template v-if="item && isPending" #footer>
        <div class="cr__footer">
          <NButton
            class="danger-icon-button"
            type="error"
            tertiary
            :disabled="busy"
            :loading="rejecting"
            @click="openReject"
          >
            <template #icon><NIcon :component="CloseOutline" /></template>
            Reject…
          </NButton>
          <div class="cr__footer-right">
            <NCheckbox v-model:checked="publish" :disabled="busy || isOutdated">
              Publish after applying
            </NCheckbox>
            <NButton
              type="primary"
              :disabled="isOutdated || busy"
              :loading="applying"
              :title="isOutdated ? 'Outdated requests cannot be applied' : undefined"
              @click="apply"
            >
              <template #icon><NIcon :component="CheckmarkOutline" /></template>
              Apply
            </NButton>
          </div>
        </div>
      </template>
    </NDrawerContent>
  </NDrawer>

  <NModal
    :show="showReject"
    preset="card"
    :title="item ? `Reject change request #${item.main.id}` : 'Reject change request'"
    class="cr-reject-modal"
    :bordered="false"
    @update:show="(value: boolean) => (showReject = value)"
  >
    <NInput
      v-model:value="rejectComment"
      type="textarea"
      :autosize="{ minRows: 3, maxRows: 8 }"
      :disabled="rejecting"
      placeholder="Optional comment — the agent sees why the request was rejected"
    />
    <template #footer>
      <div class="form-actions">
        <NButton :disabled="rejecting" @click="showReject = false">Cancel</NButton>
        <NButton type="error" :loading="rejecting" @click="submitReject">Reject</NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
:global(.cr-reject-modal) {
  width: min(480px, calc(100vw - 32px));
}

.cr {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.cr__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.cr__title {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.cr__desc {
  margin: 0;
  font-size: 13.5px;
  color: var(--c-text-2);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.cr__pre {
  white-space: pre-wrap;
}

.cr__error {
  color: var(--c-error);
}

.cr__ops {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.op {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--c-border);
  border-left: 3px solid var(--c-border-strong);
  border-radius: 10px;
  background: var(--c-surface-2);
}

.op--create {
  border-left-color: var(--c-success);
}

.op--update {
  border-left-color: #70c0e8;
}

.op--delete {
  border-color: rgba(232, 128, 128, 0.35);
  border-left-color: var(--c-error);
  background: rgba(232, 128, 128, 0.06);
}

.op__head {
  display: flex;
  align-items: center;
  gap: 8px 10px;
  flex-wrap: wrap;
}

.op__title {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.op__app {
  font-size: 12.5px;
}

.op__note {
  font-size: 13px;
}

.op__empty {
  font-size: 12.5px;
}

.diff {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--c-border);
  border-radius: 8px;
  background: var(--c-surface);
  overflow: hidden;
}

.diff__row {
  display: grid;
  grid-template-columns: minmax(110px, 0.7fr) minmax(0, 1fr) minmax(0, 1fr);
  gap: 10px;
  padding: 7px 10px;
  align-items: start;
}

.diff--no-before .diff__row {
  grid-template-columns: minmax(110px, 0.7fr) minmax(0, 2fr);
}

.diff__row:not(:last-child) {
  border-bottom: 1px solid var(--c-border);
}

.diff__row--head {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--c-text-3);
  background: var(--c-surface-2);
}

.diff__path {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  min-width: 0;
}

.diff__path code {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--c-teal);
  overflow-wrap: anywhere;
}

.diff__cell {
  min-width: 0;
  font-size: 12.5px;
}

.diff__label {
  display: none;
  font-size: 11px;
}

.diff__value {
  max-height: 180px;
  overflow: auto;
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.5;
  color: var(--c-text);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.diff__value--before {
  color: var(--c-text-2);
}

.seen {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 360px;
  overflow-y: auto;
}

.seen__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 7px 10px;
  border: 1px solid var(--c-border);
  border-radius: 8px;
  background: var(--c-surface);
}

.seen__main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

.seen__path {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.seen__sample {
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.seen__hits {
  flex: none;
  min-width: 56px;
  text-align: right;
  font-size: 12.5px;
}

.cr__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  flex-wrap: wrap;
}

.cr__footer-right {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
}

@media (max-width: 560px) {
  .diff__row,
  .diff--no-before .diff__row {
    grid-template-columns: 1fr;
    gap: 4px;
  }

  .diff__row--head {
    display: none;
  }

  .diff__label {
    display: block;
  }
}
</style>
