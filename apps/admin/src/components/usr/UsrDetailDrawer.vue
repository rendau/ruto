<script setup lang="ts">
import {
  NDescriptions,
  NDescriptionsItem,
  NDrawer,
  NDrawerContent,
  NSpin,
  NTag
} from "naive-ui";
import { getUser } from "@/api/usr";
import { useDrawerResource } from "@/composables/useDrawerResource";
import StatusTag from "@/components/common/StatusTag.vue";
import type { UsrMain } from "@/api/types";

const props = defineProps<{ show: boolean; userId: number | null }>();
const emit = defineEmits<{ "update:show": [value: boolean] }>();

const { loading, item } = useDrawerResource<UsrMain, number>({
  show: () => props.show,
  id: () => props.userId,
  fetch: getUser,
  onError: () => emit("update:show", false)
});
</script>

<template>
  <NDrawer
    :show="show"
    :width="420"
    placement="right"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NDrawerContent title="User details" closable>
      <NSpin :show="loading">
        <NDescriptions v-if="item" :column="1" label-placement="left" bordered size="small">
          <NDescriptionsItem label="Id">{{ item.id }}</NDescriptionsItem>
          <NDescriptionsItem label="Name">{{ item.name || "—" }}</NDescriptionsItem>
          <NDescriptionsItem label="Username">
            <span class="mono">{{ item.username }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="Role">
            <NTag size="small" :bordered="false" :type="item.is_admin ? 'success' : 'default'">
              {{ item.is_admin ? "Administrator" : "User" }}
            </NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Status">
            <StatusTag :active="item.active" />
          </NDescriptionsItem>
          <NDescriptionsItem label="Access">
            <span v-if="item.is_admin || item.all_apps">All applications</span>
            <div v-else-if="item.app_ids.length" class="access-list">
              <NTag v-for="id in item.app_ids" :key="id" size="small" :bordered="false">{{ id }}</NTag>
            </div>
            <span v-else class="muted">No applications</span>
          </NDescriptionsItem>
        </NDescriptions>
      </NSpin>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.access-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
