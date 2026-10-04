import type { WireSchema, WireDynamicReference, DynamicScope } from "../wire-types.js";
/** Adds a schema resource to the current dynamic anchor scope when needed. */
export function extendDynamicScope(scope: DynamicScope, schema: WireSchema): DynamicScope {
  return schema.dynamicAnchor === undefined ? scope : [...scope, schema];
}

/** Resolves a dynamic reference against its active scope and declared fallback. */
export function resolveDynamicReference(
  schema: WireSchema,
  scope: DynamicScope,
): WireSchema | undefined {
  const reference: WireDynamicReference | undefined = schema.dynamicReference;
  if (reference === undefined) return undefined;
  // The outer resource is searched first. This lets a resource that overrides
  // an anchor constrain a base schema reached through a normal `$ref`.
  return (
    scope.find((candidate: WireSchema): boolean => candidate.dynamicAnchor === reference.anchor) ??
    reference.fallback
  );
}
