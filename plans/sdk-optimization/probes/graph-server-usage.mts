import { createWebhookRouter, type WebhookRouter } from "__SDK_WEBHOOKS__";
import { createCallbackHandlers, type CallbackEndpoints } from "__SDK_CALLBACKS__";

export const router: WebhookRouter = createWebhookRouter({}, { routes: {} });
export const callbacks: CallbackEndpoints = createCallbackHandlers({});
export async function fetch(request: Request): Promise<Response> { return router.fetch(request); }
export function rejectInvalidServerUsage(): void {
  // @ts-expect-error the required routes option must not disappear
  createWebhookRouter({}, {});
  // @ts-expect-error Fetch endpoints require Request objects
  router.fetch("https://example.test/");
}
