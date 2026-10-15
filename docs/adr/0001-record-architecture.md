# ADR 0001: Record architecture decisions

- Status: accepted
- Date: 2026-10-14

## Context

The architecture was designed up front and its key choices are listed in the decision ledger in
[`ARCHITECTURE.md` §2](../ARCHITECTURE.md#2-decision-ledger-locked) (D1–D17). Those decisions are
locked for v1: implementation must follow them, and changing one needs the owner's approval. Later
decisions, and any approved change to a ledger entry, need a place where their context and
trade-offs are kept.

## Decision

- The ledger in `ARCHITECTURE.md` §2 is the authoritative record of D1–D17. It is not duplicated
  into separate ADRs.
- A change to a ledger entry, or a new decision of the same weight, gets a numbered ADR in
  `docs/adr/` (`NNNN-short-title.md`: status, date, context, decision, consequences). The ledger row
  is updated in the same change and links to the ADR.
- Smaller implementation choices (library versions, naming) are listed under "Decisions" in
  `docs/STATUS.md`.

## Consequences

- Anyone reading the ledger sees the current decision; the ADR explains why it changed.
- Superseded ADRs stay in place with status `superseded by NNNN`.
