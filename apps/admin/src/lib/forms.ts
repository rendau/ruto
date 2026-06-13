import { emptyAuth, normalizeAuth } from "@/api/normalize";
import type { Auth, AuthMethod, AuthMethodType, Variable } from "@/api/types";

export function createEmptyAuthMethod(type: AuthMethodType): AuthMethod {
  const result: AuthMethod = {};
  if (type === "basic") {
    result.basic = { users: [{ username: "", password: "" }] };
  } else if (type === "api_key") {
    result.api_key = { header: "", keys: [{ name: "", key: "" }] };
  } else if (type === "jwt") {
    result.jwt = { kid: "", roles: [] };
  } else {
    result.ip_validation = { allowed_ips: [{ name: "", ip: "" }] };
  }
  return result;
}

export function parseAuthFromJson(value: string): Auth {
  if (!value.trim()) {
    return emptyAuth();
  }
  return normalizeAuth(JSON.parse(value) as Auth);
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

function splitKeyValue(line: string): { key: string; value: string } | null {
  const trimmed = line.trim();
  if (!trimmed) {
    return null;
  }
  const colonIndex = trimmed.indexOf(":");
  const equalsIndex = trimmed.indexOf("=");
  let separatorIndex: number;
  if (colonIndex >= 0 && equalsIndex >= 0) {
    separatorIndex = Math.min(colonIndex, equalsIndex);
  } else {
    separatorIndex = Math.max(colonIndex, equalsIndex);
  }
  if (separatorIndex < 0) {
    return { key: trimmed, value: "" };
  }
  const key = trimmed.slice(0, separatorIndex).trim();
  if (!key) {
    return null;
  }
  return { key, value: trimmed.slice(separatorIndex + 1).trim() };
}

export function recordToKeyValueLines(value?: Record<string, string> | null): string {
  if (!value) {
    return "";
  }
  return Object.entries(value)
    .map(([key, itemValue]) => `${key}: ${itemValue}`)
    .join("\n");
}

export function keyValueLinesToRecord(value: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const line of value.split("\n")) {
    const parsed = splitKeyValue(line);
    if (parsed) {
      result[parsed.key] = parsed.value;
    }
  }
  return result;
}

export function variablesToKeyValueLines(value?: Variable[] | null): string {
  return (value || []).map((item) => `${item.key}: ${item.value}`).join("\n");
}

export function keyValueLinesToVariables(value: string): Variable[] {
  const result: Variable[] = [];
  for (const line of value.split("\n")) {
    const parsed = splitKeyValue(line);
    if (parsed) {
      result.push(parsed);
    }
  }
  return result;
}

export function hasDuplicateVariableKeys(value?: Variable[] | null): boolean {
  const seen = new Set<string>();
  for (const item of value || []) {
    const key = (item.key || "").trim();
    if (!key) continue;
    if (seen.has(key)) {
      return true;
    }
    seen.add(key);
  }
  return false;
}
