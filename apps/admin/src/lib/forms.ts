import type { Auth } from "../types/api";

export const emptyAuth: Auth = {
  enabled: false,
  mode: "extend",
  methods: []
};

export function prettyJson(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

export function parseAuthFromJson(value: string): Auth {
  if (!value.trim()) {
    return { ...emptyAuth };
  }
  const parsed = JSON.parse(value) as Auth;
  return {
    enabled: !!parsed.enabled,
    mode: parsed.mode || "extend",
    methods: Array.isArray(parsed.methods) ? parsed.methods : []
  };
}

export function linesToArray(value: string): string[] {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

export function arrayToLines(value?: string[]): string {
  return (value || []).join("\n");
}
