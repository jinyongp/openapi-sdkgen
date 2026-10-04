import type {
  WireEvaluationFunction,
  WireTransformFunction,
  WireMatchFunction,
  WireMatchingFunction,
} from "./wire-context.js";

export const validateWireValueWithContext: WireEvaluationFunction = (
  ...args: Parameters<WireEvaluationFunction>
): ReturnType<WireEvaluationFunction> => args[6].execution.validate(...args);
export const transformWireValueWithContext: WireTransformFunction = (
  ...args: Parameters<WireTransformFunction>
): unknown => args[6].execution.transform(...args);
export const schemaMatchesForControlFlow: WireMatchFunction = (
  ...args: Parameters<WireMatchFunction>
): boolean => args[6].execution.matches(...args);
export const matchingSchemasForControlFlow: WireMatchingFunction = (
  ...args: Parameters<WireMatchingFunction>
): ReturnType<WireMatchingFunction> => args[6].execution.matching(...args);
