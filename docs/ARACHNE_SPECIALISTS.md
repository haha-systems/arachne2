# Arachne specialist proposals

The specialist contract separates activation from proposal. A coordinator
sends the same `specialist.activate` interaction to a bounded set of registered
specialists. Each specialist receives an interaction ID, JSON input, session
context, and source event IDs. It returns a `specialist.proposal` message to the
coordinator with a generated proposal ID, specialist identity, correlation ID,
summary, confidence, evidence IDs, requested action intents, and creation time.

The specialist wrapper validates input and proposal bounds, assigns identity
and attribution itself, and emits perception, activation, and proposal records
to the shared cognitive event spine. Failed or invalid proposals are returned
with `status: failed` and an error, and are also observable in the event spine.
Multiple specialists can answer the same interaction independently; message
delivery order is the router's accepted order and does not imply proposal
quality or preference.

`RequestedAction` is a description of intent with a JSON payload. The agent has
no action executor or host capability in this contract. Proposal receipt does
not select or execute it. Bounded comparison, admission, and selection belong
to the workspace layer in AR-09; consequential execution remains behind later
governance and explicit host interfaces.

Run the standalone in-process demonstration with:

```sh
cd arachne
go run ./examples/specialist-proposals
```

It activates a planner and a critic for one interaction, collects both
proposals, and reports the shared event count without executing either
requested action.
