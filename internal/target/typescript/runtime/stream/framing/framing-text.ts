/** Pure incremental framing; adapters retain parsing, errors and reader ownership. */
export class DelimitedTextFrames {
  private readonly decoder: TextDecoder = new TextDecoder();
  private readonly encoder: TextEncoder = new TextEncoder();
  private fragments: string[] = [];
  private bytes: number = 0;

  public constructor(
    private readonly separator: "\n" | "\u001e",
    private readonly maximum: number,
    private readonly oversized: () => never,
  ) {}

  public *push(chunk: Uint8Array | undefined, done: boolean): Iterable<string> {
    const text: string = this.decoder.decode(chunk, { stream: !done });
    let start: number = 0;
    let end: number;
    while ((end = text.indexOf(this.separator, start)) >= 0) {
      this.append(text.slice(start, end));
      yield this.complete();
      start = end + 1;
    }
    this.append(text.slice(start));
    if (done && this.fragments.length > 0) yield this.complete();
  }

  private append(fragment: string): void {
    if (fragment.length === 0) return;
    // Only the new fragment is encoded. Pending frame contents are never
    // searched, joined or re-encoded until the frame completes.
    this.bytes += this.encoder.encode(fragment).byteLength;
    if (this.bytes > this.maximum) this.oversized();
    this.fragments.push(fragment);
  }

  private complete(): string {
    const frame: string = this.fragments.join("");
    this.fragments = [];
    this.bytes = 0;
    return frame;
  }
}
