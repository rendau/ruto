import { computed, ref, shallowRef } from "vue";
import type { SelectOption } from "naive-ui";

interface OptionsLookupConfig<T> {
  list: (query: string) => Promise<T[]>;
  get: (id: string) => Promise<T>;
  idOf: (item: T) => string;
  labelOf: (item: T) => string;
}

// Closure factory: the module-scoped cache is shared across all uses of the
// returned composable, while each call gets its own loading state.
export function createOptionsLookup<T>(config: OptionsLookupConfig<T>) {
  const cache = shallowRef(new Map<string, T>());

  function cacheItems(items: T[]): void {
    const next = new Map(cache.value);
    for (const item of items) {
      next.set(config.idOf(item), item);
    }
    cache.value = next;
  }

  return function useOptionsLookup() {
    const loading = ref(false);

    const options = computed<SelectOption[]>(() =>
      Array.from(cache.value.values()).map((item) => ({
        label: config.labelOf(item),
        value: config.idOf(item)
      }))
    );

    async function search(query = ""): Promise<void> {
      loading.value = true;
      try {
        cacheItems(await config.list(query.trim()));
      } finally {
        loading.value = false;
      }
    }

    async function ensure(id: string): Promise<void> {
      if (!id || cache.value.has(id)) return;
      try {
        cacheItems([await config.get(id)]);
      } catch {
        // Display falls back to the raw id.
      }
    }

    function nameOf(id: string): string {
      const item = cache.value.get(id);
      return item ? config.labelOf(item) : id;
    }

    return { options, loading, search, ensure, nameOf };
  };
}
