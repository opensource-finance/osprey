# Async Bus

Pro tier workers read transaction messages from the bus.
This document describes the message that a producer publishes to `osprey.transaction.ingested`.
The worker runs the same rule evaluation as the sync path on `POST /evaluate`.

## Topic

A producer publishes to this subject:

```text
osprey.transaction.ingested
```

Subjects are tenant-scoped.
The worker attributes each message to its subscription tenant, except on the global worker.
For the tenant attribution rules, see [ontology.html](ontology.html).

## Message Schema

The JSON payload is a `TransactionMessage`.
The field names differ from `POST /evaluate` because the worker is a separate contract.

| Field | Type | Required | Notes |
|------|------|---------|------|
| `txId` | string | yes | The transaction ID. |
| `tenantId` | string | yes on the global worker | The subscription tenant wins on a per-tenant worker. |
| `traceId` | string | no | The worker falls back to the message ID when this is empty. |
| `type` | string | no | The worker uppercases the value. |
| `debtorId` | string | no | The debtor ID. |
| `creditorId` | string | no | The creditor ID. |
| `debtorName` | string | no | Used for sanctions screening. |
| `creditorName` | string | no | Used for sanctions screening. |
| `amount` | number | yes | The transaction amount as a flat number. |
| `currency` | string | no | The worker uppercases the value. |
| `timestamp` | string | no | An RFC3339 timestamp. Empty defaults to the current time. |
| `velocityWindow` | integer | no | The window in seconds. The default is `3600`. |
| `metadata` | object | no | Caller facts. Rules get these in the `meta` map. |
| `enrichment` | object | no | Externally-computed scores and flags. |

## Metadata

The `metadata` field has the same contract as `metadata` on `POST /evaluate`.
Rules get the facts in the `meta` map.
Use `has()` for optional keys.

`old_balance` and `new_balance` must be numbers or `null`.
A non-numeric balance makes the worker drop the message.
The worker does not publish a decision in that case.
This matches the `400` response on the sync path.

The legacy `additionalData` field is an alias for `metadata`.
The `metadata` key wins when a producer sends both keys.
Use `metadata` for new producers.

## Velocity Window

The worker does not save the transaction.
Velocity reads only stored transactions.
A bus-only transaction has a velocity of zero for that reason.
For the gap and the event clock, see [ontology.html](ontology.html).

## Example

```json
{
  "txId": "tx-001",
  "tenantId": "tenant-a",
  "traceId": "trace-001",
  "type": "transfer",
  "debtorId": "d-1",
  "creditorId": "c-1",
  "debtorName": "Alice",
  "amount": 15000.0,
  "currency": "USD",
  "timestamp": "2026-05-25T09:16:30Z",
  "metadata": {
    "old_balance": 15000.0,
    "new_balance": 0.0,
    "country": "US"
  }
}
```
