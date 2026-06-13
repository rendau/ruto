<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NTag,
  useMessage,
  type FormInst,
  type FormRules
} from "naive-ui";
import { storeToRefs } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { apiErrorMessage } from "@/api/http";
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import StatusTag from "@/components/common/StatusTag.vue";

const message = useMessage();
const authStore = useAuthStore();
const { profile } = storeToRefs(authStore);

const formRef = ref<FormInst | null>(null);
const saving = ref(false);
const model = reactive({ name: profile.value?.name ?? "", password: "" });

const rules: FormRules = {
  name: [{ required: true, message: "Name is required", trigger: ["blur", "input"] }],
  password: [{ min: 6, message: "At least 6 characters", trigger: ["blur", "input"] }]
};

const accessLabel = computed(() => {
  if (!profile.value) return "—";
  if (profile.value.is_admin || profile.value.all_apps) return "All applications";
  return profile.value.app_ids.length
    ? `${profile.value.app_ids.length} application(s)`
    : "No applications";
});

async function save(): Promise<void> {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  saving.value = true;
  try {
    await authStore.updateProfile({
      name: model.name,
      ...(model.password ? { password: model.password } : {})
    });
    model.password = "";
    message.success("Profile updated");
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to update profile"));
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <PageContainer :width="640">
    <div class="page-head">
      <h1 class="page-head__title">Profile</h1>
    </div>

    <SectionCard v-if="profile" title="Account">
      <div class="account">
        <div class="account__row">
          <span class="account__label">Username</span>
          <span class="mono">{{ profile.username }}</span>
        </div>
        <div class="account__row">
          <span class="account__label">Role</span>
          <NTag size="small" :bordered="false" :type="profile.is_admin ? 'success' : 'default'">
            {{ profile.is_admin ? "Administrator" : "User" }}
          </NTag>
        </div>
        <div class="account__row">
          <span class="account__label">Status</span>
          <StatusTag :active="profile.active" />
        </div>
        <div class="account__row">
          <span class="account__label">Access</span>
          <span>{{ accessLabel }}</span>
        </div>
      </div>
    </SectionCard>

    <SectionCard title="Edit profile">
      <NForm ref="formRef" :model="model" :rules="rules" label-placement="top">
        <NFormItem label="Name" path="name">
          <NInput v-model:value="model.name" placeholder="Your name" />
        </NFormItem>
        <NFormItem label="New password (leave empty to keep)" path="password">
          <NInput
            v-model:value="model.password"
            type="password"
            show-password-on="click"
            placeholder="Unchanged"
          />
        </NFormItem>
        <NButton type="primary" :loading="saving" @click="save">Save changes</NButton>
      </NForm>
    </SectionCard>
  </PageContainer>
</template>

<style scoped>
.page-head__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
}

.account {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.account__row {
  display: grid;
  grid-template-columns: 120px 1fr;
  align-items: center;
}

.account__label {
  font-size: 12.5px;
  color: var(--c-text-3);
}
</style>
