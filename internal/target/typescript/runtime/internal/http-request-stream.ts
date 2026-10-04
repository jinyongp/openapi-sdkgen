import type {
  WireCodec,
  WireSchema,
  WireSchemas,
  StreamCodec,
  StreamContext,
  StreamFraming,
} from "./wire-types.js";
import type {
  IncrementalStreamRequestOptions,
  CompleteSequentialRequestOptions,
  EncodedStreamRequestBody,
  StreamProtocolEncodeOptions,
} from "./http-types.js";
import type {
  TextRequestEncoder,
  MultipartRequestEncoder,
  RequestStreamServices,
} from "./http-media-types.js";
import { isPromise } from "./http-execution-support.js";
export function createRequestStreamServices(
  wire: WireCodec,
  encoders: Readonly<
    Partial<Record<Exclude<StreamFraming, "multipart" | "custom">, TextRequestEncoder>>
  >,
  multipart: MultipartRequestEncoder | undefined,
): RequestStreamServices {
  const { transformWireValue }: WireCodec = wire;
  function encodeIncrementalStreamRequestBody(
    values: AsyncIterable<unknown>,
    options: IncrementalStreamRequestOptions,
  ): EncodedStreamRequestBody | Promise<EncodedStreamRequestBody> {
    const context: StreamContext = {
      contentType: options.contentType,
      maxFrameBytes: options.maxFrameBytes,
      ...(options.signal === undefined ? {} : { signal: options.signal }),
    };
    const items: AsyncIterable<unknown> = transformStreamingRequestItems(
      values,
      options.itemSchema,
      options.schemas,
    );
    const frames: AsyncIterable<unknown> = encodeStreamApplicationFrames(
      items,
      options.streamCodec,
      context,
    );
    return encodeStreamProtocolFrames(frames, {
      contentType: options.contentType,
      streamFraming: options.streamFraming,
      streamCodec: options.streamCodec,
      context,
      frameSchema: options.itemSchema,
      prefixSchemas: undefined,
      schemas: options.schemas,
      prefixEncoding: undefined,
      itemEncoding: options.itemEncoding,
      suppliedHeaders: options.suppliedHeaders,
      suppliedContentTypes: options.suppliedContentTypes,
      codecs: options.codecs,
    });
  }

  function encodeCompleteSequentialRequestBody(
    value: unknown,
    options: CompleteSequentialRequestOptions,
  ): EncodedStreamRequestBody | Promise<EncodedStreamRequestBody> {
    const transformed: unknown = transformWireValue(
      value,
      options.schema,
      options.schemas,
      "encode",
    );
    if (!Array.isArray(transformed))
      throw new TypeError("complete sequential request body must be an array value");
    const context: StreamContext = {
      contentType: options.contentType,
      maxFrameBytes: options.maxFrameBytes,
      ...(options.signal === undefined ? {} : { signal: options.signal }),
    };
    const items: AsyncIterable<unknown> = streamArrayValues(transformed);
    const frames: AsyncIterable<unknown> = encodeStreamApplicationFrames(
      items,
      options.streamCodec,
      context,
    );
    return encodeStreamProtocolFrames(frames, {
      contentType: options.contentType,
      streamFraming: options.streamFraming,
      streamCodec: options.streamCodec,
      context,
      frameSchema: options.schema.items ?? {},
      prefixSchemas: options.schema.prefixItems,
      schemas: options.schemas,
      prefixEncoding: options.prefixEncoding,
      itemEncoding: options.itemEncoding,
      suppliedHeaders: options.suppliedHeaders,
      suppliedContentTypes: options.suppliedContentTypes,
      codecs: options.codecs,
    });
  }

  function encodeStreamApplicationFrames(
    items: AsyncIterable<unknown>,
    streamCodec: StreamCodec | undefined,
    context: StreamContext,
  ): AsyncIterable<unknown> {
    if (streamCodec?.adapter !== undefined) return streamCodec.adapter.encode(items, context);
    return items;
  }

  function encodeStreamProtocolFrames(
    frames: AsyncIterable<unknown>,
    options: StreamProtocolEncodeOptions,
  ): EncodedStreamRequestBody | Promise<EncodedStreamRequestBody> {
    if (options.streamCodec?.protocol !== undefined) {
      const encoded:
        | ReadableStream<Uint8Array<ArrayBufferLike>>
        | Promise<ReadableStream<Uint8Array<ArrayBufferLike>>> =
        options.streamCodec.protocol.encode(frames, options.context);
      const finish: (body: ReadableStream<Uint8Array>) => EncodedStreamRequestBody = (
        body: ReadableStream<Uint8Array>,
      ): EncodedStreamRequestBody => ({
        body,
        contentType: options.contentType,
      });
      return isPromise(encoded) ? encoded.then(finish) : finish(encoded);
    }
    if (options.streamFraming === "multipart" && multipart !== undefined)
      return multipart(frames, options);
    const encoder: TextRequestEncoder | undefined =
      options.streamFraming === undefined ||
      options.streamFraming === "multipart" ||
      options.streamFraming === "custom"
        ? undefined
        : encoders[options.streamFraming];
    if (encoder !== undefined) return encoder(frames, options);
    throw new TypeError(`missing stream protocol for ${options.contentType}`);
  }

  async function* streamArrayValues(values: readonly unknown[]): AsyncIterable<unknown> {
    for (const value of values) yield value;
  }

  function transformStreamingRequestItems(
    values: AsyncIterable<unknown>,
    itemSchema: WireSchema,
    schemas: WireSchemas,
  ): AsyncIterable<unknown> {
    return mapAsyncIterable(values, (value: unknown): unknown =>
      transformWireValue(value, itemSchema, schemas, "encode"),
    );
  }

  function mapAsyncIterable<Input, Output>(
    values: AsyncIterable<Input>,
    transform: (value: Input) => Output,
  ): AsyncIterable<Output> {
    return {
      [Symbol.asyncIterator](): AsyncIterator<Output> {
        const iterator: AsyncIterator<Input, unknown, unknown> = values[Symbol.asyncIterator]();
        let done: boolean = false;
        return {
          async next(): Promise<IteratorResult<Output>> {
            if (done) return { done: true, value: undefined as never };
            const next: IteratorResult<Input, unknown> = await iterator.next();
            if (done || next.done === true) {
              done = true;
              return { done: true, value: undefined as never };
            }
            return { done: false, value: transform(next.value) };
          },
          async return(reason?: unknown): Promise<IteratorResult<Output>> {
            if (done) return { done: true, value: undefined as never };
            done = true;
            await iterator.return?.(reason);
            return { done: true, value: undefined as never };
          },
        };
      },
    };
  }

  return { encodeIncrementalStreamRequestBody, encodeCompleteSequentialRequestBody };
}
