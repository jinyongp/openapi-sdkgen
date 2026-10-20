import { matchesMailbox } from "./wire-format-mailbox.js";
export function matchesWireEmail(value: string): boolean {
  return matchesMailbox(value, false);
}
