# Business

Vibeporter is **free and open source** (MIT). The CLI has no account, no telemetry, and no payment step.

## What exists

A local context layer: source sessions, context packets, task handoff, adapters, and a loopback web UI.

## Possible commercial model

If a paid offering appears, it should sell **team context**, not a chat converter:

- Hosted shared project context
- Private / self-hosted deployment
- Support for teams running the open-source CLI

None of that is implemented in this repository.

## Dodo Payments

An account exists at Dodo Payments. **It is not integrated yet.** Do not add API keys, checkout, billing tables, paid plans, or webhook handlers until a paid team workflow is validated.

Revisit after that validation — not before.

## Questions to answer before any payment integration

1. **Who is the buyer?** Individual developer, team lead, or company?
2. **What are they paying for?** Hosted context, self-host license, support, or something else?
3. **What is the unit of payment?** Seat, project, packet volume, or instance?
4. **Is self-hosting required?** Many teams will not send engineering context to a third-party host.

Until those are clear, keep the product free, local, and unpaid.

The same page is published at [https://vibeporter.pages.dev/business](https://vibeporter.pages.dev/business).
