import { APIError } from "../../../internal/target/typescript/runtime/shared/runtime-support.js";
import type { TransportErrorCode } from "../../../internal/target/typescript/runtime/shared/runtime-support.js";
import { isAPIError } from "../../../internal/target/typescript/runtime/shared/runtime-support.js";
import { isRecord } from "../../../internal/target/typescript/runtime/shared/runtime-support.js";
import { defineOwnDataProperty } from "../../../internal/target/typescript/runtime/shared/runtime-support.js";
import { operationDiagnosticName } from "../../../internal/target/typescript/runtime/http/operation.js";
import { isJSONMediaType } from "../../../internal/target/typescript/runtime/shared/runtime-support.js";
import { isXMLMediaType } from "../../../internal/target/typescript/runtime/shared/runtime-support.js";

import { describe, expect, it } from "vitest";
import * as support from "../../../internal/target/typescript/runtime/shared/runtime-support.js";
import { SortDirection as sourceSortDirection } from "../../../internal/target/typescript/runtime/shared/constants.js";
import * as errors from "../../../internal/target/typescript/runtime/client/errors.js";
import * as objects from "../../../internal/target/typescript/runtime/shared/objects.js";
import * as operation from "../../../internal/target/typescript/runtime/http/operation.js";
import * as media from "../../../internal/target/typescript/runtime/media/media-type.js";
import * as codecs from "../../../internal/target/typescript/runtime/compatibility/codecs.js";
import * as xml from "../../../internal/target/typescript/runtime/compatibility/wire-xml.js";
import * as stream from "../../../internal/target/typescript/runtime/compatibility/http-stream.js";
import * as jsonStream from "../../../internal/target/typescript/runtime/compatibility/http-json-stream.js";
import * as generatedSupport from "../fixtures/generated/lifecycle/internal/runtime/shared/runtime-support.js";
import { SortDirection as generatedSortDirection } from "../fixtures/generated/lifecycle/internal/runtime/shared/constants.js";
import * as generatedErrors from "../fixtures/generated/lifecycle/internal/runtime/client/errors.js";
import * as generatedObjects from "../fixtures/generated/lifecycle/internal/runtime/shared/objects.js";
import * as generatedOperation from "../fixtures/generated/lifecycle/internal/runtime/http/operation.js";

describe("runtime constant parity", () => {
  it("keeps generated sort directions identical to the runtime template", () => {
    expect(generatedSortDirection).toEqual(sourceSortDirection);
  });
});

for (const modules of [
  { name: "source", support, errors, objects, operation },
  {
    name: "generated",
    support: generatedSupport,
    errors: generatedErrors,
    objects: generatedObjects,
    operation: generatedOperation,
  },
] as const) {
  describe(`runtime module identity (${modules.name})`, () => {
    it("preserves error constructor and guard identity across entry paths", () => {
      expect(modules.errors.APIError).toBe(modules.support.APIError);
      expect(modules.errors.TransportErrorCode).toBe(modules.support.TransportErrorCode);
      const error = new modules.support.APIError({ code: "TEST", message: "test" });
      expect(modules.errors.isAPIError(error)).toBe(true);
      expect(error).toBeInstanceOf(modules.errors.APIError);
    });
    it("forwards shared functions instead of duplicating their implementation", () => {
      expect(modules.objects.isRecord).toBe(modules.support.isRecord);
      expect(modules.objects.defineOwnDataProperty).toBe(modules.support.defineOwnDataProperty);
    });
    it("retains safe own-property construction and exact diagnostic identity", () => {
      const target: Record<string, string> = {};
      modules.objects.defineOwnDataProperty(target, "__proto__", "value");
      expect(Object.getPrototypeOf(target)).toBe(Object.prototype);
      expect(Object.hasOwn(target, "__proto__")).toBe(true);
      expect(target["__proto__"]).toBe("value");
      expect(
        modules.operation.operationDiagnosticName({ route: "GET /x", operationID: "read" }),
      ).toBe("read");
    });
  });
}

describe("full-capability compatibility facades", () => {
  it("forwards shared media, XML and stream implementations", () => {
    expect(media.isJSONMediaType).toBe(support.isJSONMediaType);
    expect(media.isXMLMediaType).toBe(support.isXMLMediaType);
    expect(xml.encodeXML).toBe(codecs.encodeXML);
    expect(xml.decodeXML).toBe(codecs.decodeXML);
    expect(jsonStream.jsonResponseStreamServices).toBe(stream.jsonResponseStreamServices);
  });
});
