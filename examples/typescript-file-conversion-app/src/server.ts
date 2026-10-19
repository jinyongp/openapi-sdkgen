import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";

interface Contact {
  readonly name: string;
  readonly email: string;
}

function problem(
  response: ServerResponse,
  status: number,
  message: string,
): void {
  response.writeHead(status, { "content-type": "application/json" });
  response.end(JSON.stringify({ message }));
}
async function convert(
  request: IncomingMessage,
  response: ServerResponse,
): Promise<void> {
  if (request.method !== "POST" || request.url !== "/convert/contacts") {
    response.writeHead(404).end();
    return;
  }
  const chunks: Buffer[] = [];
  let size: number = 0;
  for await (const chunk of request) {
    const bytes: Buffer = Buffer.from(chunk);
    size += bytes.length;
    if (size > 64 * 1024) {
      problem(response, 413, "Upload exceeds 64 KiB");
      return;
    }
    chunks.push(bytes);
  }
  let input: unknown;
  try {
    input = JSON.parse(Buffer.concat(chunks).toString("utf8"));
  } catch {
    problem(response, 400, "Expected a JSON array of contacts");
    return;
  }
  if (
    !Array.isArray(input) ||
    !input.every(
      (value: unknown): value is Contact =>
        typeof value === "object" &&
        value !== null &&
        "name" in value &&
        typeof value.name === "string" &&
        value.name.trim().length > 0 &&
        "email" in value &&
        typeof value.email === "string" &&
        value.email.trim().length > 0,
    )
  ) {
    problem(response, 400, "Each contact needs a name and email");
    return;
  }
  const contacts: readonly Contact[] = input;
  const ndjson: string = contacts
    .map((contact: Contact): string =>
      JSON.stringify({ name: contact.name, email: contact.email }),
    )
    .join("\n");
  response.writeHead(200, {
    "content-type": "application/octet-stream",
    "content-disposition": 'attachment; filename="contacts.ndjson"',
  });
  response.end(contacts.length ? ndjson + "\n" : "");
}
const server: ReturnType<typeof createServer> = createServer(
  (request: IncomingMessage, response: ServerResponse): void => {
    void convert(request, response).catch((): void => {
      response.destroy();
    });
  },
);
server.listen(
  Number(process.env.CONVERSION_API_PORT ?? 18797),
  "127.0.0.1",
  (): void => console.log("Conversion API listening"),
);
