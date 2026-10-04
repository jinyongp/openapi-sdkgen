import type { WireSchema } from "./wire-types.js";
export function matchingPropertySchemas(schema: WireSchema, name: string): readonly WireSchema[] {
  const patterns: (readonly [RegExp, WireSchema])[] = Object.entries(
    schema.patternProperties ?? {},
  ).map(
    ([pattern, child]: [string, WireSchema]): readonly [RegExp, WireSchema] =>
      [new RegExp(pattern, "u"), child] as const,
  );
  return patterns
    .filter(([pattern]: readonly [RegExp, WireSchema]): boolean => pattern.test(name))
    .map(([_pattern, child]: readonly [RegExp, WireSchema]): WireSchema => child);
}
