import { defineStore } from "pinia";
import { ref } from "vue";

const WIDTH_STORAGE_KEY = "ruto_admin_sidebar_width";
export const SIDEBAR_MIN_WIDTH = 240;
export const SIDEBAR_MAX_WIDTH = 440;
const SIDEBAR_DEFAULT_WIDTH = 296;

function clamp(value: number): number {
  return Math.min(SIDEBAR_MAX_WIDTH, Math.max(SIDEBAR_MIN_WIDTH, Math.round(value)));
}

function readStoredWidth(): number {
  const stored = Number(localStorage.getItem(WIDTH_STORAGE_KEY) || "");
  if (Number.isFinite(stored) && stored >= SIDEBAR_MIN_WIDTH && stored <= SIDEBAR_MAX_WIDTH) {
    return stored;
  }
  return SIDEBAR_DEFAULT_WIDTH;
}

export const useUiStore = defineStore("ui", () => {
  const sidebarWidth = ref(readStoredWidth());
  const mobileSidebarOpen = ref(false);

  function setSidebarWidth(width: number): void {
    const clamped = clamp(width);
    sidebarWidth.value = clamped;
    localStorage.setItem(WIDTH_STORAGE_KEY, String(clamped));
  }

  function openMobileSidebar(): void {
    mobileSidebarOpen.value = true;
  }

  function closeMobileSidebar(): void {
    mobileSidebarOpen.value = false;
  }

  return {
    sidebarWidth,
    mobileSidebarOpen,
    setSidebarWidth,
    openMobileSidebar,
    closeMobileSidebar
  };
});
