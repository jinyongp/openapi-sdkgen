import type { Client } from "__SDK_ROOT__";

export function selectedUsage(api: Client): void {
  void api.$routes["GET /users/{user-id}/authentication/requirements"]({
    path: { "user-id": "u" }, query: { $select: ["perUserMfaState"] },
  });
  void api.$operations["users.authentication.UpdateRequirements"]({
    path: { "user-id": "u" },
    body: { "@odata.type": "#microsoft.graph.strongAuthenticationRequirements", perUserMfaState: "enabled" },
  });
  void api.$operations["users.user.checkMemberGroups"]({
    path: { "user-id": "u" }, body: { groupIds: ["11111111-1111-4111-8111-111111111111"] },
  });
  void api.$routes["POST /users/{user-id}/getMailTips"]({
    path: { "user-id": "u" },
    body: { EmailAddresses: ["reader@example.test"], MailTipsOptions: "automaticReplies" },
  });
  void api.$operations["users.user.revokeSignInSessions"]({ path: { "user-id": "u" } });

  // @ts-expect-error An existing original route outside this selection is unavailable.
  void api.$routes["GET /users"];
  // @ts-expect-error An existing original operation outside this selection is unavailable.
  void api.$operations["users.user.GetUser"];
  // @ts-expect-error Selected path parameters require strings.
  void api.$routes["GET /users/{user-id}/authentication/requirements"]({ path: { "user-id": 42 } });
}
