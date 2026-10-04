import { describe, expect, it } from "vitest";
import {
  decodeXML,
  decodeWireValue,
  encodeXML,
  encodeWireValue,
} from "../../../internal/target/typescript/runtime/compatibility/codecs.js";
import type { WireSchema } from "../../../internal/target/typescript/runtime/schema/wire-types.js";

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
  it("preserves branch mapping through recursive dynamic refs and wrapped arrays", (): void => {
    const node: WireSchema = {
      dynamicAnchor: "node",
      types: ["object"],
      required: ["wire_name"],
      xml: { name: "node" },
      properties: {
        wire_name: {
          property: "wireName",
          schema: { types: ["string"], xml: { attribute: true, name: "name" } },
        },
        children: {
          property: "children",
          schema: {
            types: ["array"],
            xml: { wrapped: true },
            items: { dynamicReference: { anchor: "node", fallback: { reference: "Node" } } },
          },
        },
      },
      anyOf: [
        { properties: { wire_count: { property: "wireCount", schema: { types: ["integer"] } } } },
        { properties: { wire_label: { property: "wireLabel", schema: { types: ["string"] } } } },
      ],
    };
    const components: Record<string, WireSchema> = { Node: node };
    const schema: WireSchema = { reference: "Node" };
    const value: Record<string, unknown> = {
      wireName: "root",
      wireCount: 2,
      wireLabel: "a",
      children: [{ wireName: "child", wireCount: 3 }],
    };
    const wire: unknown = encodeWireValue(value, schema, components);
    expect(
      decodeWireValue(
        decodeXML(encodeXML(wire, schema, components), schema, components),
        schema,
        components,
      ),
    ).toEqual(value);
    const conflict: WireSchema = {
      properties: {
        value: {
          property: "value",
          schema: {
            allOf: [
              { types: ["string"], xml: { name: "first" } },
              { types: ["string"], xml: { name: "second" } },
            ],
          },
        },
      },
    };
    expect(() => encodeXML({ value: "x" }, conflict, {})).toThrow("ambiguous representation");
  });
  it("selects referenced object branches without merging inactive XML names", (): void => {
    const components: Record<string, WireSchema> = {
      Number: { types: ["integer"], minimum: 2 },
      A: {
        types: ["object"],
        required: ["kind", "count"],
        properties: {
          kind: { property: "kind", schema: { constValue: "a" } },
          count: { property: "count", schema: { reference: "Number", xml: { name: "value" } } },
        },
      },
      B: {
        types: ["object"],
        required: ["kind", "label"],
        properties: {
          kind: { property: "kind", schema: { constValue: "b" } },
          label: { property: "label", schema: { types: ["string"], xml: { name: "value" } } },
        },
      },
    };
    const schema: WireSchema = {
      xml: { name: "root" },
      oneOf: [{ reference: "A" }, { reference: "B" }],
    };
    for (const value of [
      { kind: "a", count: 2 },
      { kind: "b", label: "hello" },
    ]) {
      expect(decodeXML(encodeXML(value, schema, components), schema, components)).toEqual(value);
    }
    expect(() =>
      decodeXML("<root><kind>a</kind><value>1</value></root>", schema, components),
    ).toThrow();
  });

  it("preserves all matching anyOf fields and rejects simultaneous XML collisions", (): void => {
    const first: WireSchema = {
      types: ["object"],
      properties: { first: { property: "first", schema: { types: ["string"] } } },
    };
    const second: WireSchema = {
      types: ["object"],
      properties: { second: { property: "second", schema: { types: ["integer"] } } },
    };
    const schema: WireSchema = { xml: { name: "root" }, anyOf: [first, second] };
    const value: Record<string, unknown> = { first: "a", second: 2 };
    expect(decodeXML(encodeXML(value, schema, {}), schema, {})).toEqual(value);
    const collision: WireSchema = {
      anyOf: [
        first,
        {
          properties: {
            second: { property: "second", schema: { types: ["string"], xml: { name: "first" } } },
          },
        },
      ],
    };
    expect(() => encodeXML({ first: "a", second: "b" }, collision, {})).toThrow(
      "ambiguous representation",
    );
    expect(() => decodeXML("<root><first>a</first></root>", collision, {})).toThrow(
      "ambiguous representation",
    );
  });

  it("selects conditional fields and decodes constrained scalar unions", (): void => {
    const schema: WireSchema = {
      types: ["object"],
      properties: { kind: { property: "kind", schema: { types: ["string"] } } },
      if: {
        required: ["kind"],
        properties: { kind: { property: "kind", schema: { constValue: "count" } } },
      },
      then: { properties: { count: { property: "count", schema: { types: ["integer"] } } } },
      else: { properties: { label: { property: "label", schema: { types: ["string"] } } } },
    };
    for (const value of [
      { kind: "count", count: 3 },
      { kind: "label", label: "hello" },
    ]) {
      expect(decodeXML(encodeXML(value, schema, {}), schema, {})).toEqual(value);
    }
    const scalar: WireSchema = {
      anyOf: [
        { types: ["integer"], minimum: 2 },
        { types: ["string"], pattern: "^[a-z]+$" },
      ],
    };
    expect(decodeXML("<root>hello</root>", scalar, {})).toBe("hello");
    expect(decodeXML("<root>2</root>", scalar, {})).toBe(2);
    expect(() => decodeXML("<root>1</root>", scalar, {})).toThrow();
  });
  it.each([
    ["p", true],
    ["p", false],
    ["", true],
    ["", false],
  ] as const)("declares %s namespaces on unwrapped items (named: %s)", (prefix, named) => {
    const schema: WireSchema = {
      types: ["object"],
      xml: { name: "root" },
      properties: {
        values: {
          property: "values",
          schema: {
            types: ["array"],
            xml: { name: "items", prefix, namespace: "urn:items", wrapped: false },
            items: {
              types: ["string"],
              ...(named ? { xml: { name: "item", prefix, namespace: "urn:items" } } : {}),
            },
          },
        },
      },
    };
    const value = { values: ["one", "two"] };
    const localName = named ? "item" : "items";
    const name = prefix ? `p:${localName}` : localName;
    const declaration = prefix ? 'xmlns:p="urn:items"' : 'xmlns="urn:items"';
    const xml = `<root><${name} ${declaration}>one</${name}><${name} ${declaration}>two</${name}></root>`;
    expect(encodeXML(value, schema, {})).toBe(xml);
    expect(decodeXML(xml, schema, {})).toEqual(value);
    expect(decodeXML(encodeXML(value, schema, {}), schema, {})).toEqual(value);
    expect(decodeXML(xml.replaceAll("urn:items", "urn:other"), schema, {})).toEqual({ values: [] });
  });
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
