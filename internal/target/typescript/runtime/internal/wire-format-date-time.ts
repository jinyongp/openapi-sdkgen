import { matchesWireDate } from "./wire-format-date.js";
import { matchesWireTime } from "./wire-format-time.js";
export function matchesWireDateTime(value: string): boolean {
  const split: string[] = value.indexOf("T") >= 0 ? value.split("T", 2) : value.split("t", 2);
  return split.length === 2 && matchesWireDate(split[0]!) && matchesWireTime(split[1]!);
}
