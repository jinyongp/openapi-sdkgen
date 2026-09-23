# Security Policy

## Supported versions

Security fixes are provided only for the latest stable release of
`openapi-sdkgen`.

| Release | Security support |
| --- | --- |
| Latest stable release | Supported |
| Older stable releases | Not supported |
| Prereleases | Not supported |

As of September 23, 2026, the latest stable release is `v8.0.0`. When a newer
stable release is published, it becomes the supported release and the previous
stable release leaves security support.

Users should reproduce suspected security issues against the latest stable
release when possible.

## What to report

Security reports are appropriate for vulnerabilities in:

- the `openapi-sdkgen` CLI, compiler, parsers, input acquisition, remote
  reference handling, or output publication;
- credential, transport, trust-boundary, or filesystem handling owned by the
  generator;
- the npm launcher, downloaded release binaries, checksum verification, or
  release/distribution integrity;
- generated runtime behavior when the vulnerable behavior is reproducibly
  emitted by the supported generator without depending on application-specific
  code.

Application code, deployment configuration, third-party services, and
vulnerabilities that exist only in user-owned customizations are normally
outside this repository's scope. A third-party dependency issue is relevant
when it creates an exploitable condition in the supported
`openapi-sdkgen` release.

## Reporting a vulnerability

Do not disclose an unpatched vulnerability, exploit, credential, sensitive
log, or proof of concept in a public GitHub issue, pull request, or discussion.

If the repository's **Security** tab shows **Report a vulnerability**, use that
GitHub Private Vulnerability Reporting form. It keeps the report and discussion
private between the reporter and repository maintainers.

If that private reporting control is not available, open a public issue that
contains only a request for a private security contact channel. Do not include
the vulnerability description, affected code, reproduction steps, exploit
details, credentials, or other sensitive information in that issue.

A useful private report should include, when available:

- the affected `openapi-sdkgen` version or commit;
- the affected component and source paths;
- the security impact and realistic attack conditions;
- minimal reproduction steps or a sanitized proof of concept;
- required OpenAPI input, platform, runtime, or configuration details;
- known mitigations or workarounds;
- whether the issue has already been disclosed elsewhere.

Do not send live API keys, passwords, private keys, customer data, or
unredacted sensitive logs.

## Coordinated disclosure

Please allow time for the issue to be reproduced, fixed, and released before
publishing vulnerability details.

When a report is confirmed, maintainers may use a private GitHub Security
Advisory to coordinate remediation and disclosure. Public disclosure should
identify the affected supported release, the fixed release, and any necessary
upgrade or mitigation guidance.

This project does not promise a response or remediation SLA, and it does not
operate a vulnerability bounty program.
