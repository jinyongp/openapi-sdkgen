type SelectionFrame = { readonly value: unknown; readonly leaving: boolean };

/**
 * Collects selection values before the loader performs any module work.
 *
 * The loader supplies its reference decoder, including document/ABI validation.
 * A decoder returns undefined for a container and preserves its own errors.
 * Each container is read once per call; application getters run normally.
 */
export function collectSelectionReferences<Reference extends object>(
  selection: unknown,
  readReference: (value: object) => Reference | undefined,
): readonly Reference[] {
  const completed: WeakSet<object> = new WeakSet<object>();
  const active: WeakSet<object> = new WeakSet<object>();
  const references: Set<Reference> = new Set<Reference>();
  const stack: SelectionFrame[] = [{ value: selection, leaving: false }];

  while (stack.length !== 0) {
    const frame: SelectionFrame | undefined = stack.pop();
    if (frame === undefined) break;
    const value: unknown = frame.value;
    if (value === null || typeof value !== "object") {
      throw new TypeError("Operation selection must contain references, arrays, or groups");
    }
    if (frame.leaving) {
      active.delete(value);
      completed.add(value);
      continue;
    }
    if (completed.has(value)) continue;
    if (active.has(value)) throw new TypeError("Cyclic operation selection");

    const reference: Reference | undefined = readReference(value);
    if (reference !== undefined) {
      references.add(reference);
      completed.add(value);
      continue;
    }

    const entries: unknown[] = [];
    let thenValue: unknown;
    if (Array.isArray(value)) {
      const length: unknown = Reflect.get(value, "length");
      if (
        typeof length !== "number" ||
        !Number.isInteger(length) ||
        length < 0 ||
        length > 0xffff_ffff
      ) {
        throw new TypeError("Operation selection array has an invalid length");
      }
      for (let index: number = 0; index < length; index++)
        entries.push(Reflect.get(value, String(index)));
      thenValue = Reflect.get(value, "then");
    } else {
      const keys: string[] = Object.getOwnPropertyNames(value);
      for (const key of keys) entries.push(Reflect.get(value, key));
      const thenIndex: number = keys.indexOf("then");
      // Reuse an own getter's snapshot. Reading an inherited then only detects
      // an unresolved PromiseLike; neither this function nor the decoder awaits it.
      thenValue = thenIndex < 0 ? Reflect.get(value, "then") : entries[thenIndex];
    }
    if (typeof thenValue === "function") {
      throw new TypeError("Await asynchronous operation selection before passing it to the loader");
    }

    active.add(value);
    stack.push({ value, leaving: true });
    for (let index: number = entries.length - 1; index >= 0; index--) {
      stack.push({ value: entries[index], leaving: false });
    }
  }
  return [...references];
}
