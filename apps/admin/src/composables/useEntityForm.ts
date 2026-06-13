import { computed, ref, watch } from "vue";
import { useMessage, type FormInst } from "naive-ui";
import { apiErrorMessage } from "@/api/http";

export interface UseEntityFormOptions<E, CreateRep> {
  show: () => boolean;
  entity: () => E | null;
  seed: (entity: E | null) => void | Promise<void>;
  create: () => Promise<CreateRep>;
  update: (entity: E) => Promise<unknown>;
  messages: { created: string; updated: string };
  onSaved: (created?: CreateRep) => void;
  /** Optional extra validation beyond the NForm rules. Return false to abort. */
  validate?: () => boolean | Promise<boolean>;
}

export function useEntityForm<E, CreateRep = unknown>(options: UseEntityFormOptions<E, CreateRep>) {
  const message = useMessage();
  const formRef = ref<FormInst | null>(null);
  const submitting = ref(false);
  const isEdit = computed(() => options.entity() !== null);

  watch(
    options.show,
    async (show) => {
      if (!show) return;
      await options.seed(options.entity());
      formRef.value?.restoreValidation();
    },
    { immediate: true }
  );

  async function submit(): Promise<void> {
    try {
      await formRef.value?.validate();
    } catch {
      return;
    }
    if (options.validate) {
      const ok = await options.validate();
      if (!ok) return;
    }

    submitting.value = true;
    try {
      const entity = options.entity();
      if (entity !== null) {
        await options.update(entity);
        message.success(options.messages.updated);
        options.onSaved();
      } else {
        const created = await options.create();
        message.success(options.messages.created);
        options.onSaved(created);
      }
    } catch (error) {
      message.error(apiErrorMessage(error, "Unexpected error, please try again"));
    } finally {
      submitting.value = false;
    }
  }

  return { formRef, submitting, isEdit, submit };
}
