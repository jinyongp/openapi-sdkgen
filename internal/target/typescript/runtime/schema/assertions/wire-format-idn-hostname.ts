import { matchesDomain } from "./wire-format-domain.js";
export function matchesWireIDNHostname(value: string): boolean {
  return matchesDomain(value, true);
}
