import type { MediaCodec } from "./wire-engine.js";
import { defineOwnDataProperty, isRecord } from "./runtime-support.js";
import { extendDynamicScope, resolveDynamicReference, type WireXML } from "./wire-engine.js";
import { createWireCodec } from "./wire-engine.js";
import { decodeSchemaContent } from "./wire-engine.js";
import { isJSONMediaType, isXMLMediaType } from "./runtime-support.js";
import type { DynamicScope, WireSchema, WireSchemas, WireTransformOptions } from "./wire-engine.js";

/** Retains the existing codec facade exports from ./wire-engine.js. */
export type * from "./wire-engine.js";
/** Retains the existing codec facade exports from ./media-type.js. */
export { isJSONMediaType, isXMLMediaType } from "./runtime-support.js";

function decodeExtendedSchemaContent(
  value: string,
  mediaType: string,
  schema: WireSchema,
  components: WireSchemas,
): unknown {
  if (isXMLMediaType(mediaType)) return decodeXML(value, schema.contentSchema ?? {}, components);
  throw new TypeError(`unsupported contentMediaType ${mediaType}`);
}

/** Wire codec with XML content support, shared by generated execution plans. */
export const xmlWireCodec = /* @__PURE__ */ createWireCodec((value, schema, components, ignore) =>
  decodeSchemaContent(value, schema, components, ignore, decodeExtendedSchemaContent),
);

/** Recursively maps a value between generated property names and wire names. */
export function transformWireValue(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions | undefined = undefined,
  dynamicScope: DynamicScope = [],
): unknown {
  return xmlWireCodec.transformWireValue(
    value,
    schema,
    components,
    direction,
    options,
    dynamicScope,
  );
}

/** Converts a validated JSON wire value into generated TypeScript property names. */
export function decodeWireValue(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
): unknown {
  return xmlWireCodec.decodeWireValue(value, schema, components);
}

/** Converts generated TypeScript property names into validated JSON wire names. */
export function encodeWireValue(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
): unknown {
  return xmlWireCodec.encodeWireValue(value, schema, components);
}

/** Validates a transformed wire value against its generated schema. */
export function validateWireValue(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions | undefined = undefined,
  dynamicScope: DynamicScope = [],
): void {
  xmlWireCodec.validateWireValue(value, schema, components, direction, options, dynamicScope);
}

/** Encodes a value using its generated XML representation. */
export function encodeXML(value: unknown, schema: WireSchema, schemas: WireSchemas): string {
  const rootName = schema.reference ?? schema.xml?.name ?? "root";
  return encodeXMLElement(value, schema, schemas, rootName, true, []);
}

function encodeXMLElement(
  value: unknown,
  schema: WireSchema,
  schemas: WireSchemas,
  fallbackName: string,
  root: boolean,
  dynamicScope: DynamicScope,
): string {
  const scope = extendDynamicScope(dynamicScope, schema);
  const dynamicTarget = resolveDynamicReference(schema, scope);
  if (dynamicTarget !== undefined)
    return encodeXMLElement(value, dynamicTarget, schemas, fallbackName, root, scope);
  if (schema.reference !== undefined) {
    const referenced = schemas[schema.reference];
    if (referenced === undefined)
      throw new TypeError(`XML schema references missing component ${schema.reference}`);
    return encodeXMLElement(value, referenced, schemas, fallbackName, root, scope);
  }
  const xml = schema.xml;
  if (xml?.nodeType === "none") return "";
  if (xml?.nodeType === "text") return escapeXMLText(xmlScalar(value));
  if (xml?.nodeType === "cdata")
    return `<![CDATA[${xmlScalar(value).replaceAll("]]>", "]]]]><![CDATA[>")}]]>`;
  const name = xmlName(xml, fallbackName);
  if (Array.isArray(value)) {
    const itemSchema = schema.items ?? {};
    const wrapped = xmlArrayWrapped(xml);
    const itemName = itemSchema.xml?.name ?? (wrapped ? fallbackName : name);
    const values = value
      .map((item) => encodeXMLElement(item, itemSchema, schemas, itemName, false, scope))
      .join("");
    return wrapped ? wrapXML(name, namespaceAttributes(xml, root), values) : values;
  }
  if (!isRecord(value))
    return wrapXML(name, namespaceAttributes(xml, root), escapeXMLText(xmlScalar(value)));
  const attributes: string[] = [];
  const children: string[] = [];
  let text = "";
  for (const [wireName, property] of Object.entries(schema.properties ?? {})) {
    const item = value[wireName];
    if (item === undefined || item === null) continue;
    const childXML = property.schema.xml;
    const childName = childXML?.name ?? wireName;
    if (childXML?.attribute || childXML?.nodeType === "attribute") {
      attributes.push(`${xmlName(childXML, childName)}="${escapeXMLAttribute(xmlScalar(item))}"`);
      continue;
    }
    if (childXML?.nodeType === "text") {
      text += escapeXMLText(xmlScalar(item));
      continue;
    }
    if (childXML?.nodeType === "cdata") {
      text += `<![CDATA[${xmlScalar(item).replaceAll("]]>", "]]]]><![CDATA[>")}]]>`;
      continue;
    }
    children.push(encodeXMLElement(item, property.schema, schemas, childName, false, scope));
  }
  return wrapXML(
    name,
    [...namespaceAttributes(xml, root), ...attributes],
    text + children.join(""),
  );
}

function wrapXML(name: string, attributes: readonly string[], content: string): string {
  const start = `<${name}${attributes.length === 0 ? "" : ` ${attributes.join(" ")}`}>`;
  return `${start}${content}</${name}>`;
}

function xmlName(xml: WireXML | undefined, fallback: string): string {
  const name = xml?.name ?? fallback;
  return xml?.prefix === undefined || xml.prefix === "" ? name : `${xml.prefix}:${name}`;
}

function xmlArrayWrapped(xml: WireXML | undefined): boolean {
  return xml?.wrapped === true || xml?.nodeType === "element";
}

function namespaceAttributes(xml: WireXML | undefined, include: boolean): string[] {
  if (!include || xml?.namespace === undefined || xml.namespace === "") return [];
  return [
    xml.prefix === undefined || xml.prefix === ""
      ? `xmlns="${escapeXMLAttribute(xml.namespace)}"`
      : `xmlns:${xml.prefix}="${escapeXMLAttribute(xml.namespace)}"`,
  ];
}

function xmlScalar(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean" || typeof value === "bigint")
    return String(value);
  if (value === null || value === undefined) return "";
  throw new TypeError(
    "XML scalar node requires a string, number, boolean, bigint, null, or undefined value",
  );
}

function escapeXMLText(value: string): string {
  return value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
}

function escapeXMLAttribute(value: string): string {
  return escapeXMLText(value).replaceAll('"', "&quot;").replaceAll("'", "&apos;");
}

interface XMLNode {
  readonly name: string;
  readonly attributes: Readonly<Record<string, string>>;
  readonly children: XMLNode[];
  text: string;
}

/** Decodes one XML representation using the generated OpenAPI XML schema metadata. */
export function decodeXML(source: string, schema: WireSchema, components: WireSchemas): unknown {
  return decodeXMLNode(parseXMLDocument(source), schema, components, []);
}

function parseXMLDocument(source: string): XMLNode {
  const tokens =
    source.match(/<!\[CDATA\[[\s\S]*?\]\]>|<!--[\s\S]*?-->|<\?[^]*?\?>|<[^>]+>|[^<]+/g) ?? [];
  const roots: XMLNode[] = [];
  const stack: XMLNode[] = [];
  for (const token of tokens) {
    if (token.startsWith("<!--") || token.startsWith("<?")) continue;
    if (token.startsWith("<![CDATA[")) {
      if (stack.length === 0)
        throw new TypeError("XML character data appears outside the document element");
      stack[stack.length - 1]!.text += token.slice(9, -3);
      continue;
    }
    if (token.startsWith("<!"))
      throw new TypeError("XML declarations other than comments and CDATA are unsupported");
    if (token.startsWith("</")) {
      const name = token.slice(2, -1).trim();
      const current = stack.pop();
      if (current === undefined || current.name !== name)
        throw new TypeError(`XML closing tag ${name} does not match the open element`);
      continue;
    }
    if (token.startsWith("<")) {
      const selfClosing = /\/>$/.test(token);
      const body = token.slice(1, selfClosing ? -2 : -1).trim();
      const match = /^([^\s/>]+)([\s\S]*)$/.exec(body);
      if (match === null) throw new TypeError("XML element has no name");
      const node: XMLNode = {
        name: match[1]!,
        attributes: parseXMLAttributes(match[2] ?? ""),
        children: [],
        text: "",
      };
      if (stack.length === 0) roots.push(node);
      else stack[stack.length - 1]!.children.push(node);
      if (!selfClosing) stack.push(node);
      continue;
    }
    if (stack.length === 0) {
      if (token.trim() !== "") throw new TypeError("XML text appears outside the document element");
      continue;
    }
    stack[stack.length - 1]!.text += unescapeXML(token);
  }
  if (stack.length !== 0 || roots.length !== 1)
    throw new TypeError("XML document must contain one balanced root element");
  return roots[0]!;
}

function parseXMLAttributes(source: string): Readonly<Record<string, string>> {
  const result = Object.create(null) as Record<string, string>;
  const expression = /([^\s=]+)\s*=\s*("[^"]*"|'[^']*')/g;
  let match: RegExpExecArray | null;
  while ((match = expression.exec(source)) !== null)
    defineOwnDataProperty(result, match[1]!, unescapeXML(match[2]!.slice(1, -1)));
  if (source.replace(expression, "").trim() !== "")
    throw new TypeError("XML attribute syntax is invalid");
  return result;
}

function decodeXMLNode(
  node: XMLNode,
  schema: WireSchema,
  components: WireSchemas,
  dynamicScope: DynamicScope,
): unknown {
  const scope = extendDynamicScope(dynamicScope, schema);
  const dynamicTarget = resolveDynamicReference(schema, scope);
  if (dynamicTarget !== undefined) return decodeXMLNode(node, dynamicTarget, components, scope);
  if (schema.reference !== undefined) {
    const referenced = components[schema.reference];
    if (referenced === undefined)
      throw new TypeError(`XML schema references missing component ${schema.reference}`);
    return decodeXMLNode(node, referenced, components, scope);
  }
  if (schema.types?.includes("array")) {
    const itemSchema = schema.items ?? {};
    return node.children.map((child) => decodeXMLNode(child, itemSchema, components, scope));
  }
  if (schema.types?.includes("object") || schema.properties !== undefined) {
    const result = Object.create(null) as Record<string, unknown>;
    for (const [wireName, property] of Object.entries(schema.properties ?? {})) {
      const xml = property.schema.xml;
      const name = xmlName(xml, wireName);
      if (xml?.attribute || xml?.nodeType === "attribute") {
        const value = node.attributes[name];
        if (value !== undefined)
          defineOwnDataProperty(result, wireName, decodeXMLScalar(value, property.schema));
        continue;
      }
      if (property.schema.types?.includes("array")) {
        const itemSchema = property.schema.items ?? {};
        const container = xmlArrayWrapped(xml)
          ? node.children.find((child) => child.name === name)
          : node;
        if (container !== undefined) {
          const itemName = xmlName(itemSchema.xml, itemSchema.xml?.name ?? wireName);
          defineOwnDataProperty(
            result,
            wireName,
            container.children
              .filter((child) => child.name === itemName)
              .map((child) => decodeXMLNode(child, itemSchema, components, scope)),
          );
        }
        continue;
      }
      const child = node.children.find((entry) => entry.name === name);
      if (child !== undefined)
        defineOwnDataProperty(
          result,
          wireName,
          decodeXMLNode(child, property.schema, components, scope),
        );
    }
    return result;
  }
  return decodeXMLScalar(node.text, schema);
}

function decodeXMLScalar(value: string, schema: WireSchema): unknown {
  if (schema.types?.includes("integer")) {
    const parsed = Number(value);
    if (!Number.isInteger(parsed)) throw new TypeError("XML value is not an integer");
    return parsed;
  }
  if (schema.types?.includes("number")) {
    const parsed = Number(value);
    if (!Number.isFinite(parsed)) throw new TypeError("XML value is not a number");
    return parsed;
  }
  if (schema.types?.includes("boolean")) {
    if (value === "true") return true;
    if (value === "false") return false;
    throw new TypeError("XML value is not a boolean");
  }
  return value;
}

function unescapeXML(value: string): string {
  return value
    .replace(
      /&#(?:x([0-9a-fA-F]+)|([0-9]+));/gu,
      (_, hexadecimal: string | undefined, decimal: string | undefined) => {
        const codePoint = Number.parseInt(
          hexadecimal ?? decimal ?? "",
          hexadecimal === undefined ? 10 : 16,
        );
        if (!isXMLCharacterCodePoint(codePoint))
          throw new TypeError("XML character reference is invalid");
        return String.fromCodePoint(codePoint);
      },
    )
    .replaceAll("&lt;", "<")
    .replaceAll("&gt;", ">")
    .replaceAll("&quot;", '"')
    .replaceAll("&apos;", "'")
    .replaceAll("&amp;", "&");
}

function isXMLCharacterCodePoint(value: number): boolean {
  return (
    value === 0x9 ||
    value === 0xa ||
    value === 0xd ||
    (value >= 0x20 && value <= 0xd7ff) ||
    (value >= 0xe000 && value <= 0xfffd) ||
    (value >= 0x10000 && value <= 0x10ffff)
  );
}

/** Buffered media handlers shared by XML plans without importing request orchestration. */
export const bufferedXMLCodecExtensions = {
  encodeRequestBody(
    contentType: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
  ): BodyInit | Promise<BodyInit> {
    const normalized = contentType.toLowerCase();
    if (isJSONMediaType(normalized)) return JSON.stringify(value);
    if (isXMLMediaType(normalized)) return encodeXML(value, schema ?? {}, schemas);
    if (normalized.startsWith("text/")) return String(value);
    if (value instanceof Blob || value instanceof ArrayBuffer || ArrayBuffer.isView(value)) {
      return value as BodyInit;
    }
    const codec = codecs.get(contentType.split(";", 1)[0]?.trim().toLowerCase() ?? "");
    if (codec?.encode === undefined) throw new TypeError("missing encode codec for " + contentType);
    return codec.encode(value, { contentType });
  },
  encodeXML,
  decodeXML,
};
