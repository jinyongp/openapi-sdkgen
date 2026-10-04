import { matchesWireHostname } from "./wire-format-hostname.js";
export function matchesWireIDNHostname(value: string): boolean {
  if (/\s/u.test(value) || value.length === 0) return false;
  try {
    return matchesWireHostname(new URL("http://" + value).hostname);
  } catch {
    return false;
  }
}
