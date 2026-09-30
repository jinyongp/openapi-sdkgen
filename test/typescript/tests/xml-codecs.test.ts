import { describe, expect, it } from "vitest";
import {
  decodeXML,
  encodeXML,
  type WireSchema,
} from "../../../internal/target/typescript/runtime/internal/codecs.js";

const catalogSchema: WireSchema = {
  types: ["object"],
  xml: { name: "catalog", prefix: "c", namespace: "urn:catalog" },
  properties: {
    catalog_id: {
      property: "catalogId",
      schema: {
        types: ["string"],
        xml: { name: "id", prefix: "c", attribute: true },
      },
    },
    items: {
      property: "items",
      schema: {
        types: ["array"],
        xml: { name: "items", prefix: "c", wrapped: true },
        items: {
          types: ["object"],
          xml: { name: "item", prefix: "c" },
          properties: {
            code: {
              property: "code",
              schema: {
                types: ["string"],
                xml: { name: "code", prefix: "c", attribute: true },
              },
            },
            count: {
              property: "count",
              schema: { types: ["integer"], xml: { name: "count", prefix: "c" } },
            },
            enabled: {
              property: "enabled",
              schema: { types: ["boolean"], xml: { name: "enabled", prefix: "c" } },
            },
            label: {
              property: "label",
              schema: { types: ["string"], xml: { name: "label", prefix: "c" } },
            },
          },
        },
      },
    },
  },
};

describe("XML runtime codecs", () => {
  it("round-trips namespaces, attributes, wrapped arrays and scalar values", () => {
    const wire = {
      catalog_id: 'catalog&"one',
      items: [
        { code: "a&b", count: 2, enabled: true, label: "<first>" },
        { code: "second", count: 3, enabled: false, label: "plain" },
      ],
    };

    const encoded = encodeXML(wire, catalogSchema, {});
    expect(encoded).toBe(
      '<c:catalog xmlns:c="urn:catalog" c:id="catalog&amp;&quot;one">' +
        '<c:items><c:item c:code="a&amp;b"><c:count>2</c:count><c:enabled>true</c:enabled>' +
        "<c:label>&lt;first&gt;</c:label></c:item>" +
        '<c:item c:code="second"><c:count>3</c:count><c:enabled>false</c:enabled>' +
        "<c:label>plain</c:label></c:item></c:items></c:catalog>",
    );
    expect(decodeXML(encoded, catalogSchema, {})).toEqual(wire);
  });

  it("resolves referenced XML schemas and rejects missing components", () => {
    const components: Record<string, WireSchema> = {
      Item: {
        types: ["object"],
        xml: { name: "item" },
        properties: {
          value: {
            property: "value",
            schema: { types: ["number"], xml: { name: "value" } },
          },
        },
      },
    };
    const reference: WireSchema = { reference: "Item" };

    expect(encodeXML({ value: 1.5 }, reference, components)).toBe(
      "<item><value>1.5</value></item>",
    );
    expect(decodeXML("<item><value>1.5</value></item>", reference, components)).toEqual({
      value: 1.5,
    });
    expect(() => encodeXML({}, { reference: "Missing" }, {})).toThrow(
      "XML schema references missing component Missing",
    );
    expect(() => decodeXML("<item/>", { reference: "Missing" }, {})).toThrow(
      "XML schema references missing component Missing",
    );
  });

  it("rejects malformed documents, invalid scalar values and invalid character references", () => {
    expect(() =>
      decodeXML(
        "<item><count>1</item>",
        {
          types: ["object"],
          properties: {
            count: { property: "count", schema: { types: ["integer"], xml: { name: "count" } } },
          },
        },
        {},
      ),
    ).toThrow("does not match the open element");

    expect(() => decodeXML("<value>1.5</value>", { types: ["integer"] }, {})).toThrow(
      "XML value is not an integer",
    );
    expect(() => decodeXML("<value>maybe</value>", { types: ["boolean"] }, {})).toThrow(
      "XML value is not a boolean",
    );
    expect(() => decodeXML("<value>&#0;</value>", { types: ["string"] }, {})).toThrow(
      "XML character reference is invalid",
    );
    expect(() => decodeXML("<value broken></value>", { types: ["string"] }, {})).toThrow(
      "XML attribute syntax is invalid",
    );
  });

  it("preserves comments and CDATA while rejecting text outside the root", () => {
    expect(
      decodeXML(
        '<?xml version="1.0"?><!--before--><value><![CDATA[a < b & c]]></value><!--after-->',
        { types: ["string"] },
        {},
      ),
    ).toBe("a < b & c");
    expect(() => decodeXML("outside<value>inside</value>", { types: ["string"] }, {})).toThrow(
      "XML text appears outside the document element",
    );
  });
});
