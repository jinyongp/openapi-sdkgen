import type { WireProperty, WireSchema } from "./codecs.js";

/**
 * Constructs identity-mapped property descriptors from compiler-owned arrays.
 * Exact keys, wrapper descriptors and child schema identities are preserved.
 * The concrete return type bounds structural inference in generated modules.
 */
export function wireProperties(
  keys: readonly string[],
  schemas: readonly WireSchema[],
): Readonly<Record<string, WireProperty>> {
  if (keys.length !== schemas.length) {
    throw new TypeError("wire property/schema count mismatch");
  }
  const entries: [string, WireProperty][] = new Array(keys.length);
  for (let index = 0; index < keys.length; index++) {
    const property = keys[index]!;
    entries[index] = [property, { property, schema: schemas[index]! }];
  }
  return Object.fromEntries(entries);
}
