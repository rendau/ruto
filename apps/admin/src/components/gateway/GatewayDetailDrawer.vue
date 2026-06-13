<script setup lang="ts">
import { computed } from "vue";
import { NAlert, NDrawer, NDrawerContent, NTag } from "naive-ui";
import { useSnapshotStore } from "@/stores/snapshot";
import { useIsMobile } from "@/composables/useIsMobile";
import { formatUnixAge, formatUnixDateTime } from "@/lib/datetime";
import { formatBytes } from "@/lib/format";
import CopyText from "@/components/common/CopyText.vue";
import DefList from "@/components/common/DefList.vue";
import DefRow from "@/components/common/DefRow.vue";
import type { GatewayStateItem } from "@/api/types";

const props = defineProps<{ show: boolean; gateway: GatewayStateItem | null }>();
const emit = defineEmits<{ "update:show": [value: boolean] }>();

const snapshotStore = useSnapshotStore();
const isMobile = useIsMobile();

const statusType = computed<"success" | "warning" | "error">(() => {
  if (props.gateway?.status === "online") return "success";
  if (props.gateway?.status === "stale") return "warning";
  return "error";
});

const upToDate = computed(
  () => props.gateway?.snapshot_version === snapshotStore.version && Boolean(snapshotStore.version)
);
</script>

<template>
  <NDrawer
    :show="show"
    :width="isMobile ? '100%' : 480"
    placement="right"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NDrawerContent title="Gateway" closable>
      <div v-if="gateway" class="gw">
        <div class="gw__head">
          <span class="gw__id mono">{{ gateway.gateway_id }}</span>
          <NTag size="small" :bordered="false" :type="statusType">{{ gateway.status }}</NTag>
        </div>

        <NAlert v-if="gateway.last_error" type="error" :bordered="false" class="gw__error">
          {{ gateway.last_error }}
        </NAlert>

        <DefList>
          <DefRow label="Host">{{ gateway.host_name || "—" }}</DefRow>
          <DefRow label="Applied version">
            <div class="gw__version">
              <CopyText :value="gateway.snapshot_version" label="Snapshot version" wrap />
              <NTag size="tiny" :bordered="false" :type="upToDate ? 'success' : 'warning'">
                {{ upToDate ? "current" : "behind" }}
              </NTag>
            </div>
          </DefRow>
          <DefRow label="Target version">
            <span class="mono">{{ snapshotStore.version.slice(0, 16) || "—" }}</span>
          </DefRow>
          <DefRow label="Last applied">
            {{ formatUnixDateTime(gateway.last_apply_at_unix) }}
            <span class="muted">({{ formatUnixAge(gateway.last_apply_at_unix) }})</span>
          </DefRow>
          <DefRow label="Last seen">
            {{ formatUnixDateTime(gateway.last_seen_at_unix) }}
            <span class="muted">({{ formatUnixAge(gateway.last_seen_at_unix) }})</span>
          </DefRow>
          <DefRow label="Started">{{ formatUnixDateTime(gateway.started_at_unix) }}</DefRow>
          <DefRow label="Memory">{{ formatBytes(gateway.memory_alloc_bytes) }}</DefRow>
          <DefRow label="Goroutines">{{ gateway.goroutines_count }}</DefRow>
        </DefList>
      </div>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.gw {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.gw__head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.gw__id {
  font-size: 15px;
  font-weight: 600;
}

.gw__version {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
