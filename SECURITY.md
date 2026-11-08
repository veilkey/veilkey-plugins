# Security Policy

This repository is part of [VeilKey](https://github.com/veilkey/veilkey-selfhosted), a self-hosted secret manager that keeps secrets away from AI coding tools. It follows the same security policy as the main VeilKey repository.

## Supported Versions

This repository has no tagged releases. Only the latest commit on `main` receives security fixes.

## Reporting a Vulnerability

Please do **not** open a public issue or pull request for a security problem.

Report it privately in one of these ways:

1. **GitHub private vulnerability reporting** (preferred): open the **Security** tab of this repository and choose **Report a vulnerability**.
2. **Email**: team@dalsoop.com

Please include the affected commit, the steps to reproduce, and the impact you observed. If the issue also affects [veilkey-selfhosted](https://github.com/veilkey/veilkey-selfhosted), you may report it there instead.

## Response Process

- We acknowledge every report within **7 days**.
- We aim to fix and publicly disclose the issue within **90 days** of the report, coordinating the disclosure date with you.
- With your permission we credit you in the published advisory. If you prefer not to be named, we will not name you.

## Scope

These plugins run inside VeilKey services, so the scope follows the VeilKey [security model](https://github.com/veilkey/veilkey-selfhosted/blob/main/docs/security-model.md). Threats that the security model lists under "What VeilKey does NOT protect against" are out of scope.

## Maintainer

- [dalsoop](https://github.com/dalsoop)
