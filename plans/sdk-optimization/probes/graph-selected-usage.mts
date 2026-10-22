import type { Client } from "__SDK_ROOT__";

export function selectedUsage(api: Client): void {
  void api.$routes["GET /users"]({ query: { $top: 1 } });
  void api.$operations["users.user.GetUser"]({ path: { "user-id": "u" } });
  void api.$operations["users.user.UpdateUser"]({
    path: { "user-id": "u" },
    body: { displayName: "Grace" },
  });
  void api.$routes["POST /users/{user-id}/sendMail"]({
    path: { "user-id": "u" },
    body: {
      message: {
        subject: "Hello",
        body: { contentType: "html", content: "<p>Hello</p>" },
        toRecipients: [{ emailAddress: { address: "reader@example.test" } }],
      },
      saveToSentItems: false,
    },
  });
  void api.$operations["groups.ListMembers"]({ path: { "group-id": "g" }, query: { $top: 1 } });

  // @ts-expect-error An existing Graph route outside this selection is unavailable.
  void api.$routes["GET /organization"];
  // @ts-expect-error An existing Graph operation ID outside this selection is unavailable.
  void api.$operations["users.user.DeleteUser"];
  // @ts-expect-error The selected path parameter requires a string.
  void api.$routes["GET /users/{user-id}"]({ path: { "user-id": 42 } });
}
