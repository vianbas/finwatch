# Data classification

FinWatch is a reference implementation that uses **synthetic data only**. This
document defines the data classes the project recognises and how each must be
handled.

## Classes

| Class | Definition | Allowed in repo? | Handling |
| ----- | ---------- | ---------------- | -------- |
| **Public** | Source code, docs, contracts, example config. | Yes | Standard open-source handling. |
| **Synthetic operational** | Fabricated transactions, customers, alerts used for demos/tests. | Yes | Must be clearly synthetic; never derived from real records. |
| **Example credentials** | Non-secret dev credentials for local use. | Yes, labelled | Only in `.env.example` / `docker-compose.yml`; never reused from real systems. |
| **Secret** | Real passwords, tokens, keys, certificates. | **No** | Never committed. Inject at runtime via environment/secret stores. |
| **Real personal / financial data** | Any real customer, account, or transaction data. | **No** | Prohibited anywhere in the project, including tests, fixtures, screenshots, and docs. |
| **Proprietary** | Any specific organisation's names, logos, internal schemas, thresholds, or implementation details. | **No** | Prohibited; keep everything generic and public. |

## Rules

1. **No real data, ever.** All transaction and customer data is synthetic.
2. **No secrets in version control.** `.env` is git-ignored; CI holds no
   deployment secrets and uses least-privilege tokens.
3. **No proprietary references.** See [`.claude/rules/security.md`](../../.claude/rules/security.md).
4. **Logs and errors** must never contain secrets or real personal/financial
   data. Synthetic identifiers only.
5. **Generating synthetic data** must not reverse-engineer or embed real
   distributions, thresholds, or schemas from any real institution.

## Responsibilities

- Contributors ensure changes comply before opening a PR.
- Reviewers check diffs for secrets, real data, and proprietary references.
- Vulnerabilities or accidental disclosures are reported privately per
  [`SECURITY.md`](../../SECURITY.md).
