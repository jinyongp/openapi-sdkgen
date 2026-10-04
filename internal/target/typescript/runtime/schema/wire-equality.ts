import { isRecord } from "../shared/runtime-support.js";
/** Compares JSON values structurally, preserving array order and object key identity. */
export function wireValueEquals(left: unknown, right: unknown): boolean {
  if (typeof left === "number" && typeof right === "number") return left === right;
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) && Array.isArray(right)) {
    if (left.length !== right.length) return false;
    for (let index: number = 0; index < left.length; index++) {
      if (Object.hasOwn(left, index) !== Object.hasOwn(right, index)) return false;
      if (Object.hasOwn(left, index) && !wireValueEquals(left[index], right[index])) return false;
    }
    return true;
  }
  if (isRecord(left) && isRecord(right)) {
    const leftKeys: string[] = Object.keys(left).sort();
    const rightKeys: string[] = Object.keys(right).sort();
    return (
      leftKeys.length === rightKeys.length &&
      leftKeys.every(
        (key: string, index: number): boolean =>
          key === rightKeys[index] && wireValueEquals(left[key], right[key]),
      )
    );
  }
  return false;
}
