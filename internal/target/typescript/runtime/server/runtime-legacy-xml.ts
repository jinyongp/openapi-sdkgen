import type { WireSchema, WireSchemas, WireXML } from "../schema/wire-types.js";
import { defineOwnDataProperty } from "../shared/runtime-support.js";
import {
  wireArrayItemSchema as inboundWireArrayItemSchema,
  wirePropertyNames as inboundWirePropertyNames,
  wirePropertySchema as inboundWirePropertySchema,
  wireSchemaTypes as inboundWireSchemaTypes,
} from "../schema/schema-query.js";
import type {
  InboundSchema,
  InboundSchemas,
  InboundXMLNode,
  ServerCodecContext,
} from "./runtime-types.js";
import { isRecord } from "../shared/runtime-support.js";
import { inboundSchemaRecord } from "./runtime-shared.js";
import { materializeInboundWireSchema } from "./runtime-shared.js";
import { inboundWireSchemaAlternatives } from "./runtime-shared.js";
import { resolveInboundSchema } from "./runtime-shared.js";
import { schemaAcceptsType } from "./runtime-shared.js";
import { inboundArrayItemSchema } from "./runtime-shared.js";
import { isInboundSchema } from "./runtime-shared.js";
import { decodeInboundParameterValue } from "./runtime-shared.js";

export function decodeLegacyXML(
  codecContext: ServerCodecContext,
  source: string,
  schema: InboundSchema | undefined,
  schemas: InboundSchemas,
  wireSchema: WireSchema | undefined = undefined,
  wireSchemas: WireSchemas | undefined = undefined,
): unknown {
  const root: InboundXMLNode = parseInboundXML(source);
  const schemaXML: Readonly<Record<string, unknown>> | undefined = isRecord(
    inboundSchemaRecord(schema)["xml"],
  )
    ? (inboundSchemaRecord(schema)["xml"] as Readonly<Record<string, unknown>>)
    : undefined;
  const rootName: string =
    wireSchema?.reference ??
    wireSchema?.xml?.name ??
    (typeof schemaXML?.["name"] === "string" ? schemaXML["name"] : "root");
  if (wireSchema !== undefined)
    wireSchema = materializeInboundWireSchema(wireSchema, wireSchemas ?? {});
  return decodeInboundXMLNode(
    codecContext,
    root,
    schema ?? {},
    schemas,
    wireSchema,
    wireSchemas ?? {},
    false,
    rootName,
  );
}

export function parseInboundXML(source: string): InboundXMLNode {
  const tokens: string[] =
    source.match(/<!\[CDATA\[[\s\S]*?\]\]>|<!--[\s\S]*?-->|<\?[^]*?\?>|<[^>]+>|[^<]+/g) ?? [];
  const roots: InboundXMLNode[] = [];
  const stack: InboundXMLNode[] = [];
  for (const token of tokens) {
    if (token.startsWith("<!--") || token.startsWith("<?")) continue;
    if (token.startsWith("<![CDATA[")) {
      if (stack.length === 0)
        throw new TypeError("XML character data is outside the document element");
      stack[stack.length - 1]!.text += token.slice(9, -3);
      stack[stack.length - 1]!.hasText = true;
      continue;
    }
    if (token.startsWith("<!")) throw new TypeError("XML declarations are not supported");
    if (token.startsWith("</")) {
      const name: string = token.slice(2, -1).trim();
      const node: InboundXMLNode | undefined = stack.pop();
      if (node === undefined || node.name !== name) throw new TypeError("XML closing tag mismatch");
      continue;
    }
    if (token.startsWith("<")) {
      const closing: boolean = /\/>$/.test(token);
      const body: string = token.slice(1, closing ? -2 : -1).trim();
      const match: RegExpExecArray | null = /^([^\s/>]+)([\s\S]*)$/.exec(body);
      if (match === null) throw new TypeError("XML element has no name");
      const node: InboundXMLNode = {
        name: match[1]!,
        attributes: parseInboundXMLAttributes(match[2] ?? ""),
        children: [],
        text: "",
        hasText: false,
      };
      if (stack.length === 0) roots.push(node);
      else stack[stack.length - 1]!.children.push(node);
      if (!closing) stack.push(node);
      continue;
    }
    if (stack.length === 0) {
      if (token.trim() !== "") throw new TypeError("XML text is outside the document element");
      continue;
    }
    stack[stack.length - 1]!.text += unescapeInboundXML(token);
    stack[stack.length - 1]!.hasText = true;
  }
  if (stack.length !== 0 || roots.length !== 1) throw new TypeError("XML document is not balanced");
  return roots[0]!;
}

export function parseInboundXMLAttributes(source: string): Readonly<Record<string, string>> {
  const result: Record<string, string> = Object.create(null) as Record<string, string>;
  const expression: RegExp = /([^\s=]+)\s*=\s*("[^"]*"|'[^']*')/g;
  let match: RegExpExecArray | null;
  while ((match = expression.exec(source)) !== null) {
    const name: string = match[1]!;
    if (Object.hasOwn(result, name)) throw new TypeError("duplicate XML attribute " + name);
    defineOwnDataProperty(result, name, unescapeInboundXML(match[2]!.slice(1, -1)));
  }
  if (source.replace(expression, "").trim() !== "")
    throw new TypeError("XML attribute syntax is invalid");
  return result;
}

export function decodeInboundXMLNode(
  codecContext: ServerCodecContext,
  node: InboundXMLNode,
  schema: InboundSchema,
  schemas: InboundSchemas,
  wireSchema: WireSchema | undefined = undefined,
  wireSchemas: WireSchemas = {},
  correlated: boolean = false,
  rootName: string = "root",
): unknown {
  if (wireSchema !== undefined && !correlated) {
    wireSchema = materializeInboundWireSchema(wireSchema, wireSchemas);
    let fallback: unknown;
    let failure: unknown;
    for (const alternative of inboundWireSchemaAlternatives(wireSchema, wireSchemas)) {
      try {
        const value: unknown = decodeInboundXMLNode(
          codecContext,
          node,
          schema,
          schemas,
          alternative,
          wireSchemas,
          true,
          rootName,
        );
        fallback = value;
        codecContext.wire.validateWireValue(value, wireSchema, wireSchemas, "decode");
        return value;
      } catch (error: unknown) {
        failure = error;
      }
    }
    if (failure !== undefined) throw failure;
    return fallback;
  }
  const resolved: Readonly<Record<string, unknown>> = inboundSchemaRecord(
    resolveInboundSchema(schema, schemas),
  );
  if (
    schemaAcceptsType(resolved["type"], "array") ||
    (wireSchema !== undefined && inboundWireSchemaTypes(wireSchema, wireSchemas).includes("array"))
  ) {
    const xml: Record<string, unknown> | WireXML = isRecord(resolved["xml"])
      ? resolved["xml"]
      : (wireSchema?.xml ?? {});
    if (inboundXMLArrayWrapped(xml)) {
      if (node.name !== inboundXMLQualifiedName(xml, rootName))
        throw new TypeError("unexpected XML array wrapper " + node.name);
      for (const name of Object.keys(node.attributes)) {
        if (name !== "xmlns" && !name.startsWith("xmlns:"))
          throw new TypeError("unexpected XML array wrapper attribute " + name);
      }
      if (node.text.trim() !== "") throw new TypeError("unexpected XML array wrapper text");
    }
    return node.children.map((child: InboundXMLNode, index: number): unknown => {
      const item: InboundSchema = inboundArrayItemSchema(resolved, index);
      const resolvedItem: InboundSchema = resolveInboundSchema(item, schemas);
      const wireItem: WireSchema | undefined =
        wireSchema === undefined
          ? undefined
          : inboundWireArrayItemSchema(wireSchema, index, wireSchemas);
      const itemDescriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(resolvedItem);
      const itemXML: Record<string, unknown> | WireXML = isRecord(itemDescriptor["xml"])
        ? itemDescriptor["xml"]
        : (wireItem?.xml ?? {});
      const parentItemFallbackName: string = inboundXMLArrayWrapped(xml)
        ? rootName
        : inboundXMLQualifiedName(xml, rootName);
      const itemFallbackName: string =
        typeof itemXML?.name === "string" ? itemXML.name : parentItemFallbackName;
      if (inboundXMLArrayWrapped(xml) && child.name !== inboundXMLQualifiedName(itemXML, rootName))
        throw new TypeError("unexpected XML array item " + child.name);
      return decodeInboundXMLNode(
        codecContext,
        child,
        resolvedItem,
        schemas,
        wireItem,
        wireSchemas,
        false,
        itemFallbackName,
      );
    });
  }
  const properties: Record<string, unknown> = isRecord(resolved["properties"])
    ? resolved["properties"]
    : {};
  const wirePropertyNames: readonly string[] =
    wireSchema === undefined ? [] : inboundWirePropertyNames(wireSchema, wireSchemas);
  if (
    schemaAcceptsType(resolved["type"], "object") ||
    Object.keys(properties).length !== 0 ||
    (wireSchema !== undefined &&
      (inboundWireSchemaTypes(wireSchema, wireSchemas).includes("object") ||
        wirePropertyNames.length !== 0))
  ) {
    const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
    const consumedAttributes: Set<string> = new Set<string>();
    const consumedChildren: Set<InboundXMLNode> = new Set<InboundXMLNode>();
    for (const name of new Set([...Object.keys(properties), ...wirePropertyNames])) {
      const childSchema: InboundSchema = isInboundSchema(properties[name]) ? properties[name] : {};
      const resolvedChild: InboundSchema = resolveInboundSchema(childSchema, schemas);
      const childDescriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(resolvedChild);
      const wireProperty: WireSchema | undefined =
        wireSchema === undefined
          ? undefined
          : inboundWirePropertySchema(wireSchema, name, wireSchemas);
      const xml: Record<string, unknown> | WireXML = isRecord(childDescriptor["xml"])
        ? childDescriptor["xml"]
        : (wireProperty?.xml ?? {});
      const xmlName: string = inboundXMLQualifiedName(xml, name);
      if (xml["attribute"] === true || xml["nodeType"] === "attribute") {
        if (node.attributes[xmlName] !== undefined) {
          consumedAttributes.add(xmlName);
          defineOwnDataProperty(
            result,
            name,
            decodeInboundXMLScalar(
              codecContext,
              node.attributes[xmlName]!,
              resolvedChild,
              wireProperty,
              wireSchemas,
            ),
          );
        }
        continue;
      }
      if (xml["nodeType"] === "text" || xml["nodeType"] === "cdata") {
        if (node.hasText)
          defineOwnDataProperty(
            result,
            name,
            decodeInboundXMLScalar(
              codecContext,
              node.text,
              resolvedChild,
              wireProperty,
              wireSchemas,
            ),
          );
        continue;
      }
      if (
        schemaAcceptsType(childDescriptor["type"], "array") ||
        (wireProperty !== undefined &&
          inboundWireSchemaTypes(wireProperty, wireSchemas).includes("array"))
      ) {
        const container: InboundXMLNode | undefined = inboundXMLArrayWrapped(xml)
          ? node.children.find((child: InboundXMLNode): boolean => child.name === xmlName)
          : node;
        if (container !== undefined) {
          if (container !== node) {
            consumedChildren.add(container);
            for (const name of Object.keys(container.attributes)) {
              if (name !== "xmlns" && !name.startsWith("xmlns:"))
                throw new TypeError("unexpected XML array wrapper attribute " + name);
            }
            if (container.text.trim() !== "")
              throw new TypeError("unexpected XML array wrapper text");
          }
          const values: unknown[] = [];
          for (const child of container.children) {
            const item: InboundSchema = inboundArrayItemSchema(childDescriptor, values.length);
            const resolvedItem: InboundSchema = resolveInboundSchema(item, schemas);
            const itemDescriptor: Readonly<Record<string, unknown>> =
              inboundSchemaRecord(resolvedItem);
            const wireItem: WireSchema | undefined =
              wireProperty === undefined
                ? undefined
                : inboundWireArrayItemSchema(wireProperty, values.length, wireSchemas);
            const itemXML: Record<string, unknown> | WireXML = isRecord(itemDescriptor["xml"])
              ? itemDescriptor["xml"]
              : (wireItem?.xml ?? {});
            const wrapperFallbackName: string = typeof xml?.name === "string" ? xml.name : name;
            const parentItemFallbackName: string = inboundXMLArrayWrapped(xml)
              ? wrapperFallbackName
              : xmlName;
            const itemFallbackName: string =
              typeof itemXML?.name === "string" ? itemXML.name : parentItemFallbackName;
            const itemName: string = inboundXMLQualifiedName(itemXML, parentItemFallbackName);
            if (inboundXMLArrayWrapped(xml) && child.name !== itemName)
              throw new TypeError("unexpected XML array item " + child.name);
            if (!inboundXMLArrayWrapped(xml) && child.name !== xmlName && child.name !== itemName)
              continue;
            consumedChildren.add(child);
            values.push(
              decodeInboundXMLNode(
                codecContext,
                child,
                resolvedItem,
                schemas,
                wireItem,
                wireSchemas,
                false,
                itemFallbackName,
              ),
            );
          }
          defineOwnDataProperty(result, name, values);
        }
        continue;
      }
      const child: InboundXMLNode | undefined = node.children.find(
        (entry: InboundXMLNode): boolean => entry.name === xmlName,
      );
      if (child !== undefined) {
        consumedChildren.add(child);
        const childFallbackName: string = typeof xml?.name === "string" ? xml.name : name;
        defineOwnDataProperty(
          result,
          name,
          decodeInboundXMLNode(
            codecContext,
            child,
            resolvedChild,
            schemas,
            wireProperty,
            wireSchemas,
            false,
            childFallbackName,
          ),
        );
      }
    }
    for (const [name, value] of Object.entries(node.attributes)) {
      if (consumedAttributes.has(name)) continue;
      if (name === "xmlns" || name.startsWith("xmlns:")) continue;
      const wireProperty: WireSchema | undefined =
        wireSchema === undefined
          ? undefined
          : inboundWirePropertySchema(wireSchema, name, wireSchemas);
      defineOwnDataProperty(
        result,
        name,
        wireProperty === undefined
          ? value
          : decodeInboundXMLScalar(codecContext, value, {}, wireProperty, wireSchemas),
      );
    }
    for (const child of node.children) {
      if (consumedChildren.has(child)) continue;
      const wireProperty: WireSchema | undefined =
        wireSchema === undefined
          ? undefined
          : inboundWirePropertySchema(wireSchema, child.name, wireSchemas);
      const value: unknown =
        wireProperty === undefined
          ? decodeInboundUnknownXMLNode(child)
          : decodeInboundXMLNode(
              codecContext,
              child,
              {},
              schemas,
              wireProperty,
              wireSchemas,
              false,
              child.name,
            );
      const previous: unknown = result[child.name];
      defineOwnDataProperty(
        result,
        child.name,
        previous === undefined
          ? value
          : Array.isArray(previous)
            ? [...previous, value]
            : [previous, value],
      );
    }
    return result;
  }
  return decodeInboundXMLScalar(codecContext, node.text, resolved, wireSchema, wireSchemas);
}

export function decodeInboundUnknownXMLNode(node: InboundXMLNode): unknown {
  if (node.children.length === 0 && Object.keys(node.attributes).length === 0) return node.text;
  const result: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
  for (const [name, value] of Object.entries(node.attributes))
    defineOwnDataProperty(result, name, value);
  for (const child of node.children) {
    const value: unknown = decodeInboundUnknownXMLNode(child);
    const previous: unknown = result[child.name];
    defineOwnDataProperty(
      result,
      child.name,
      previous === undefined
        ? value
        : Array.isArray(previous)
          ? [...previous, value]
          : [previous, value],
    );
  }
  if (node.text.trim() !== "") defineOwnDataProperty(result, "#text", node.text);
  return result;
}

export function inboundXMLQualifiedName(
  xml: Readonly<Record<string, unknown>> | WireSchema["xml"],
  fallback: string,
): string {
  const name: string = typeof xml?.name === "string" ? xml.name : fallback;
  return typeof xml?.prefix === "string" && xml.prefix !== "" ? xml.prefix + ":" + name : name;
}

export function inboundXMLArrayWrapped(
  xml: Readonly<Record<string, unknown>> | WireSchema["xml"],
): boolean {
  return xml?.wrapped === true || xml?.nodeType === "element";
}

export function decodeInboundXMLScalar(
  codecContext: ServerCodecContext,
  value: string,
  schema: InboundSchema,
  wireSchema: WireSchema | undefined = undefined,
  wireSchemas: WireSchemas = {},
): unknown {
  if (wireSchema !== undefined)
    return decodeInboundParameterValue(codecContext, value, schema, {}, wireSchema, wireSchemas);
  const descriptor: Readonly<Record<string, unknown>> = inboundSchemaRecord(schema);
  if (schemaAcceptsType(descriptor["type"], "integer")) {
    const parsed: number = Number(value);
    if (!Number.isInteger(parsed)) throw new TypeError("XML value is not an integer");
    return parsed;
  }
  if (schemaAcceptsType(descriptor["type"], "number")) {
    const parsed: number = Number(value);
    if (!Number.isFinite(parsed)) throw new TypeError("XML value is not a number");
    return parsed;
  }
  if (schemaAcceptsType(descriptor["type"], "boolean")) {
    if (value === "true") return true;
    if (value === "false") return false;
    throw new TypeError("XML value is not a boolean");
  }
  return value;
}

export function unescapeInboundXML(value: string): string {
  return value
    .replace(
      /&#(?:x([0-9a-fA-F]+)|([0-9]+));/gu,
      (_: string, hexadecimal: string | undefined, decimal: string | undefined): string => {
        const codePoint: number = Number.parseInt(
          hexadecimal ?? decimal ?? "",
          hexadecimal === undefined ? 10 : 16,
        );
        if (!isInboundXMLCharacterCodePoint(codePoint))
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

export function isInboundXMLCharacterCodePoint(value: number): boolean {
  return (
    value === 0x9 ||
    value === 0xa ||
    value === 0xd ||
    (value >= 0x20 && value <= 0xd7ff) ||
    (value >= 0xe000 && value <= 0xfffd) ||
    (value >= 0x10000 && value <= 0x10ffff)
  );
}
