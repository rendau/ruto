import type { SelectOption } from "naive-ui";
import type {
  AuthMethodType,
  CompressionAlgorithm,
  CompressionBackend,
  CompressionResponse,
  LoggingLevel,
  LoggingMode
} from "@/api/types";

export const HTTP_METHODS = [
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "HEAD",
  "OPTIONS",
  "CONNECT",
  "TRACE",
  "*"
] as const;

export type HttpMethod = (typeof HTTP_METHODS)[number];

export const HTTP_METHOD_OPTIONS: SelectOption[] = HTTP_METHODS.map((method) => ({
  label: method,
  value: method
}));

// Naive UI tag types used to colour-code HTTP method badges.
export type TagType = "default" | "primary" | "info" | "success" | "warning" | "error";

const METHOD_TAG: Record<string, TagType> = {
  GET: "success",
  POST: "info",
  PUT: "warning",
  PATCH: "primary",
  DELETE: "error",
  HEAD: "default",
  OPTIONS: "default",
  CONNECT: "default",
  TRACE: "default",
  "*": "default"
};

export function methodTagType(method: string): TagType {
  return METHOD_TAG[(method || "").trim().toUpperCase()] ?? "default";
}

export const AUTH_MODE_OPTIONS: SelectOption[] = [
  { label: "extend", value: "extend" },
  { label: "replace", value: "replace" }
];

export const AUTH_METHOD_TYPES: { type: AuthMethodType; label: string }[] = [
  { type: "basic", label: "Basic" },
  { type: "api_key", label: "API Key" },
  { type: "jwt", label: "JWT" },
  { type: "ip_validation", label: "IP Validation" }
];

export const AUTH_METHOD_LABEL: Record<AuthMethodType, string> = {
  basic: "Basic",
  api_key: "API Key",
  jwt: "JWT",
  ip_validation: "IP Validation"
};

export const LOGGING_MODE_OPTIONS: { label: string; value: LoggingMode }[] = [
  { label: "extend", value: "extend" },
  { label: "replace", value: "replace" }
];

export const LOGGING_LEVEL_OPTIONS: { label: string; value: LoggingLevel }[] = [
  { label: "inherit", value: "" },
  { label: "all", value: "all" },
  { label: "error", value: "error" },
  { label: "don't log", value: "none" }
];

export const COMPRESSION_BACKEND_OPTIONS: { label: string; value: CompressionBackend }[] = [
  { label: "inherit", value: "" },
  { label: "plain (gateway compresses)", value: "identity" },
  { label: "as the client asks (backend may compress)", value: "client" }
];

export const COMPRESSION_RESPONSE_OPTIONS: { label: string; value: CompressionResponse }[] = [
  { label: "inherit", value: "" },
  { label: "on", value: "on" },
  { label: "off", value: "off" }
];

export const COMPRESSION_ALGORITHM_OPTIONS: { label: string; value: CompressionAlgorithm }[] = [
  { label: "br (Brotli)", value: "br" },
  { label: "zstd (Zstandard)", value: "zstd" },
  { label: "gzip", value: "gzip" }
];
