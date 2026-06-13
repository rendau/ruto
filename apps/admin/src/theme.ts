import type { GlobalThemeOverrides } from "naive-ui";

// Color scheme follows Naive UI's built-in dark theme (neutral grays + mint
// primary) — same look as the kusec reference. We override almost nothing;
// these tokens are mirrored as CSS custom properties in assets/styles/main.css.
export const palette = {
  bg: "#101014",
  bgSoft: "#18181c",
  surface: "#18181c",
  surface2: "#202024",
  surface3: "#2a2a30",
  border: "rgba(255, 255, 255, 0.09)",
  borderStrong: "rgba(255, 255, 255, 0.16)",
  codeBg: "#0e0e12",

  primary: "#63e2b7",
  primaryHover: "#7fe7c4",
  primaryPressed: "#5acea7",

  text: "rgba(255, 255, 255, 0.9)",
  text2: "rgba(255, 255, 255, 0.72)",
  text3: "rgba(255, 255, 255, 0.46)",

  success: "#63e2b7",
  warning: "#f2c97d",
  error: "#e88080"
} as const;

const FONT_FAMILY =
  'Inter, system-ui, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif';
const FONT_FAMILY_MONO =
  'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace';

export const themeOverrides: GlobalThemeOverrides = {
  common: {
    borderRadius: "8px",
    borderRadiusSmall: "5px",
    fontFamily: FONT_FAMILY,
    fontFamilyMono: FONT_FAMILY_MONO,
    fontSize: "14px"
  },
  Layout: {
    siderColor: palette.bgSoft,
    headerColor: palette.bgSoft
  },
  Card: {
    titleFontWeight: "600"
  },
  Tabs: {
    tabFontWeightActive: "600"
  }
};
