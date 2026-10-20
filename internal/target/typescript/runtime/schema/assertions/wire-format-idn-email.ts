import { matchesMailbox } from "./wire-format-mailbox.js";
export function matchesWireIDNEmail(value: string): boolean {
  return matchesMailbox(value, true);
}
