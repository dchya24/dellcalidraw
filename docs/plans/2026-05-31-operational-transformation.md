# Operational Transformation for Concurrent Edits — Plan

> **Status:** PLAN ONLY. Not yet implemented.
> **Created:** 2026-05-31
> **Owners:** Backend + Frontend collab teams.

This is a design plan, not a green-light. OT is a multi-week project
that touches the WebSocket protocol, room state machine, and frontend
sync layer. Before any code lands, this plan needs review and a
deliberate decision on scope.

## Background

The application currently uses **last-write-wins** (LWW) conflict
resolution: each `update_elements` message replaces the server's
version of the affected elements, and the server rebroadcasts. The
frontend's `elementSyncService` debounces local changes to 100 ms and
sends deltas (added / updated / deleted).

LWW is correct as long as concurrent edits target *disjoint* elements.
It silently drops updates when two users edit the same element at the
same time. This is acceptable for most diagram work but degrades
visibly during simultaneous interactive sessions (e.g. one user
dragging a shape while another retypes its label).

## Goals

- **Convergence**: every client ends up with the same final state
  regardless of message arrival order.
- **Intent preservation**: each user's local edit appears applied,
  even when transformed against concurrent edits.
- **Causality**: an update built on a previous version is delivered
  to peers in an order consistent with that dependency.
- **Backward compatibility**: existing rooms and clients keep working
  while OT rolls out behind a feature flag.

## Non-goals (this iteration)

- Element-level locking (would block legitimate concurrent edits).
- CRDTs (Yjs/Automerge) — bigger overhaul of the wire protocol; worth
  considering separately if OT proves too gnarly.
- Conflict-free undo across users (multi-user undo is a known hard
  problem and out of scope here).
- Rich-text OT inside text labels (treat text as opaque string for now).

## Scope

OT will be applied to **element field updates** only:

| Field family            | Strategy                                          |
| ----------------------- | ------------------------------------------------- |
| `x`, `y`, `width`, `height`, `angle` | Numeric LWW with version counter      |
| `points` (lines/arrows) | Index-based OT with insert/delete primitives      |
| `text` on text elements | String OT (Jupiter-style, char-level operations)  |
| Style attrs (color, etc) | LWW per attribute                                |
| Lifecycle (`isDeleted`) | Tombstone wins                                    |

For each family the operation set must satisfy `transform(a, b)` such
that `apply(state, a) → apply(state, transform(b, a)) == apply(state, b) → apply(state, transform(a, b))` (TP1).

## Architecture

### Server: per-room operation log

```
                ┌──────────────────────────────────────────┐
client A ──► op_a ──► RoomOpLog [seq N+1]
                                                            ├──► broadcast op_a (transformed against ops > seq) to A,B
client B ──► op_b ──► RoomOpLog [seq N+2 after transform]
                ▲                                           │
                └─────────── ack(seq) ──────────────────────┘
```

- New table `room_ops` keyed by `(room_db_id, seq, user_id, kind, payload)`.
- `Hub.handleUpdateElements` becomes `handleOp`:
  1. Parse op + parent seq (last seq client saw).
  2. Transform against ops in the log with seq > parent seq.
  3. Append the transformed op to the log, return its seq.
  4. Broadcast `(seq, op)` to other room members.
- The existing throttled `BatchSaveElements` becomes a periodic
  *materialise* job that replays ops since the last snapshot into the
  `room_elements` table. The element table stays the source of truth
  for fresh room loads; the op log catches the live tail.

### Frontend: pending op buffer

- `elementSyncService` adds two pieces of state per tab:
  - `lastAckedSeq` — last seq number observed from the server
  - `pendingOps` — local operations not yet acked
- On local edit, generate an op with `parentSeq = lastAckedSeq`,
  apply it locally optimistically, push to `pendingOps`, send.
- On server ack with `(seq, transformedOp)`:
  - If `transformedOp` differs from the local op, replace the local
    application with the transformed version.
  - Drop matching entry from `pendingOps`.
- On remote op `(seq, op)`:
  - Transform op against every entry in `pendingOps`, apply to local
    scene, advance `lastAckedSeq`.

### Wire protocol additions

```jsonc
// new outbound message
{ "type": "op", "payload": { "parentSeq": N, "kind": "...", "ops": [...] } }

// new inbound (broadcast or self-ack)
{ "type": "op_applied", "payload": { "seq": N+1, "userId": "...", "ops": [...] } }
```

The legacy `update_elements` path stays alive for non-OT clients
during transition (feature flag `EXCALIDRAW_OT_ENABLED`).

## Database changes

Migration `0000XX_room_ops`:

```sql
CREATE TABLE room_ops (
  id          BIGSERIAL PRIMARY KEY,
  room_id     UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  seq         BIGINT NOT NULL,
  user_id     VARCHAR(255) NOT NULL,
  kind        VARCHAR(64)  NOT NULL,
  payload     JSONB NOT NULL,
  created_at  TIMESTAMP DEFAULT NOW(),
  UNIQUE(room_id, seq)
);
CREATE INDEX idx_room_ops_room_seq ON room_ops(room_id, seq);
```

Snapshot strategy (after seq watermark) prunes `room_ops` rows older
than the latest materialise checkpoint to bound table size.

## Phasing

1. **Phase 1 — Numeric fields only (1 week)**
   Add op log + transform for `x/y/width/height/angle/style attrs`.
   Text and points still use LWW. Behind feature flag, default off.
2. **Phase 2 — Point arrays for arrows/lines (1 week)**
   Index-based insert/delete OT for `points`. Adds risk: this is
   where most TP1 violations historically happen.
3. **Phase 3 — Text labels (1.5 weeks)**
   Char-level Jupiter OT on `text`. Requires careful cursor/selection
   transformation on the FE so users don't lose their place.
4. **Phase 4 — Production rollout (0.5 week)**
   Flip the flag on by default, monitor for divergence (server-side
   diff check between authoritative log replay and client snapshot
   sample).

Total estimate: **5–7 days of coding** + **3–5 days of correctness
testing** with a peer reviewing the transform table. Plan budget at
~2 weeks calendar time including review.

## Risks

- **Correctness over time.** A buggy transform converges fine on
  small examples but diverges on long sessions. Without an
  authoritative test suite of recorded sessions, regressions land
  silently.
- **Rebroadcast amplification.** Rapid drag streams produce one op
  per cursor frame; the log fills fast. Need to coalesce sequential
  ops from the same author + element + field within a short window
  before persisting.
- **Snapshot recovery.** Rebuilding state from log + last snapshot
  must be deterministic. Any non-deterministic transform (random
  tiebreaker, map-iteration order, etc.) wedges replay.
- **Backward compat trap.** Mixed clients (some OT, some LWW) means
  the server must accept both protocols in parallel. Plan to flip
  the LWW path off only after every deployed client supports OT.

## Decision points before kickoff

1. **OT vs CRDT?** Yjs gives correctness guarantees out of the box
   but reshapes the storage layer. For a feature with this much
   risk, evaluating Yjs once before committing to OT is cheap.
2. **Where does materialise run?** In the WS hub goroutine, in a
   dedicated worker, or in Postgres logic? Each option has different
   failure modes during high write volume.
3. **What's the divergence detection plan?** Sample clients send
   periodic state hashes; server compares against its own replay.
   Without this, we won't know we're broken.
4. **What's the rollback plan?** Feature flag + ability to truncate
   `room_ops` and force-resync from `room_elements` on every joined
   client.

## Recommendation

Park OT until at least the following are done:

- A recorded-session replay harness exists (so transforms have a
  regression net).
- The Yjs path has been spike-tested for one element type, to compare
  effort against this OT plan.
- We have monitoring (Phase 13) so divergence in production is
  observable rather than reported by users.

Until then, document the LWW behavior in user-facing docs (e.g.
"simultaneous edits to the same element may keep only one"). That's a
1-paragraph addition vs a 2-week build.
