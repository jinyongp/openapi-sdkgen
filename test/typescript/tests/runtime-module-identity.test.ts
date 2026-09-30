import { describe, expect, it } from "vitest";
import * as support from "../../../internal/target/typescript/runtime/internal/runtime-support.js";
import { SortDirection as sourceSortDirection } from "../../../internal/target/typescript/runtime/internal/constants.js";
import * as errors from "../../../internal/target/typescript/runtime/internal/errors.js";
import * as objects from "../../../internal/target/typescript/runtime/internal/objects.js";
import * as operation from "../../../internal/target/typescript/runtime/internal/operation.js";
import * as media from "../../../internal/target/typescript/runtime/internal/media-type.js";
import * as codecs from "../../../internal/target/typescript/runtime/internal/codecs.js";
import * as xml from "../../../internal/target/typescript/runtime/internal/wire-xml.js";
import * as stream from "../../../internal/target/typescript/runtime/internal/http-stream.js";
import * as jsonStream from "../../../internal/target/typescript/runtime/internal/http-json-stream.js";
import * as generatedSupport from "../fixtures/generated/lifecycle/internal/runtime/runtime-support.js";
import { SortDirection as generatedSortDirection } from "../fixtures/generated/lifecycle/internal/runtime/constants.js";
import * as generatedErrors from "../fixtures/generated/lifecycle/internal/runtime/errors.js";
import * as generatedObjects from "../fixtures/generated/lifecycle/internal/runtime/objects.js";
import * as generatedOperation from "../fixtures/generated/lifecycle/internal/runtime/operation.js";
import * as generatedMedia from "../fixtures/generated/lifecycle/internal/runtime/media-type.js";
import * as generatedCodecs from "../fixtures/generated/lifecycle/internal/runtime/codecs.js";
import * as generatedXML from "../fixtures/generated/lifecycle/internal/runtime/wire-xml.js";
import * as generatedStream from "../fixtures/generated/lifecycle/internal/runtime/http-stream.js";
import * as generatedJSONStream from "../fixtures/generated/lifecycle/internal/runtime/http-json-stream.js";

describe("runtime constant parity", () => {
  it("keeps generated sort directions identical to the runtime template", () => {
    expect(generatedSortDirection).toEqual(sourceSortDirection);
  });
});

for (const modules of [
  { name: "source", support, errors, objects, operation, media, codecs, xml, stream, jsonStream },
  {
    name: "generated",
    support: generatedSupport,
    errors: generatedErrors,
    objects: generatedObjects,
    operation: generatedOperation,
    media: generatedMedia,
    codecs: generatedCodecs,
    xml: generatedXML,
    stream: generatedStream,
    jsonStream: generatedJSONStream,
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
      expect(modules.operation.operationDiagnosticName).toBe(
        modules.support.operationDiagnosticName,
      );
      expect(modules.media.isJSONMediaType).toBe(modules.support.isJSONMediaType);
      expect(modules.media.isXMLMediaType).toBe(modules.support.isXMLMediaType);
      expect(modules.xml.encodeXML).toBe(modules.codecs.encodeXML);
      expect(modules.xml.decodeXML).toBe(modules.codecs.decodeXML);
      expect(modules.jsonStream.jsonResponseStreamServices).toBe(
        modules.stream.jsonResponseStreamServices,
      );
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
