import type { GlobalThemeOverrides } from "naive-ui";

// Single source of truth for the brand palette. Mirrored as CSS custom
// properties in assets/styles/main.css so plain CSS and Naive UI stay in sync.
export const palette = {
  bg: "#0b0e15",
  bgSoft: "#0f131c",
  surface: "#141a25",
  surface2: "#19202d",
  surface3: "#202938",
  border: "#242d3d",
  borderStrong: "#313c52",
  codeBg: "#0d1119",

  primary: "#5b82f0",
  primaryHover: "#7397f6",
  primaryPressed: "#4569cf",
  teal: "#22d3c5",

  text: "#e7edf6",
  text2: "#aab6c8",
  text3: "#7c889b",

  success: "#43c98b",
  warning: "#e8b23a",
  error: "#ef6f72"
} as const;

const FONT_FAMILY =
  'Inter, system-ui, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif';
const FONT_FAMILY_MONO =
  'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace';

export const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: palette.primary,
    primaryColorHover: palette.primaryHover,
    primaryColorPressed: palette.primaryPressed,
    primaryColorSuppl: palette.primaryHover,
    successColor: palette.success,
    successColorHover: "#5cd49d",
    successColorPressed: "#37b87b",
    warningColor: palette.warning,
    warningColorHover: "#f0c25b",
    warningColorPressed: "#d49f2c",
    errorColor: palette.error,
    errorColorHover: "#f4888a",
    errorColorPressed: "#d85d60",
    infoColor: palette.primary,

    borderRadius: "9px",
    borderRadiusSmall: "6px",

    fontFamily: FONT_FAMILY,
    fontFamilyMono: FONT_FAMILY_MONO,
    fontSize: "14px",

    bodyColor: palette.bg,
    cardColor: palette.surface,
    modalColor: "#161d29",
    popoverColor: palette.surface2,
    tableColor: palette.surface,
    tableColorHover: "rgba(91, 130, 240, 0.06)",
    tableHeaderColor: "#10151e",
    inputColor: "#10141d",
    inputColorDisabled: "#11161f",
    codeColor: palette.codeBg,
    actionColor: "#10151e",
    hoverColor: "rgba(91, 130, 240, 0.09)",

    borderColor: palette.border,
    dividerColor: palette.border,

    textColorBase: palette.text,
    textColor1: palette.text,
    textColor2: palette.text2,
    textColor3: palette.text3,
    placeholderColor: "#5f6b7e",
    iconColor: palette.text3,

    scrollbarColor: "rgba(120, 134, 158, 0.35)",
    scrollbarColorHover: "rgba(120, 134, 158, 0.55)"
  },
  Layout: {
    color: palette.bg,
    siderColor: palette.bgSoft,
    headerColor: palette.bgSoft,
    siderBorderColor: palette.border,
    headerBorderColor: palette.border
  },
  Card: {
    color: palette.surface,
    borderColor: palette.border,
    titleFontWeight: "600"
  },
  Menu: {
    itemColorActive: "rgba(91, 130, 240, 0.16)",
    itemColorActiveHover: "rgba(91, 130, 240, 0.22)"
  },
  DataTable: {
    thColor: "#10151e",
    tdColorHover: "rgba(91, 130, 240, 0.06)",
    borderColor: palette.border
  },
  Drawer: {
    color: "#11161f"
  },
  Tabs: {
    tabFontWeightActive: "600"
  }
};
