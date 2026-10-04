/** Decoding failure whose response is safe for the generated router to return. */
export class InboundRequestError extends Error {
  readonly response: Response;
  constructor(response: Response) {
    super("Inbound request could not be decoded");
    this.response = response;
  }
}
