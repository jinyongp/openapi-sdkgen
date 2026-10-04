import type { WireSchema, WireProperty } from "./wire-types.js";
import type { ValidationContext } from "./wire-context.js";
import { defineOwnDataProperty, isRecord } from "../shared/runtime-support.js";

/** Combines mappings of one original instance without applying a mapping twice. */
export function mergeWireRepresentations(
  original: unknown,
  values: readonly unknown[],
  context: ValidationContext,
): unknown {
  if (values.length === 0) return original;
  if (values.length === 1) return values[0];
  if (values.every((value: unknown): boolean => value === original)) return original;
  if (values.every(Array.isArray)) {
    const source: unknown[] = Array.isArray(original) ? original : [];
    return values[0]!.map((_: unknown, index: number): unknown =>
      mergeWireRepresentations(
        source[index],
        values.map((value: unknown[]): unknown => value[index]),
        context,
      ),
    );
  }
  if (values.every(isRecord)) {
    const source: Record<string, unknown> = isRecord(original)
      ? original
      : (Object.create(null) as Record<string, unknown>);
    const result: Record<string, unknown> = {};
    const keys: Set<string> = new Set(
      values.flatMap((value: Record<string, unknown>): string[] => Object.keys(value)),
    );
    const targets: Set<string> = new Set<string>();
    for (const key of keys) {
      const mapped: Record<string, unknown>[] = values.filter(
        (value: Record<string, unknown>): boolean | undefined =>
          context.mappedProperties?.get(value)?.has(key),
      );
      // Explicit destinations survive removal of the same name as a source elsewhere.
      if (
        mapped.length === 0 &&
        Object.hasOwn(source, key) &&
        values.some((value: Record<string, unknown>): boolean => !Object.hasOwn(value, key))
      )
        continue;
      if (mapped.length > 0) targets.add(key);
      defineOwnDataProperty(
        result,
        key,
        mergeWireRepresentations(
          source[key],
          (mapped.length > 0
            ? mapped
            : values.filter((value: Record<string, unknown>): boolean => Object.hasOwn(value, key))
          ).map((value: Record<string, unknown>): unknown => value[key]),
          context,
        ),
      );
    }
    if (targets.size > 0) {
      context.mappedProperties ??= new WeakMap<object, ReadonlySet<string>>();
      context.mappedProperties.set(result, targets);
    }
    return result;
  }
  return values[0];
}

/** Correlated child contracts and destination for one source object property. */
export interface ClassifiedWireProperty {
  readonly sourceName: string;
  readonly targetName: string;
  readonly wireName: string;
  readonly schemas: readonly WireSchema[];
  readonly additional: boolean;
}

/** Classifies an instance's keys within this schema object, before name mapping. */
export function classifyWireProperties(
  value: Readonly<Record<string, unknown>>,
  schema: WireSchema,
  direction: "encode" | "decode",
  context: ValidationContext,
): ClassifiedWireProperty[] {
  const declared: Map<string, DeclaredWireProperty> = new Map(
    Object.entries(schema.properties ?? {}).map(
      ([wireName, definition]: [string, WireProperty]): readonly [string, DeclaredWireProperty] =>
        [
          direction === "encode" ? definition.property : wireName,
          { wireName, definition },
        ] as const,
    ),
  );
  return Object.keys(value).map((sourceName: string): ClassifiedWireProperty => {
    const property: DeclaredWireProperty | undefined = declared.get(sourceName);
    const wireName: string = property?.wireName ?? sourceName;
    const schemas: WireSchema[] = property === undefined ? [] : [property.definition.schema];
    schemas.push(...(context.handlers.patterns?.(schema, wireName) ?? []));
    const additional: boolean = schemas.length === 0;
    if (
      additional &&
      schema.additionalProperties !== undefined &&
      schema.additionalProperties !== false
    )
      schemas.push(schema.additionalProperties);
    return {
      sourceName,
      wireName,
      targetName: direction === "encode" ? wireName : (property?.definition.property ?? sourceName),
      schemas,
      additional,
    };
  });
}

type DeclaredWireProperty = { readonly wireName: string; readonly definition: WireProperty };
