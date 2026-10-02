<script setup lang="ts">
import { computed, h, reactive, ref } from "vue";
import {
  NButton,
  NCheckbox,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputGroup,
  NModal,
  NSelect,
  useDialog,
  type FormItemInst,
  type FormRules
} from "naive-ui";
import { DiceOutline } from "@vicons/ionicons5";
import { createUser, updateUser } from "@/api/usr";
import { useEntityForm } from "@/composables/useEntityForm";
import { useAppOptions } from "@/composables/useAppOptions";
import { useClipboard } from "@/composables/useClipboard";
import { generatePassword } from "@/lib/password";
import SwitchField from "@/components/common/SwitchField.vue";
import type { UsrCreateRep, UsrMain } from "@/api/types";

const props = defineProps<{ show: boolean; user: UsrMain | null }>();
const emit = defineEmits<{ "update:show": [value: boolean]; saved: [] }>();

const appOptions = useAppOptions();
const { copy } = useClipboard();
const dialog = useDialog();

const passwordItemRef = ref<FormItemInst | null>(null);
// A generated password is shown in clear text: the admin has to see what they hand over.
const passwordRevealed = ref(false);

interface FormModel {
  name: string;
  username: string;
  password: string;
  active: boolean;
  is_admin: boolean;
  all_apps: boolean;
  app_ids: string[];
}

const model = reactive<FormModel>({
  name: "",
  username: "",
  password: "",
  active: true,
  is_admin: false,
  all_apps: true,
  app_ids: []
});

const { formRef, submitting, isEdit, submit } = useEntityForm<UsrMain, UsrCreateRep>({
  show: () => props.show,
  entity: () => props.user,
  seed: async (user) => {
    model.name = user?.name ?? "";
    model.username = user?.username ?? "";
    model.password = "";
    passwordRevealed.value = false;
    model.active = user?.active ?? true;
    model.is_admin = user?.is_admin ?? false;
    model.all_apps = user?.all_apps ?? true;
    model.app_ids = [...(user?.app_ids ?? [])];
    void appOptions.search();
    for (const id of model.app_ids) {
      void appOptions.ensure(id);
    }
  },
  create: () =>
    createUser({
      name: model.name,
      username: model.username,
      password: model.password,
      active: model.active,
      is_admin: model.is_admin,
      all_apps: model.is_admin ? true : model.all_apps,
      app_ids: model.is_admin || model.all_apps ? [] : model.app_ids
    }),
  update: (user) => {
    const allApps = model.is_admin ? true : model.all_apps;
    return updateUser({
      id: user.id,
      name: model.name,
      active: model.active,
      is_admin: model.is_admin,
      all_apps: allApps,
      update_app_ids: true,
      app_ids: model.is_admin || allApps ? [] : model.app_ids,
      ...(model.password ? { password: model.password } : {})
    });
  },
  messages: { created: "User created", updated: "User updated" },
  onSaved: () => {
    emit("saved");
    if (model.password) {
      void copyCredentials();
    }
    close();
  }
});

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: "Name is required", trigger: ["blur", "input"] }],
  username: isEdit.value
    ? []
    : [{ required: true, message: "Username is required", trigger: ["blur", "input"] }],
  password: isEdit.value
    ? []
    : [
        { required: true, message: "Password is required", trigger: ["blur", "input"] },
        { min: 6, message: "At least 6 characters", trigger: ["blur", "input"] }
      ]
}));

const submitLabel = computed(
  () => `${isEdit.value ? "Save" : "Create"}${model.password ? " & copy" : ""}`
);

function fillGeneratedPassword(): void {
  model.password = generatePassword();
  passwordRevealed.value = true;
  passwordItemRef.value?.restoreValidation();
}

async function copyCredentials(): Promise<void> {
  const text = [
    window.location.origin,
    `Username: ${model.username.trim()}`,
    `Password: ${model.password}`
  ].join("\n");
  if (await copy(text, "Link, username and password copied")) return;
  // The form is already closed and the saved password can't be read back,
  // so without the clipboard this dialog is the only place left to take it from.
  dialog.warning({
    title: "Copy the credentials manually",
    content: () =>
      h(
        "pre",
        { class: "mono", style: "margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; user-select: all" },
        text
      ),
    positiveText: "Done"
  });
}

function close(): void {
  emit("update:show", false);
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="isEdit ? 'Edit user' : 'New user'"
    class="usr-modal"
    :bordered="false"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NForm ref="formRef" :model="model" :rules="rules" :disabled="submitting" label-placement="top">
      <NFormItem label="Name" path="name">
        <NInput v-model:value="model.name" placeholder="Jane Doe" />
      </NFormItem>
      <NFormItem label="Username" path="username">
        <NInput
          v-model:value="model.username"
          :disabled="isEdit"
          :input-props="{ autocomplete: 'off' }"
          placeholder="jane"
        />
      </NFormItem>
      <NFormItem
        ref="passwordItemRef"
        :label="isEdit ? 'New password (leave empty to keep)' : 'Password'"
        path="password"
      >
        <NInputGroup>
          <NInput
            v-model:value="model.password"
            :type="passwordRevealed ? 'text' : 'password'"
            show-password-on="click"
            :input-props="{ autocomplete: 'new-password' }"
            :placeholder="isEdit ? 'Unchanged' : 'Choose a password'"
          />
          <NButton tertiary :disabled="submitting" @click="fillGeneratedPassword">
            <template #icon><NIcon :component="DiceOutline" /></template>
            Generate
          </NButton>
        </NInputGroup>
      </NFormItem>

      <div class="usr-modal__switches">
        <SwitchField v-model="model.active" label="Active" />
        <NCheckbox v-model:checked="model.is_admin">Administrator</NCheckbox>
      </div>

      <template v-if="!model.is_admin">
        <NFormItem label="Application access">
          <NCheckbox v-model:checked="model.all_apps">Access to all applications</NCheckbox>
        </NFormItem>
        <NFormItem v-if="!model.all_apps" label="Allowed applications" path="app_ids">
          <NSelect
            v-model:value="model.app_ids"
            multiple
            filterable
            :options="appOptions.options.value"
            :loading="appOptions.loading.value"
            placeholder="Select applications"
            @search="appOptions.search"
          />
        </NFormItem>
      </template>
    </NForm>

    <template #footer>
      <div class="form-actions">
        <NButton :disabled="submitting" @click="close">Cancel</NButton>
        <NButton
          type="primary"
          :loading="submitting"
          :title="model.password ? 'Also copies link, username and password' : undefined"
          @click="submit"
        >
          {{ submitLabel }}
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
:global(.usr-modal) {
  width: min(480px, calc(100vw - 32px));
}

.usr-modal__switches {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 18px;
}
</style>
