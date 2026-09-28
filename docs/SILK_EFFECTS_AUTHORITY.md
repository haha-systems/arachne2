# Silk effects, grants, and authority checks

**Task:** SILK-06  
**Scope:** Static effect summaries and conservative runtime authorization for host calls.  
**Depends on:** [Host protocol contract](SILK_HOST_PROTOCOL.md) and the Rust runtime from SILK-02.

Effects describe what execution may do. Grants describe which declared host
functions a session may invoke. They are related but distinct: an effect does
not grant permission, and a grant does not cause an effect by itself.

## Effect vocabulary

Every callable has a finite effect set. The Phase 1/2 core vocabulary is:

| Effect                | Meaning                                                       | Example                                                   |
| --------------------- | ------------------------------------------------------------- | --------------------------------------------------------- |
| `pure`                | Computes a result without changing observable state.          | arithmetic, `len`, string operations                      |
| `session_state_read`  | Reads session-local mutable bindings.                         | reading a `state` value                                   |
| `session_state_write` | Changes session-local mutable bindings.                       | assigning a `state` value                                 |
| `process_output`      | Emits data to a runtime-controlled output channel.            | `print`                                                   |
| `clock`               | Reads nondeterministic time or a host-supplied time value.    | `now`                                                     |
| `delay`               | Suspends the current execution for a requested duration.      | `sleep`                                                   |
| `host_read`           | Requests information from an external host service.           | search or retrieval function                              |
| `host_write`          | Requests a host-owned state change.                           | write or task submission function                         |
| `host_privileged`     | Requests a host action with explicitly elevated consequences. | deployment or protected configuration change              |
| `unknown`             | The callable or its effects cannot be statically resolved.    | computed function value without a declared effect summary |

`pure` is the empty set, not a permission category. A function may have more
than one effect. For example, a host function that reads a resource and emits
an audit record has `{host_read, host_write}`. `unknown` is the top element for
static analysis: it conservatively means that any effect may occur.

`trace.emit` is runtime observability, not a Silk program effect and not a host
grant. Trace delivery failure is a runtime/protocol error. Host functions carry
the effect labels declared in their session descriptors; unknown labels are a
session creation error.

## Effect inference

The compiler computes a conservative effect summary for every procedure:

1. Collect direct effects from assignments, core functions, lifecycle forms,
   and declared host-function calls.
2. Add the summaries of statically resolved callees.
3. Propagate summaries across the call graph until they stop changing; recursive
   call components converge by finite set union.
4. If a call target is unresolved or dynamically selected without an explicit
   declared summary, include `unknown` and the union of all declared effect
   classes that target could resolve to.

The summary is an upper bound. Runtime checks still apply to every host call,
including calls in statically pure-looking or unreachable branches. A declared
procedure effect ceiling, when supplied, must contain the inferred set. A
ceiling smaller than the inferred set is a compile/load error. Omitting a
ceiling does not suppress inference or runtime checks.

Static output reports procedure identity, direct and transitive effect sets,
host-function dependencies, unknown-call sites, and whether an explicit ceiling
was satisfied. It does not claim that a call will execute, that a host function
is safe, or that the session authorizes the call.

## Grant model

A grant is session-scoped and names one authority identifier. The grant set is
provided by the host at `session.create`; Silk code cannot add, widen, delegate,
or copy grants into another session. No wildcard grants are valid in protocol
version 1.0.

A host-function descriptor declares:

- a unique function name;
- a unique authority identifier required for invocation;
- its effect set;
- an input schema and, when present, output schema.

For the initial protocol, a grant entry names an authority identifier and an
effect ceiling. The runtime may dispatch only when one grant has the exact
authority identifier and its ceiling contains every effect declared by the
function. The session catalog must also contain the exact function name. A
declaration without a matching grant is callable in name resolution but denied
at runtime. A grant without a matching descriptor has no callable effect.

```json
{
  "name": "language.summarize",
  "authority": "language.summarize",
  "effect": ["host_read"],
  "inputSchema": { "type": "object" }
}
```

```json
{
  "authority": "language.summarize",
  "effect_ceiling": ["host_read"]
}
```

The host owns implementation-specific resource checks and credentials. If a
host needs path-, tenant-, or record-level restriction, it must expose a
descriptor/authority pair whose own contract enforces that restriction. Silk
must not infer resource safety from a function name or caller identity.

## Runtime check order

Before any host code can run, the runtime performs these checks in order:

1. Resolve the exact function name in the current session catalog.
2. Validate the argument JSON value against the descriptor input schema.
3. Confirm the call's declared effect set is covered by the active procedure's
   effect ceiling, if any.
4. Find a session grant with an exact authority identifier match and a ceiling
   covering the full effect set.
5. Check that the session is open and its deadline/cancellation state allows a
   new external call.
6. Emit the authority decision and only then dispatch `host.call` to the host.

Any failed check stops before step 6. In particular, the host must not receive
an invocation for an unauthorized call. The runtime does not partially grant
effects, strip an argument to make a request pass, retry a denied call, or fall
back to a different host function.

`AuthorityDenied` is a structured runtime error containing session ID, call
identity, function name, required authority, required effects, and the failed
check reason. It must not expose the complete grant list or secrets. A trace
records the denial decision and confirms that dispatch did not occur. A
missing descriptor is `HostFunctionNotDeclared`, not `AuthorityDenied`.

## Failure and replay behavior

- Invalid arguments fail before the authority decision and before host dispatch.
- A declared but ungranted function fails closed with `AuthorityDenied`.
- A granted call whose host fails becomes `HostError`; the host error does not
  imply that the operation had no partial external effect.
- A transport failure after dispatch has an unknown outcome. Runtime does not
  retry automatically; caller/recovery policy must decide what to do.
- Recorded nondeterministic host-call results may be replayed only when the
  recording matches function identity, arguments, session/program identity,
  and sequence position. Replay supplies the recorded result without invoking
  the live host.
- A recording cannot grant authority. A recorded success does not override a
  denied call in a session whose active grants do not permit that call.

## Inspectable findings and trace decisions

Static inspection reports a **possible effect** and **required authority** for
every host call reachable from an entry procedure. Runtime trace reports the
actual authority decision for each attempted call:

```json
{
  "event": "authority.decision",
  "session_id": "session-42",
  "call_id": "call-7",
  "function": "language.summarize",
  "required_authority": "language.summarize",
  "required_effects": ["host_read"],
  "decision": "denied",
  "reason": "grant_missing",
  "dispatched": false
}
```

Trace payloads must redact sensitive arguments according to the trace contract;
redaction may not hide the function identity, effect set, decision, or whether
dispatch occurred. Static and runtime effect labels use stable protocol strings
and are versioned with the protocol.

## Conformance cases for the implementation

SILK-02/SILK-09 runtime tests must prove at least:

- exact matching grant dispatches the declared host function once;
- missing descriptor yields `HostFunctionNotDeclared` and zero dispatches;
- descriptor with no matching grant yields `AuthorityDenied` and zero dispatches;
- a grant with a narrower effect ceiling is denied before dispatch;
- malformed arguments are rejected before dispatch;
- a denied call appears in trace with `dispatched: false`;
- recursive call graphs converge to a stable transitive effect set;
- unresolved indirect calls include `unknown` and do not gain authority;
- a host failure is traced as `HostError` and is not silently replayed as success;
- identical sessions do not share grants or decisions.

## Notes for follow-on work

- The runtime now has protocol effect labels, session descriptor/grant data
  types, and a `dispatch_host_call` gate. Exact function lookup, exact
  authority matching, effect-ceiling coverage, and fail-closed unknown effects
  happen before its dispatch closure can run. The structured error and
  authorization types are serializable for later trace/protocol integration.
- `analyze_effects` accepts procedure/call-graph facts and produces serializable
  direct/transitive effects, host dependencies, required authorities,
  unresolved call sites, and effect-ceiling violations. Recursive graphs
  converge by finite-set union.
- SILK-08/09 classify dotted source calls as host functions and invoke the
  dispatch gate immediately before provider calls. Argument-schema validation,
  live procedure effect ceilings, session cancellation/deadline checks, and
  trace emission remain later work.
- Effect extraction from lowered IR into `analyze_effects` is not connected; the
  analyzer still accepts an explicit procedure/call graph from its caller.
- Keep effect labels separate from authority IDs: two functions may have the
  same effect and require different authorities, and one function may require
  more than one effect.
- SILK-07 must carry effect sets and required authorities in IR so SILK-12 can
  inspect them without source. SILK-11 must preserve the denial decision and
  dispatch flag in traces.
- Arachne-only functions belong in Arachne's host catalog. They do not become
  Silk syntax or receive ambient authority when Arachne hosts a session.
