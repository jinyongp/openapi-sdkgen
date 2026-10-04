/** Rejects non-finite numbers, including nested values outside declared fields. */
export function assertFiniteJSONNumbers(
  value: unknown,
  seen: WeakSet<object> = new WeakSet<object>(),
): void {
  if (typeof value === "number") {
    if (!Number.isFinite(value)) throw new TypeError("must be a finite JSON number");
    return;
  }
  if (typeof value !== "object" || value === null || seen.has(value)) return;
  seen.add(value);
  if (Array.isArray(value)) {
    for (const item of value) assertFiniteJSONNumbers(item, seen);
    return;
  }
  for (const item of Object.values(value)) assertFiniteJSONNumbers(item, seen);
}
