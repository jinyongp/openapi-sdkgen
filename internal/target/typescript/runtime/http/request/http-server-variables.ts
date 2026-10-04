import type {
  OperationDefinition,
  ServerDefinition,
  ServerVariableDefinition,
  ServerSelection,
} from "../operation.js";
import { resolveServerBaseURL } from "./http-server.js";
function expandServerVariables(
  server: ServerDefinition,
  selection: ServerSelection | undefined,
): string {
  const variables: Readonly<Record<string, string>> = selection?.variables ?? {};
  const expanded: string = server.url.replace(/\{([^}]+)\}/g, (_: string, name: string): string => {
    const definition: ServerVariableDefinition | undefined = server.variables?.find(
      (item: ServerVariableDefinition): boolean => item.name === name,
    );
    if (definition === undefined)
      throw new TypeError(`Server ${server.id} has no variable ${name}`);
    const value: string = variables[name] ?? definition.defaultValue;
    if (definition.enumValues !== undefined && !definition.enumValues.includes(value)) {
      throw new TypeError(
        `Server variable ${name} must be one of ${definition.enumValues.join(", ")}`,
      );
    }
    return value;
  });
  return expanded;
}
/** Resolves declared server variables using the shared URL and origin rules. */
export function resolveOperationBaseURL(
  baseURL: string | undefined,
  origin: string | undefined,
  selection: ServerSelection | undefined,
  operation: OperationDefinition,
): string {
  return resolveServerBaseURL(baseURL, origin, selection, operation, expandServerVariables);
}
