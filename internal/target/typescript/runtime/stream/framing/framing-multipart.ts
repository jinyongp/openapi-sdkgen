export type MultipartFramingFailure = "opening" | "boundary" | "limit" | "incomplete";
type MultipartFramingState = "opening" | "part" | "suffix" | "closed";

/** Stateful KMP matcher visits each new byte once, including split boundaries. */
class BoundaryMatcher {
  private matched: number = 0;
  private readonly prefixes: number[];

  public constructor(private readonly marker: Uint8Array) {
    this.prefixes = new Array<number>(marker.length).fill(0);
    for (let index: number = 1, prefix: number = 0; index < marker.length; index++) {
      while (prefix > 0 && marker[index] !== marker[prefix]) prefix = this.prefixes[prefix - 1]!;
      if (marker[index] === marker[prefix]) prefix++;
      this.prefixes[index] = prefix;
    }
  }

  public feed(byte: number): boolean {
    while (this.matched > 0 && byte !== this.marker[this.matched])
      this.matched = this.prefixes[this.matched - 1]!;
    if (byte === this.marker[this.matched]) this.matched++;
    return this.matched === this.marker.length;
  }

  public reset(): void {
    this.matched = 0;
  }
}

/** Retains chunk views and copies only a completed part. */
export class MultipartByteFrames {
  private state: MultipartFramingState = "opening";
  private readonly opening: BoundaryMatcher;
  private readonly separator: BoundaryMatcher;
  private readonly openingLength: number;
  private readonly separatorLength: number;
  private fragments: Uint8Array[] = [];
  private bytes: number = 0;
  private suffix: number[] = [];
  private frame: Uint8Array<ArrayBuffer> | undefined;
  private openingSuffix: boolean = false;

  public constructor(
    boundary: string,
    private readonly maximum: number | undefined,
    private readonly fail: (reason: MultipartFramingFailure) => never,
  ) {
    const encoder: TextEncoder = new TextEncoder();
    const opening: Uint8Array<ArrayBuffer> = encoder.encode("--" + boundary);
    const separator: Uint8Array<ArrayBuffer> = encoder.encode("\r\n--" + boundary);
    this.opening = new BoundaryMatcher(opening);
    this.separator = new BoundaryMatcher(separator);
    this.openingLength = opening.length;
    this.separatorLength = separator.length;
  }

  public get closed(): boolean {
    return this.state === "closed";
  }

  public *push(chunk: Uint8Array): Iterable<Uint8Array<ArrayBuffer>> {
    let start: number = this.state === "part" ? 0 : -1;
    for (let index: number = 0; index < chunk.length && !this.closed; index++) {
      const byte: number = chunk[index]!;
      if (this.state === "suffix") {
        this.suffix.push(byte);
        if (this.suffix.length < 2) continue;
        const closing: boolean = this.suffix[0] === 45 && this.suffix[1] === 45;
        if (!closing && (this.suffix[0] !== 13 || this.suffix[1] !== 10))
          this.fail(this.openingSuffix ? "opening" : "boundary");
        const frame: Uint8Array<ArrayBuffer> | undefined = this.frame;
        this.frame = undefined;
        this.state = closing ? "closed" : "part";
        this.bytes = 0;
        this.suffix = [];
        this.separator.reset();
        start = closing ? -1 : index + 1;
        if (frame !== undefined) yield frame;
        continue;
      }
      this.bytes++;
      if (this.state === "opening") {
        if (this.opening.feed(byte)) {
          this.state = "suffix";
          this.openingSuffix = true;
          continue;
        }
        if (this.maximum !== undefined && this.bytes > 8192 + this.openingLength + 2)
          this.fail("limit");
      } else {
        if (this.separator.feed(byte)) {
          this.fragments.push(chunk.subarray(start, index + 1));
          this.frame = this.complete(this.bytes - this.separatorLength);
          this.state = "suffix";
          this.openingSuffix = false;
          start = -1;
          continue;
        }
        if (
          this.maximum !== undefined &&
          this.bytes > this.maximum + 8192 + this.separatorLength + 4
        )
          this.fail("limit");
      }
    }
    if (this.state === "part" && start >= 0 && start < chunk.length)
      this.fragments.push(chunk.subarray(start));
  }

  public finish(): void {
    if (!this.closed) this.fail("incomplete");
  }

  private complete(length: number): Uint8Array<ArrayBuffer> {
    const result: Uint8Array<ArrayBuffer> = new Uint8Array(length);
    let offset: number = 0;
    for (const fragment of this.fragments) {
      const size: number = Math.min(fragment.length, length - offset);
      if (size <= 0) break;
      result.set(fragment.subarray(0, size), offset);
      offset += size;
    }
    this.fragments = [];
    return result;
  }
}
