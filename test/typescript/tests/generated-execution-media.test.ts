import { describe, expect, it } from "vitest";
import { createClient } from "../fixtures/generated/execution-media/index.js";
import { createRequestContext } from "../fixtures/generated/execution-media/internal/runtime/http-execution-support.js";
import { provider as json } from "../fixtures/generated/execution-media/internal/executions/json/post.js";
import { provider as xml } from "../fixtures/generated/execution-media/internal/executions/xml/post.js";
import { provider as header } from "../fixtures/generated/execution-media/internal/executions/header/get.js";
import { provider as parameter } from "../fixtures/generated/execution-media/internal/executions/parameter/get.js";
import { provider as form } from "../fixtures/generated/execution-media/internal/executions/form/post.js";
import { provider as multipart } from "../fixtures/generated/execution-media/internal/executions/multipart/post.js";
import { provider as text } from "../fixtures/generated/execution-media/internal/executions/text/post.js";
import { provider as embedded } from "../fixtures/generated/execution-media/internal/executions/embedded/get.js";
import { provider as nested } from "../fixtures/generated/execution-media/internal/executions/nested/get.js";
import { provider as error } from "../fixtures/generated/execution-media/internal/executions/error/get.js";
import { provider as custom } from "../fixtures/generated/execution-media/internal/executions/custom/get.js";
import { provider as idless } from "../fixtures/generated/execution-media/internal/executions/plain/get.js";
import type { ClientOptions } from "../fixtures/generated/execution-media/internal/runtime/configuration.js";

const item = { id: "one", count: 2 };
const xmlBody = "<item><count>2</count><id>one</id></item>";
const baseURL = "https://execution.test";

function setup(fetch: typeof globalThis.fetch, extra: Partial<ClientOptions> = {}) {
  const options = { baseURL, fetch, ...extra };
  return { context: createRequestContext(options), full: createClient(options) };
}

describe("real emitted media execution providers", () => {
  it("does not select XML solely because a JSON schema has XML metadata", async () => {
    const bodies: unknown[] = [];
    const { context, full } = setup(async (_url, init) => {
      bodies.push(JSON.parse(String(init?.body)));
      return Response.json(item);
    });
    expect(json.profile).toBe("json");
    expect(await json.bind(context)({ body: item })).toEqual(item);
    expect(await full.$operations.jsonEcho({ body: item })).toEqual(item);
    expect(bodies).toEqual([item, item]);
  });

  it("matches full-client XML body encoding and response decoding", async () => {
    const bodies: unknown[] = [];
    const { context, full } = setup(async (_url, init) => {
      bodies.push(init?.body);
      return new Response(xmlBody, { headers: { "content-type": "application/xml" } });
    });
    expect(xml.profile).toBe("buffered-xml");
    expect(await xml.bind(context)({ body: item })).toEqual(item);
    expect(await full.$operations.xmlEcho({ body: item })).toEqual(item);
    expect(bodies).toEqual([xmlBody, xmlBody]);
  });

  it("includes XML parameter and response-header implementations", async () => {
    const urls: string[] = [];
    const { context, full } = setup(async (url) => {
      urls.push(String(url));
      return Response.json(item, { headers: { "X-Item": xmlBody } });
    });
    expect(parameter.profile).toBe("buffered-xml");
    expect(header.profile).toBe("buffered-xml");
    expect(await parameter.bind(context)({ query: { item } })).toEqual(item);
    expect(await full.$operations.xmlParameter({ query: { item } })).toEqual(item);
    expect(urls[0]).toBe(urls[1]);
    expect(new URL(urls[0]!).searchParams.get("item")).toBe(xmlBody);
    expect((await header.bind(context).raw()).headers["X-Item"]).toEqual(item);
    expect((await full.$operations.xmlHeader.raw()).headers["X-Item"]).toEqual(item);
  });

  it("retains working form and multipart encoding through the general profile", async () => {
    const bodies: unknown[] = [];
    const { context, full } = setup(async (_url, init) => {
      const body = init?.body;
      bodies.push(body instanceof FormData ? Object.fromEntries(body.entries()) : String(body));
      return Response.json(item);
    });
    expect(form.profile).toBe("general");
    expect(multipart.profile).toBe("general");
    expect(await form.bind(context)({ body: item })).toEqual(item);
    expect(await full.$operations.formEcho({ body: item })).toEqual(item);
    expect(await multipart.bind(context)({ body: item })).toEqual(item);
    expect(await full.$operations.multipartEcho({ body: item })).toEqual(item);
    expect(bodies[0]).toBe(bodies[1]);
    expect(Object.fromEntries(new URLSearchParams(String(bodies[0])))).toEqual({
      id: "one",
      count: "2",
    });
    expect(bodies[2]).toEqual({ id: "one", count: "2" });
    expect(bodies[3]).toEqual(bodies[2]);
  });

  it("retains text bodies and validates before sending", async () => {
    const bodies: unknown[] = [];
    const { context, full } = setup(async (_url, init) => {
      bodies.push(init?.body);
      return new Response(String(init?.body), { headers: { "content-type": "text/plain" } });
    });
    expect(text.profile).toBe("general");
    expect(await text.bind(context)({ body: "hello" })).toBe("hello");
    expect(await full.$operations.textEcho({ body: "hello" })).toBe("hello");
    expect(bodies).toEqual(["hello", "hello"]);
    await expect(xml.bind(context)({ body: { id: "one", count: -1 } })).rejects.toMatchObject({
      code: "REQUEST_ENCODE_FAILED",
    });
    expect(bodies).toHaveLength(2);
  });

  it("preserves media-root content annotation handling from the full client", async () => {
    const body = "<item><id>one</id><count>-1</count></item>";
    const { context, full } = setup(async () => Response.json(body));
    // The existing lowerer marks the media-root schema ignoreContentMediaType.
    // This case is distinct from content carried by a nested JSON property.
    expect(await embedded.bind(context)()).toBe(body);
    expect(await full.$operations.embeddedXML()).toBe(body);
  });

  it("validates nested XML content without changing the JSON string result", async () => {
    let body = { payload: xmlBody };
    const { context, full } = setup(async () => Response.json(body));
    expect(nested.profile).toBe("buffered-xml");
    expect(await nested.bind(context)()).toEqual(body);
    expect(await full.$operations.nestedXML()).toEqual(body);
    body = { payload: "<item><id>one</id><count>-1</count></item>" };
    await expect(nested.bind(context)()).rejects.toMatchObject({ code: "RESPONSE_DECODE_FAILED" });
    await expect(full.$operations.nestedXML()).rejects.toMatchObject({
      code: "RESPONSE_DECODE_FAILED",
    });
  });

  it("keeps XML error bodies instead of planning from the success response alone", async () => {
    const { context, full } = setup(
      async () =>
        new Response(xmlBody, { status: 400, headers: { "content-type": "application/xml" } }),
    );
    expect(error.profile).toBe("buffered-xml");
    await expect(error.bind(context)()).rejects.toMatchObject({ status: 400, data: item });
    await expect(full.$operations.xmlError()).rejects.toMatchObject({ status: 400, data: item });
  });

  it("retains parameter and response codec callbacks without the general registry", async () => {
    const urls: string[] = [];
    let decoded = 0;
    const { context, full } = setup(
      async (url) => {
        urls.push(String(url));
        return new Response("custom-result", {
          headers: { "content-type": "application/x-record" },
        });
      },
      {
        codecs: {
          "application/x-number": { encodeParameter: async (value) => `n:${String(value)}` },
          "application/x-record": {
            decode: async (response) => {
              expect(await response.text()).toBe("custom-result");
              decoded++;
              return item;
            },
          },
        },
      },
    );
    expect(custom.profile).toBe("json");
    expect(await custom.bind(context)({ query: { number: 7 } })).toEqual(item);
    expect(await full.$operations.customCodec({ query: { number: 7 } })).toEqual(item);
    expect(urls[0]).toBe(urls[1]);
    expect(new URL(urls[0]!).searchParams.get("number")).toBe("n:7");
    expect(decoded).toBe(2);
  });

  it("binds an ID-less operation by its exact route", async () => {
    const { context, full } = setup(async () => Response.json(item));
    expect(idless.route).toBe("GET /plain");
    expect("operationID" in idless).toBe(false);
    expect(await idless.bind(context)()).toEqual(await full.$routes["GET /plain"]());
  });
});
