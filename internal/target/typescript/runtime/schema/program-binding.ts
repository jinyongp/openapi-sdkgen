import type {
  WireSchema,
  WireSchemas,
  WireCodec,
  WireTransformOptions,
  DynamicScope,
  WireValidationHandlers,
  WireExecution,
} from "./wire-types.js";
import { createValidationContext } from "./wire-state.js";
import { validateProgram, transformProgram } from "./program-execution.js";
const strict: WireTransformOptions = { unknownProperties: "reject" };

/** Dispatches only prepared programs. Arbitrary schemas require the compatibility facade. */
export function bindProgramCodec(
  handlers: WireValidationHandlers,
  execution: WireExecution,
): WireCodec {
  function transformWireValue(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options: WireTransformOptions = strict,
    scope: DynamicScope = [],
  ): unknown {
    return transformProgram(
      value,
      schema,
      components,
      direction,
      options,
      scope,
      createValidationContext(handlers, execution),
    );
  }
  function decodeWireValue(value: unknown, schema: WireSchema, components: WireSchemas): unknown {
    return transformWireValue(value, schema, components, "decode");
  }
  function encodeWireValue(value: unknown, schema: WireSchema, components: WireSchemas): unknown {
    return transformWireValue(value, schema, components, "encode");
  }
  function validateWireValue(
    value: unknown,
    schema: WireSchema,
    components: WireSchemas,
    direction: "encode" | "decode",
    options: WireTransformOptions = strict,
    scope: DynamicScope = [],
  ): void {
    validateProgram(
      value,
      schema,
      components,
      direction,
      options,
      scope,
      createValidationContext(handlers, execution),
    );
  }
  return { transformWireValue, decodeWireValue, encodeWireValue, validateWireValue };
}
