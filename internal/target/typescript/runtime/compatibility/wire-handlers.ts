import type { WireValidationHandlers } from "../schema/wire-context.js";
import { extendDynamicScope, resolveDynamicReference } from "../schema/references/wire-dynamic.js";
import { decodeSchemaContent } from "../schema/content/wire-content.js";
import { validateLiteral } from "../schema/assertions/wire-literal.js";
import { validateNumber } from "../schema/assertions/wire-number.js";
import { validateString } from "../schema/assertions/wire-string.js";
import { matchesWireFormat } from "../schema/assertions/wire-format.js";
import {
  validateComposition,
  transformComposition,
} from "../schema/composition/wire-composition.js";
import {
  arrayBefore,
  arrayUnique,
  arrayContains,
  arrayAfter,
} from "../schema/assertions/wire-array.js";
import { validateMultipleOf } from "../schema/assertions/wire-multiple-of.js";
import { validateStringPattern } from "../schema/assertions/wire-string-pattern.js";
import {
  objectBefore,
  dependencies,
  propertyNames,
  objectAfter,
} from "../schema/assertions/wire-object.js";
import { matchingPropertySchemas } from "../schema/assertions/wire-pattern.js";
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
