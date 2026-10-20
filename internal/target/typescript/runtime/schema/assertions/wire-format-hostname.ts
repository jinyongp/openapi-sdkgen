import { matchesDomain } from "./wire-format-domain.js";
export function matchesWireHostname(value: string): boolean {
  return matchesDomain(value, false);
}
