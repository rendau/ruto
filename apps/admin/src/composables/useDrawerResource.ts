import { ref, shallowRef, watch } from "vue";
import { useMessage } from "naive-ui";
import { apiErrorMessage } from "@/api/http";

export interface UseDrawerResourceOptions<T, Id> {
  show: () => boolean;
  id: () => Id | null;
  fetch: (id: Id) => Promise<T>;
  onLoaded?: (item: T) => void | Promise<void>;
  onError?: () => void;
}

export function useDrawerResource<T, Id extends string | number>(
  options: UseDrawerResourceOptions<T, Id>
) {
  const message = useMessage();
  const loading = ref(false);
  const item = shallowRef<T | null>(null);

  async function reload(): Promise<void> {
    const id = options.id();
    if (!options.show() || id == null || id === "") return;
    loading.value = true;
    item.value = null;
    try {
      const loaded = await options.fetch(id);
      item.value = loaded;
      await options.onLoaded?.(loaded);
    } catch (error) {
      message.error(apiErrorMessage(error, "Failed to load"));
      options.onError?.();
    } finally {
      loading.value = false;
    }
  }

  watch(() => [options.show(), options.id()] as const, reload);

  return { loading, item, reload };
}
