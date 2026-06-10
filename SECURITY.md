# Security Policy

FinWatch is an educational reference implementation that uses **synthetic data
only**. It is not a certified security, fraud, AML, sanctions, or compliance
product. Even so, we take the security of the codebase seriously.

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, report privately using one of:

- GitHub's **private vulnerability reporting** ("Report a vulnerability" under
  the repository's *Security* tab), or
- a direct private message to the maintainer (`@vianbas`).

Please include:

- a description of the issue and its impact,
- steps to reproduce or a proof of concept,
- affected components/versions, and
- any suggested remediation.

We aim to acknowledge reports within a few business days and will keep you
updated on remediation progress. Please allow reasonable time to address the
issue before any public disclosure.

## Scope

In scope: code in this repository (backend, frontend, contracts, CI, Docker).

Out of scope: third-party dependencies (report upstream), and any deployment a
third party operates from this code.

## Handling expectations

- No real personal or financial data should ever appear in reports — use
  synthetic examples.
- Do not include secrets or credentials in reports.
- See [`docs/threat-model.md`](docs/threat-model.md) and
  [`docs/security/data-classification.md`](docs/security/data-classification.md).
