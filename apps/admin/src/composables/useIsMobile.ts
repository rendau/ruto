import { useMediaQuery } from "@vueuse/core";

// Reactive flag for phone-sized viewports. Used to make drawers full-width.
export function useIsMobile() {
  return useMediaQuery("(max-width: 640px)");
}
