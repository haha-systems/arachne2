# Arachne regulation and prediction error

`regulation.Regulator` evaluates explicit inputs for one interaction and emits
linked `prediction_error` and `regulation` events. Inputs include an expected
and observed JSON outcome, caller-declared salience, active specialists and
their capacity, pending actions and their capacity, and the workspace's base
admission capacity. All source event IDs are retained. Expected and observed
values are represented in events by canonical JSON digests rather than raw
contents.

## Signal definitions

- **Prediction error / surprise:** for numeric JSON values, `min(1, abs(observed - expected) / max(1, abs(expected)))`. For other JSON values, canonical equality yields `0`; any mismatch yields `1`. Surprise currently equals prediction-error magnitude.
- **Salience:** the caller's declared number in `[0, 1]`; it is not inferred from language or treated as an objective property.
- **Load:** active specialists divided by specialist capacity, where counts must fit the configured capacity.
- **Action pressure:** pending actions divided by action capacity, with the same bounded-count rule.
- **Cognitive state:** `strained` when load or action pressure reaches its threshold; otherwise `alert` when surprise or salience reaches its threshold; otherwise `steady`.

The default policy starts with thresholds `0.75` for load, action pressure, and
salience, `0.5` for surprise, and a capacity reduction of one. These are
configurable starting values, not research-derived organism constants.

## Explicit effect on workspace behavior

The evaluation returns a `workspace.SelectionPolicy` with signal event IDs and
plain-language reasons. Each high load or action-pressure signal reduces the
workspace admission capacity by the configured amount, clamped to at least one.
High surprise or high declared salience requires candidate proposals to carry
evidence event links. `Workspace.SelectWithPolicy` records that adjusted
capacity, evidence requirement, reasons, and signal IDs alongside every
selected and rejected proposal. Its selection event becomes a parent of the
subsequent broadcast decision event.

This makes the signal-to-behavior path inspectable without embedding regulation
inside Silk. It changes proposal admission policy only; it does not change Silk
host grants, authorize actions, promote memories, or approve consequential
changes. The integrated [`specialist-proposals` example](../arachne/examples/specialist-proposals/main.go)
shows a high-surprise, high-load interaction reducing capacity from two to one.

The arithmetic is an initial, falsifiable control rule, not a claim of causal
learning or general active inference. Inputs are supplied explicitly; the
regulator does not yet estimate salience, load, action pressure, or predictions
from an autonomous sensor. AR-11 owns validation of whether these configured
effects change cognition as intended. The reference inventory's local signal
and coordination measurements remain bounded fixture evidence, not production
or general-cognition evidence.
