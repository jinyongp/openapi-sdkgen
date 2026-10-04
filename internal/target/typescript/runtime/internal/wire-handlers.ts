import type { WireValidationHandlers } from "./wire-context.js";
import { extendDynamicScope, resolveDynamicReference } from "./wire-dynamic.js";
import { decodeSchemaContent } from "./wire-content.js";
import { validateLiteral } from "./wire-literal.js";
import { validateNumber } from "./wire-number.js";
import { validateString } from "./wire-string.js";
import { matchesWireFormat } from "./wire-format.js";
import { validateComposition, transformComposition } from "./wire-composition.js";
import { arrayBefore, arrayUnique, arrayContains, arrayAfter } from "./wire-array.js";
import { validateMultipleOf } from "./wire-multiple-of.js";
import { validateStringPattern } from "./wire-string-pattern.js";
import { objectBefore, dependencies, propertyNames, objectAfter } from "./wire-object.js";
import { matchingPropertySchemas } from "./wire-pattern.js";
/** Compatibility composition for arbitrary schemas; selected providers import individual handlers. */
export const fullWireHandlers: WireValidationHandlers = {
  dynamic: { extend: extendDynamicScope, resolve: resolveDynamicReference },
  decodeContent: decodeSchemaContent,
  literal: validateLiteral,
  multipleOf: validateMultipleOf,
  number: validateNumber,
  stringPattern: validateStringPattern,
  string: validateString,
  format: matchesWireFormat,
  composition: validateComposition,
  transformComposition,
  arrayBefore,
  arrayUnique,
  arrayContains,
  arrayAfter,
  objectBefore,
  dependencies,
  propertyNames,
  objectAfter,
  patterns: matchingPropertySchemas,
};
