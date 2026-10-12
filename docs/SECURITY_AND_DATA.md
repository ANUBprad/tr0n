# SECURITY_AND_DATA.md

> **SUPERSEDED** by [`SECURITY.md`](SECURITY.md). Kept for history; not canonical.
> See [`README.md`](README.md) for the authoritative document set.

**Status**: PRE-HACKATHON DESIGN — NOT IMPLEMENTATION

## Data Policy (Hard Constraint)
- **Only** public, synthetic, or data we are explicitly allowed to use
- **Never** use private, login-protected, or restricted company information
- **Synthetic-first**: Generate fictional company data (Acme Org)
- **No PII harvesting**: No scraping of real orgs without permission
- **Deterministic**: Synthetic data reproducible

## Security Principles
- No secrets/keys in repo (use .env.example post-implementation)
- Input validation at boundaries (post-Oct 15)
- Least privilege for any external calls
- Local-first (minimize external surface)
- Avoid logging sensitive data
- Read-only graph queries where possible for demo

## Provenance & Trust
- Evidence paths mandatory (explainable)
- No unverifiable claims
- Agent must cite supporting paths

## Disclosure
- AI coding assistants will be used (disclose per hackathon rules)
- Team understands and can explain all submitted code

## Ethics
- Synthetic org only; no impersonation of real entities
- Avoid generating realistic sensitive scenarios beyond what's needed

No implementation security code added in foundation phase.

