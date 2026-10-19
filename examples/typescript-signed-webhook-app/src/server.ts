import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import {
  createWebhookRouter,
  type WebhookHandlers,
  type Webhooks,
} from "./generated/api/server/webhooks.js";
import { verify } from "./signature.js";

type PaidResponse = Webhooks["orderPaid"]["POST"]["response"];
const configuredSecret: string | undefined = process.env.WEBHOOK_SECRET;
if (!configuredSecret)
  throw new Error("Set WEBHOOK_SECRET on the receiver and sender.");
const secret: string = configuredSecret;
const deliveries: Map<string, string> = new Map();
const orders: string[] = [];
const handlers: WebhookHandlers = {
  orderPaid: {
    POST: ({ body, params }): PaidResponse => {
      const id: string = params.headerParams["x-webhook-id"];
      const fingerprint: string = JSON.stringify([
        body.orderID,
        body.amountCents,
      ]);
      const previous: string | undefined = deliveries.get(id);
      if (previous !== undefined)
        return { status: previous === fingerprint ? 204 : 409 };
      deliveries.set(id, fingerprint);
      orders.push(body.orderID);
      return { status: 204 };
    },
  },
};
const router: ReturnType<typeof createWebhookRouter> = createWebhookRouter(
  handlers,
  { routes: { orderPaid: "/hooks/orders/paid" } },
);

async function handle(
  request: IncomingMessage,
  response: ServerResponse,
): Promise<void> {
  if (request.method === "GET" && request.url === "/fulfillments") {
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({ processed: orders.length, orders }));
    return;
  }
  if (request.method !== "POST" || request.url !== "/hooks/orders/paid") {
    response.writeHead(404).end();
    return;
  }
  const chunks: Buffer[] = [];
  let size: number = 0;
  for await (const chunk of request) {
    const data: Buffer = Buffer.from(chunk);
    size += data.length;
    if (size > 64 * 1024) {
      response.writeHead(413).end();
      return;
    }
    chunks.push(data);
  }
  const raw: Buffer = Buffer.concat(chunks);
  const headers: Headers = new Headers();
  for (const [name, value] of Object.entries(request.headers)) {
    if (typeof value === "string") headers.set(name, value);
    else if (value) for (const item of value) headers.append(name, item);
  }
  if (
    !verify(
      secret,
      headers.get("x-webhook-id") ?? "",
      headers.get("x-webhook-timestamp") ?? "",
      headers.get("x-webhook-signature") ?? "",
      raw,
    )
  ) {
    response.writeHead(401).end();
    return;
  }
  // Verify the original bytes before the generated receiver decodes JSON.
  const result: Response = await router.fetch(
    new Request("http://localhost/hooks/orders/paid", {
      method: "POST",
      headers,
      body: new Uint8Array(raw),
    }),
  );
  response.writeHead(result.status, Object.fromEntries(result.headers));
  response.end(new Uint8Array(await result.arrayBuffer()));
}
const server: ReturnType<typeof createServer> = createServer(
  (request: IncomingMessage, response: ServerResponse): void => {
    void handle(request, response).catch((): void => {
      response.destroy();
    });
  },
);
server.listen(
  Number(process.env.WEBHOOK_API_PORT ?? 18799),
  "127.0.0.1",
  (): void => console.log("Signed webhook API listening"),
);
