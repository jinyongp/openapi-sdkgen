import type { MediaCodec, WireCodec, WireProperty } from "./wire-engine.js";
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
export const xmlWireCodec: WireCodec = /* @__PURE__ */ createWireCodec(
  (
    value: string,
    schema: WireSchema,
    components: WireSchemas,
    ignore: boolean | undefined,
  ): unknown => decodeSchemaContent(value, schema, components, ignore, decodeExtendedSchemaContent),
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
  const rootName: string = schema.reference ?? schema.xml?.name ?? "root";
  return encodeXMLElement(value, schema, schemas, rootName, [], { xml: XML_NAMESPACE });
}

/** Resolves only the current XML node; recursive property schemas remain lazy. */
function xmlRepresentation(
  schema: WireSchema,
  components: WireSchemas,
  dynamicScope: DynamicScope,
  seen: Set<WireSchema> = new Set<WireSchema>(),
): WireSchema {
  if (seen.has(schema)) return {};
  const path: Set<WireSchema> = new Set(seen).add(schema);
  const scope: DynamicScope = extendDynamicScope(dynamicScope, schema);
  const parents: WireSchema[] = [];
  const dynamicTarget: WireSchema | undefined = resolveDynamicReference(schema, scope);
  if (dynamicTarget !== undefined) parents.push(dynamicTarget);
  if (schema.reference !== undefined) {
    const target: WireSchema | undefined = components[schema.reference];
    if (target === undefined)
      throw new TypeError(`XML schema references missing component ${schema.reference}`);
    parents.push(target);
  }
  parents.push(...(schema.allOf ?? []));
  const own: MutableWireSchema = { ...schema };
  delete own.reference;
  delete own.dynamicReference;
  delete own.allOf;
  const representations: WireSchema[] = parents.map((parent: WireSchema): WireSchema =>
    xmlRepresentation(parent, components, scope, path),
  );
  representations.push(own);
  let result: WireSchema = {};
  for (const representation of representations) {
    const properties: Record<string, WireProperty> = Object.assign(
      Object.create(null),
      result.properties,
    ) as Record<string, NonNullable<WireSchema["properties"]>[string]>;
    for (const [name, property] of Object.entries(representation.properties ?? {})) {
      const previous: WireProperty | undefined = properties[name];
      defineOwnDataProperty(
        properties,
        name,
        previous === undefined
          ? property
          : {
              ...property,
              schema: { allOf: [previous.schema, property.schema] },
            },
      );
    }
    result = {
      ...result,
      ...representation,
      ...(Object.keys(properties).length === 0 ? {} : { properties }),
    };
  }
  return result;
}

function xmlProperties(
  schema: WireSchema,
  components: WireSchemas,
  scope: DynamicScope,
): [string, WireSchema][] {
  const result: [string, WireSchema][] = [];
  const names: Set<string> = new Set<string>();
  for (const [wireName, property] of Object.entries(schema.properties ?? {})) {
    const child: WireSchema = xmlRepresentation(property.schema, components, scope);
    const xml: WireXML | undefined = child.xml;
    if (xml?.nodeType === "none") continue;
    const name: string =
      xml?.nodeType === "text" || xml?.nodeType === "cdata"
        ? "content"
        : `${xml?.attribute || xml?.nodeType === "attribute" ? "attribute" : "element"}:${xml?.namespace ?? ""}:${xml?.name ?? wireName}`;
    if (names.has(name))
      throw new TypeError(`XML properties have an ambiguous representation: ${wireName}`);
    names.add(name);
    result.push([wireName, child]);
  }
  return result;
}

function xmlArrayItem(
  schema: WireSchema,
  schemas: WireSchemas,
  scope: DynamicScope,
  fallbackName: string,
): XMLArrayItem {
  const xml: WireXML | undefined = schema.xml;
  const item: WireSchema = xmlRepresentation(schema.items ?? {}, schemas, scope);
  const wrapped: boolean = xmlArrayWrapped(xml);
  return {
    name: item.xml?.name ?? (wrapped ? fallbackName : (xml?.name ?? fallbackName)),
    wrapped,
    schema: wrapped
      ? item
      : {
          ...item,
          xml: {
            ...(xml?.prefix === undefined ? {} : { prefix: xml.prefix }),
            ...(xml?.namespace === undefined ? {} : { namespace: xml.namespace }),
            ...item.xml,
          },
        },
  };
}

function encodeXMLElement(
  value: unknown,
  schema: WireSchema,
  schemas: WireSchemas,
  fallbackName: string,
  dynamicScope: DynamicScope,
  inheritedNamespaces: Readonly<Record<string, string>>,
): string {
  const scope: DynamicScope = extendDynamicScope(dynamicScope, schema);
  schema = xmlRepresentation(schema, schemas, scope);
  const xml: WireXML | undefined = schema.xml;
  if (xml?.nodeType === "none") return "";
  if (xml?.nodeType === "text") return escapeXMLText(xmlScalar(value));
  if (xml?.nodeType === "cdata")
    return `<![CDATA[${xmlScalar(value).replaceAll("]]>", "]]]]><![CDATA[>")}]]>`;
  const name: string = xmlName(xml, fallbackName);
  const namespaces: Record<string, string> = Object.assign(
    Object.create(null),
    inheritedNamespaces,
  ) as Record<string, string>;
  const declarations: string[] = namespaceAttributes(xml, namespaces);
  if (Array.isArray(value)) {
    const {
      schema: itemSchema,
      name: itemName,
      wrapped,
    }: XMLArrayItem = xmlArrayItem(schema, schemas, scope, fallbackName);
    if (wrapped) expandedXMLName(name, namespaces, false);
    const values: string = value
      .map((item: unknown): string =>
        encodeXMLElement(
          item,
          itemSchema,
          schemas,
          itemName,
          scope,
          wrapped ? namespaces : inheritedNamespaces,
        ),
      )
      .join("");
    return wrapped ? wrapXML(name, declarations, values) : values;
  }
  expandedXMLName(name, namespaces, false);
  if (!isRecord(value)) return wrapXML(name, declarations, escapeXMLText(xmlScalar(value)));
  const attributes: string[] = [];
  const children: string[] = [];
  let text: string = "";
  for (const [wireName, childSchema] of xmlProperties(schema, schemas, scope)) {
    const item: unknown = value[wireName];
    if (item === undefined || item === null) continue;
    const childXML: WireXML | undefined = childSchema.xml;
    const childName: string = childXML?.name ?? wireName;
    if (childXML?.attribute || childXML?.nodeType === "attribute") {
      declarations.push(...namespaceAttributes(childXML, namespaces));
      expandedXMLName(xmlName(childXML, childName), namespaces, true);
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
    children.push(encodeXMLElement(item, childSchema, schemas, childName, scope, namespaces));
  }
  return wrapXML(name, [...declarations, ...attributes], text + children.join(""));
}

function wrapXML(name: string, attributes: readonly string[], content: string): string {
  const start: string = `<${name}${attributes.length === 0 ? "" : ` ${attributes.join(" ")}`}>`;
  return `${start}${content}</${name}>`;
}

function xmlName(xml: WireXML | undefined, fallback: string): string {
  const name: string = xml?.name ?? fallback;
  return xml?.prefix === undefined || xml.prefix === "" ? name : `${xml.prefix}:${name}`;
}

function xmlArrayWrapped(xml: WireXML | undefined): boolean {
  return xml?.wrapped === true || xml?.nodeType === "element";
}

const XML_NAMESPACE: "http://www.w3.org/XML/1998/namespace" =
  "http://www.w3.org/XML/1998/namespace";
const XMLNS_NAMESPACE: "http://www.w3.org/2000/xmlns/" = "http://www.w3.org/2000/xmlns/";

function namespaceAttributes(
  xml: WireXML | undefined,
  namespaces: Record<string, string>,
): string[] {
  if (xml?.namespace === undefined || xml.namespace === "") return [];
  const prefix: string = xml.prefix ?? "";
  validateNamespace(prefix, xml.namespace);
  if (namespaces[prefix] === xml.namespace) return [];
  defineOwnDataProperty(namespaces, prefix, xml.namespace);
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
  readonly namespaces: Readonly<Record<string, string>>;
  text: string;
}

/** Decodes one XML representation using the generated OpenAPI XML schema metadata. */
export function decodeXML(source: string, schema: WireSchema, components: WireSchemas): unknown {
  return decodeXMLNode(parseXMLDocument(source), schema, components, []);
}

function parseXMLDocument(source: string): XMLNode {
  source = source.replace(/\r\n?/g, "\n");
  for (const character of source)
    if (!isXMLCharacterCodePoint(character.codePointAt(0)!))
      throw new TypeError("XML character is invalid");
  const roots: XMLNode[] = [];
  const stack: XMLNode[] = [];
  for (const token of xmlTokens(source)) {
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
      const name: string = token.slice(2, -1).trim();
      const current: XMLNode | undefined = stack.pop();
      if (current === undefined || current.name !== name)
        throw new TypeError(`XML closing tag ${name} does not match the open element`);
      continue;
    }
    if (token.startsWith("<")) {
      const selfClosing: boolean = /\/>$/.test(token);
      const body: string = token.slice(1, selfClosing ? -2 : -1).trim();
      const match: RegExpExecArray | null = /^([^\s/>]+)([\s\S]*)$/.exec(body);
      if (match === null) throw new TypeError("XML element has no name");
      const node: XMLNode = {
        name: match[1]!,
        attributes: parseXMLAttributes(match[2] ?? ""),
        children: [],
        text: "",
        namespaces: Object.assign(
          Object.create(null),
          stack.at(-1)?.namespaces ?? { xml: XML_NAMESPACE },
        ),
      };
      for (const [key, uri] of Object.entries(node.attributes)) {
        if (key !== "xmlns" && !key.startsWith("xmlns:")) continue;
        const prefix: string = key === "xmlns" ? "" : key.slice(6);
        validateNamespace(prefix, uri);
        defineOwnDataProperty(node.namespaces, prefix, uri);
      }
      expandedXMLName(node.name, node.namespaces, false);
      const attributes: Set<string> = new Set<string>();
      for (const key of Object.keys(node.attributes)) {
        if (key === "xmlns" || key.startsWith("xmlns:")) continue;
        const expanded: string = expandedXMLName(key, node.namespaces, true);
        if (attributes.has(expanded)) throw new TypeError("XML attribute is duplicated");
        attributes.add(expanded);
      }
      if (stack.length === 0) roots.push(node);
      else stack[stack.length - 1]!.children.push(node);
      if (!selfClosing) stack.push(node);
      continue;
    }
    if (stack.length === 0) {
      if (token.trim() !== "") throw new TypeError("XML text appears outside the document element");
      continue;
    }
    if (token.includes("]]>")) throw new TypeError("XML text contains a CDATA terminator");
    stack[stack.length - 1]!.text += unescapeXML(token);
  }
  if (stack.length !== 0 || roots.length !== 1)
    throw new TypeError("XML document must contain one balanced root element");
  return roots[0]!;
}

function* xmlTokens(source: string): Generator<string> {
  let offset: number = source.startsWith("\uFEFF") ? 1 : 0;
  while (offset < source.length) {
    const start: number = offset;
    if (source[offset] !== "<") {
      const end: number = source.indexOf("<", offset);
      offset = end < 0 ? source.length : end;
    } else {
      const terminator: "-->" | "]]>" | "?>" | undefined = source.startsWith("<!--", offset)
        ? "-->"
        : source.startsWith("<![CDATA[", offset)
          ? "]]>"
          : source.startsWith("<?", offset)
            ? "?>"
            : undefined;
      if (terminator !== undefined) {
        const end: number = source.indexOf(
          terminator,
          offset + (terminator === "-->" ? 4 : terminator === "]]>" ? 9 : 2),
        );
        if (end < 0) throw new TypeError("XML token is unterminated");
        offset = end + terminator.length;
        if (
          terminator === "-->" &&
          (source.slice(start + 4, end).includes("--") || source[end - 1] === "-")
        )
          throw new TypeError("XML comment is invalid");
      } else {
        let quote: string = "";
        offset++;
        for (; offset < source.length; offset++) {
          const character: string = source[offset]!;
          if (quote !== "") {
            if (character === quote) quote = "";
          } else if (character === '"' || character === "'") quote = character;
          else if (character === ">") {
            offset++;
            break;
          } else if (character === "<") throw new TypeError("XML tag is invalid");
        }
        if (quote !== "" || source[offset - 1] !== ">")
          throw new TypeError("XML token is unterminated");
      }
    }
    yield source.slice(start, offset);
  }
}

function isXMLNameStart(code: number): boolean {
  return (
    code === 95 ||
    (code >= 65 && code <= 90) ||
    (code >= 97 && code <= 122) ||
    (code >= 0xc0 && code <= 0xd6) ||
    (code >= 0xd8 && code <= 0xf6) ||
    (code >= 0xf8 && code <= 0x2ff) ||
    (code >= 0x370 && code <= 0x37d) ||
    (code >= 0x37f && code <= 0x1fff) ||
    (code >= 0x200c && code <= 0x200d) ||
    (code >= 0x2070 && code <= 0x218f) ||
    (code >= 0x2c00 && code <= 0x2fef) ||
    (code >= 0x3001 && code <= 0xd7ff) ||
    (code >= 0xf900 && code <= 0xfdcf) ||
    (code >= 0xfdf0 && code <= 0xfffd) ||
    (code >= 0x10000 && code <= 0xeffff)
  );
}

function validateXMLName(name: string): void {
  const parts: string[] = name.split(":");
  if (
    parts.length > 2 ||
    parts.some((part: string): boolean => {
      const characters: string[] = [...part];
      return (
        characters.length === 0 ||
        !isXMLNameStart(characters[0]!.codePointAt(0)!) ||
        characters.slice(1).some((character: string): boolean => {
          const code: number = character.codePointAt(0)!;
          return (
            !isXMLNameStart(code) &&
            code !== 45 &&
            code !== 46 &&
            code !== 0xb7 &&
            !(code >= 48 && code <= 57) &&
            !(code >= 0x300 && code <= 0x36f) &&
            !(code >= 0x203f && code <= 0x2040)
          );
        })
      );
    })
  )
    throw new TypeError("XML name is invalid");
}

function validateNamespace(prefix: string, uri: string): void {
  if (prefix !== "") validateXMLName(prefix);
  if (
    prefix.includes(":") ||
    prefix === "xmlns" ||
    uri === XMLNS_NAMESPACE ||
    (prefix === "xml") !== (uri === XML_NAMESPACE) ||
    (prefix !== "" && uri === "")
  )
    throw new TypeError("XML namespace declaration is invalid");
}

function expandedXMLName(
  name: string,
  namespaces: Readonly<Record<string, string>>,
  attribute: boolean,
): string {
  validateXMLName(name);
  const separator: number = name.indexOf(":");
  const prefix: string = separator < 0 ? "" : name.slice(0, separator);
  const local: string = separator < 0 ? name : name.slice(separator + 1);
  const uri: string = attribute && prefix === "" ? "" : (namespaces[prefix] ?? "");
  if (prefix !== "" && uri === "") throw new TypeError(`XML prefix ${prefix} is undeclared`);
  return `${uri}\u0000${local}`;
}

function matchesXMLName(
  name: string,
  node: XMLNode,
  xml: WireXML | undefined,
  fallback: string,
  attribute: boolean = false,
): boolean {
  const expected: string = xmlName(xml, fallback);
  if (xml?.namespace === undefined && xml?.prefix === undefined)
    return name.split(":").at(-1) === expected;
  const namespaces: Record<string, string> = Object.assign(
    Object.create(null),
    node.namespaces,
  ) as Record<string, string>;
  if (xml?.namespace !== undefined)
    defineOwnDataProperty(namespaces, xml.prefix ?? "", xml.namespace);
  return (
    expandedXMLName(name, node.namespaces, attribute) ===
    expandedXMLName(expected, namespaces, attribute)
  );
}

function parseXMLAttributes(source: string): Readonly<Record<string, string>> {
  const result: Record<string, string> = Object.create(null) as Record<string, string>;
  const expression: RegExp = /(?:^|\s+)([^\s=]+)\s*=\s*("[^"]*"|'[^']*')/g;
  let match: RegExpExecArray | null;
  while ((match = expression.exec(source)) !== null) {
    const name: string = match[1]!;
    validateXMLName(name);
    if (Object.hasOwn(result, name)) throw new TypeError("XML attribute is duplicated");
    const raw: string = match[2]!.slice(1, -1);
    if (raw.includes("<")) throw new TypeError("XML attribute syntax is invalid");
    defineOwnDataProperty(result, name, unescapeXML(raw.replace(/[\t\n]/g, " ")));
  }
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
  const scope: DynamicScope = extendDynamicScope(dynamicScope, schema);
  schema = xmlRepresentation(schema, components, scope);
  if (schema.types?.includes("array")) {
    const itemSchema: WireSchema = schema.items ?? {};
    return node.children.map((child: XMLNode): unknown =>
      decodeXMLNode(child, itemSchema, components, scope),
    );
  }
  if (schema.types?.includes("object") || schema.properties !== undefined) {
    const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
    for (const [wireName, childSchema] of xmlProperties(schema, components, scope)) {
      const xml: WireXML | undefined = childSchema.xml;
      if (xml?.nodeType === "none") continue;
      if (xml?.nodeType === "text" || xml?.nodeType === "cdata") {
        defineOwnDataProperty(result, wireName, decodeXMLScalar(node.text, childSchema));
        continue;
      }
      if (xml?.attribute || xml?.nodeType === "attribute") {
        const key: string | undefined = Object.keys(node.attributes).find(
          (key: string): boolean =>
            key !== "xmlns" &&
            !key.startsWith("xmlns:") &&
            matchesXMLName(key, node, xml, wireName, true),
        );
        const value: string | undefined = key === undefined ? undefined : node.attributes[key];
        if (value !== undefined)
          defineOwnDataProperty(result, wireName, decodeXMLScalar(value, childSchema));
        continue;
      }
      if (childSchema.types?.includes("array")) {
        const {
          schema: itemSchema,
          name: itemName,
          wrapped,
        }: XMLArrayItem = xmlArrayItem(childSchema, components, scope, wireName);
        const container: XMLNode | undefined = wrapped
          ? node.children.find((child: XMLNode): boolean =>
              matchesXMLName(child.name, child, xml, wireName),
            )
          : node;
        if (container !== undefined) {
          defineOwnDataProperty(
            result,
            wireName,
            container.children
              .filter((child: XMLNode): boolean =>
                matchesXMLName(child.name, child, itemSchema.xml, itemName),
              )
              .map((child: XMLNode): unknown =>
                decodeXMLNode(child, itemSchema, components, scope),
              ),
          );
        }
        continue;
      }
      const child: XMLNode | undefined = node.children.find((entry: XMLNode): boolean =>
        matchesXMLName(entry.name, entry, xml, wireName),
      );
      if (child !== undefined)
        defineOwnDataProperty(
          result,
          wireName,
          decodeXMLNode(child, childSchema, components, scope),
        );
    }
    return result;
  }
  return decodeXMLScalar(node.text, schema);
}

function decodeXMLScalar(value: string, schema: WireSchema): unknown {
  if (schema.types?.includes("integer")) {
    const parsed: number = Number(value);
    if (!Number.isInteger(parsed)) throw new TypeError("XML value is not an integer");
    return parsed;
  }
  if (schema.types?.includes("number")) {
    const parsed: number = Number(value);
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
  return value.replace(/&([^;]*);|&/gu, (reference: string, entity: string | undefined): string => {
    const named: Readonly<Record<string, string>> = {
      lt: "<",
      gt: ">",
      quot: '"',
      apos: "'",
      amp: "&",
    };
    if (entity !== undefined && Object.hasOwn(named, entity)) return named[entity]!;
    const numeric: RegExpExecArray | null = /^#(?:x([0-9a-fA-F]+)|([0-9]+))$/.exec(entity ?? "");
    if (numeric !== null) {
      const hexadecimal: string | undefined = numeric[1];
      const decimal: string | undefined = numeric[2];
      const codePoint: number = Number.parseInt(
        hexadecimal ?? decimal ?? "",
        hexadecimal === undefined ? 10 : 16,
      );
      if (!isXMLCharacterCodePoint(codePoint))
        throw new TypeError("XML character reference is invalid");
      return String.fromCodePoint(codePoint);
    }
    throw new TypeError(`XML character reference ${reference} is invalid`);
  });
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
interface BufferedXMLCodecExtensions {
  encodeRequestBody(
    contentType: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
  ): BodyInit | Promise<BodyInit>;
  encodeXML: typeof encodeXML;
  decodeXML: typeof decodeXML;
}

/** Buffered media handlers shared by XML plans without importing request orchestration. */
export const bufferedXMLCodecExtensions: BufferedXMLCodecExtensions = {
  encodeRequestBody(
    contentType: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
  ): BodyInit | Promise<BodyInit> {
    const normalized: string = contentType.toLowerCase();
    if (isJSONMediaType(normalized)) return JSON.stringify(value);
    if (isXMLMediaType(normalized)) return encodeXML(value, schema ?? {}, schemas);
    if (normalized.startsWith("text/")) return String(value);
    if (value instanceof Blob || value instanceof ArrayBuffer || ArrayBuffer.isView(value)) {
      return value as BodyInit;
    }
    const codec: MediaCodec<unknown> | undefined = codecs.get(
      contentType.split(";", 1)[0]?.trim().toLowerCase() ?? "",
    );
    if (codec?.encode === undefined) throw new TypeError("missing encode codec for " + contentType);
    return codec.encode(value, { contentType });
  },
  encodeXML,
  decodeXML,
};

type MutableWireSchema = { -readonly [Key in keyof WireSchema]: WireSchema[Key] };

type XMLArrayItem = {
  schema: WireSchema;
  name: string;
  wrapped: boolean;
};
