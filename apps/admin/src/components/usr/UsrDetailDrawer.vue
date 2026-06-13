<script setup lang="ts">
import { NDrawer, NDrawerContent, NSpin, NTag } from "naive-ui";
import { getUser } from "@/api/usr";
import { useDrawerResource } from "@/composables/useDrawerResource";
import { useIsMobile } from "@/composables/useIsMobile";
import StatusTag from "@/components/common/StatusTag.vue";
import DefList from "@/components/common/DefList.vue";
import DefRow from "@/components/common/DefRow.vue";
import type { UsrMain } from "@/api/types";

const props = defineProps<{ show: boolean; userId: number | null }>();
const emit = defineEmits<{ "update:show": [value: boolean] }>();

const isMobile = useIsMobile();

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
    :width="isMobile ? '100%' : 420"
    placement="right"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NDrawerContent title="User details" closable>
      <NSpin :show="loading">
        <DefList v-if="item">
          <DefRow label="Id">{{ item.id }}</DefRow>
          <DefRow label="Name">{{ item.name || "—" }}</DefRow>
          <DefRow label="Username">
            <span class="mono">{{ item.username }}</span>
          </DefRow>
          <DefRow label="Role">
            <NTag size="small" :bordered="false" :type="item.is_admin ? 'success' : 'default'">
              {{ item.is_admin ? "Administrator" : "User" }}
            </NTag>
          </DefRow>
          <DefRow label="Status">
            <StatusTag :active="item.active" />
          </DefRow>
          <DefRow label="Access">
            <span v-if="item.is_admin || item.all_apps">All applications</span>
            <div v-else-if="item.app_ids.length" class="access-list">
              <NTag v-for="id in item.app_ids" :key="id" size="small" :bordered="false">{{ id }}</NTag>
            </div>
            <span v-else class="muted">No applications</span>
          </DefRow>
        </DefList>
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
