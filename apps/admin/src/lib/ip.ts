// A light client-side check of addresses and networks: it catches typos before
// the round trip, the server (net/netip) stays the authority on what is valid.
import type { RootTrustedClient } from "@/api/types";

const IPV4_OCTET = "(25[0-5]|2[0-4]\\d|1\\d\\d|[1-9]?\\d)";
const IPV4 = new RegExp(`^${IPV4_OCTET}(\\.${IPV4_OCTET}){3}$`);
const IPV6_CHARS = /^[0-9a-f:.]+$/i;
const PREFIX_BITS = /^\d{1,3}$/;

function isIpv4(value: string): boolean {
  return IPV4.test(value);
}

// The URL parser already knows every IPv6 spelling ("::", embedded IPv4 and so on).
function isIpv6(value: string): boolean {
  if (!value.includes(":") || !IPV6_CHARS.test(value)) return false;
  try {
    new URL(`http://[${value}]/`);
    return true;
  } catch {
    return false;
  }
}

// "" when the value is an IPv4/IPv6 address or a network in CIDR notation.
export function ipOrCidrError(raw: string): string {
  const value = raw.trim();
  if (!value) return "address is required";
  const slash = value.indexOf("/");
  const address = slash < 0 ? value : value.slice(0, slash);
  const bits = slash < 0 ? "" : value.slice(slash + 1);
  const v4 = isIpv4(address);
  if (!v4 && !isIpv6(address)) {
    return "must be an IPv4 or IPv6 address or a network like 203.0.113.0/24";
  }
  if (slash < 0) return "";
  const max = v4 ? 32 : 128;
  if (!PREFIX_BITS.test(bits) || Number(bits) > max) {
    return `network prefix must be from /0 to /${max}`;
  }
  return "";
}

// Fully empty rows are dropped on save, so they aren't an error.
export function trustedClientError(item: RootTrustedClient): string {
  if (!item.name.trim() && !item.ip.trim()) return "";
  const problem = ipOrCidrError(item.ip);
  if (problem) return problem;
  // same limit as the server: a trusted network skips every block and rate limit
  const value = item.ip.trim();
  const slash = value.indexOf("/");
  if (slash < 0) return "";
  const bits = Number(value.slice(slash + 1));
  const minBits = value.includes(":") ? 32 : 8;
  return bits < minBits ? `network is too wide: at most /${minBits}` : "";
}

// The first problem of the section as one line for a toast; "" when it's valid.
export function trustedClientsError(items: RootTrustedClient[]): string {
  for (const [index, item] of items.entries()) {
    const problem = trustedClientError(item);
    if (problem) {
      const name = item.name.trim();
      return `Trusted clients, row ${index + 1}${name ? ` (${name})` : ""}: ${problem}`;
    }
  }
  return "";
}
