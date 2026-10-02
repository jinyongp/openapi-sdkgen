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
  it("rejects XML properties whose text cannot be assigned unambiguously", () => {
    const schema: WireSchema = {
      types: ["object"],
      properties: {
        first: { property: "first", schema: { types: ["string"], xml: { nodeType: "text" } } },
        second: { property: "second", schema: { types: ["string"], xml: { nodeType: "cdata" } } },
      },
    };
    expect(() => encodeXML({ first: "a", second: "b" }, schema, {})).toThrow(
      "ambiguous representation",
    );
    expect(() => decodeXML("<root>ab</root>", schema, {})).toThrow("ambiguous representation");
  });
  it("shares composed and referenced XML property representations", () => {
    const components: Record<string, WireSchema> = {
      Base: { properties: { id: { property: "id", schema: { types: ["string"] } } } },
      Number: { types: ["integer"] },
    };
    const schema: WireSchema = {
      types: ["object"],
      xml: { name: "record" },
      allOf: [
        { reference: "Base" },
        { properties: { count: { property: "count", schema: { reference: "Number" } } } },
      ],
    };
    const value = { id: "a", count: 2 };
    expect(encodeXML(value, schema, components)).toBe(
      "<record><id>a</id><count>2</count></record>",
    );
    expect(decodeXML("<record><id>a</id><count>2</count></record>", schema, components)).toEqual(
      value,
    );
  });

  it.each(["text", "cdata"] as const)("decodes %s nodes from parent text", (nodeType) => {
    const schema: WireSchema = {
      types: ["object"],
      xml: { name: "record" },
      properties: {
        value: { property: "value", schema: { types: ["string"], xml: { nodeType } } },
      },
    };
    const value = { value: "a < b & ]]> c" };
    expect(decodeXML(encodeXML(value, schema, {}), schema, {})).toEqual(value);
  });

  it("preserves recursive property references and XML metadata beside a ref", () => {
    const components: Record<string, WireSchema> = {
      Node: {
        types: ["object"],
        properties: {
          value: { property: "value", schema: { types: ["string"] } },
          next: { property: "next", schema: { reference: "Node", xml: { name: "child" } } },
        },
      },
    };
    const schema: WireSchema = { reference: "Node", xml: { name: "record" } };
    const value = { value: "a", next: { value: "b" } };
    const xml = "<record><value>a</value><child><value>b</value></child></record>";
    expect(encodeXML(value, schema, components)).toBe(xml);
    expect(decodeXML(xml, schema, components)).toEqual(value);
  });

  it("tokenizes quoted delimiters and decodes character references exactly once", () => {
    const schema: WireSchema = {
      types: ["object"],
      properties: {
        note: { property: "note", schema: { types: ["string"], xml: { attribute: true } } },
        value: { property: "value", schema: { types: ["string"] } },
      },
    };
    expect(
      decodeXML(
        '<record note="x>y"><value>&#38;lt; &amp;#65; &#x1F600;</value></record>',
        schema,
        {},
      ),
    ).toEqual({ note: "x>y", value: "&lt; &#65; 😀" });
  });

  it("declares child namespaces and matches alternate prefixes by URI", () => {
    const schema: WireSchema = {
      types: ["object"],
      xml: { name: "record", prefix: "r", namespace: "urn:root" },
      properties: {
        value: {
          property: "value",
          schema: {
            types: ["string"],
            xml: { name: "value", prefix: "c", namespace: "urn:child" },
          },
        },
      },
    };
    expect(encodeXML({ value: "hello" }, schema, {})).toBe(
      '<r:record xmlns:r="urn:root"><c:value xmlns:c="urn:child">hello</c:value></r:record>',
    );
    expect(
      decodeXML(
        '<x:record xmlns:x="urn:root" xmlns:y="urn:child"><y:value>hello</y:value></x:record>',
        schema,
        {},
      ),
    ).toEqual({ value: "hello" });
    expect(
      decodeXML('<record><c:value xmlns:c="urn:wrong">hello</c:value></record>', schema, {}),
    ).toEqual({});
  });

  it.each([
    "<root><x:value/></root>",
    '<root a="1" a="2"/>',
    '<root xmlns:x="urn:a" xmlns:y="urn:a" x:a="1" y:a="2"/>',
    '<root a="1"b="2"/>',
    '<root a="<"/>',
    "<root>&unknown;</root>",
    "<root>&amp</root>",
    "<root><![CDATA[unfinished</root>",
    '<root a="unfinished>',
    "<root><!--bad--comment--></root>",
    "<root><!--bad---></root>",
    "<root>bad]]>text</root>",
    '<root xmlns:xml="wrong"/>',
    '<root xmlns:x=""/>',
    "<1root/>",
    "<root>\u0000</root>",
    '<!DOCTYPE root [<!ENTITY x SYSTEM "file:///etc/passwd">]><root>&x;</root>',
  ])("rejects malformed or unsupported XML: %s", (source) => {
    expect(() => decodeXML(source, { types: ["string"] }, {})).toThrow(TypeError);
  });

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
