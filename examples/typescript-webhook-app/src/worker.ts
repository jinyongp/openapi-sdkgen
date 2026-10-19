import {
  createWebhookRouter,
  type WebhookHandlers,
  type WebhookRouter,
  type Webhooks,
} from "./generated/api/server/webhooks.js";

type CompletionResponse = Webhooks["todoCompleted"]["POST"]["response"];

const handlers: WebhookHandlers = {
  todoCompleted: {
    POST: async ({ body }): Promise<CompletionResponse> => {
      console.log(body.todoID, body.completed);
      return { status: 204 };
    },
  },
};

export function createReceiver(expectedToken: string): WebhookRouter {
  return createWebhookRouter(handlers, {
    routes: {
      todoCompleted: "/hooks/todos/completed",
    },
    authenticate: ({ request }): Response | undefined =>
      request.headers.get("x-webhook-token") === expectedToken
        ? undefined
        : new Response("Unauthorized", { status: 401 }),
  });
}
