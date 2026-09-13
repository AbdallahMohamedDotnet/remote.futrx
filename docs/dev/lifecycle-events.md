# Lifecycle events

`internal/lifecycle` provides typed, synchronous notifications for application
self-update transitions. The process composition root creates one
`UpdatePublisher` and gives producers only the publish methods declared by
their own service ports. No production subscriber is currently registered.

## Delivery contract

`UpdateEvent` carries only the target release, update kind, initiating account,
and one of the defined `UpdateState` values: `started`, `succeeded`, or `failed`.
It deliberately excludes logs, credentials, and mutable run state.

For every publish call, `UpdatePublisher`:

1. snapshots the current subscriber list;
2. calls that snapshot synchronously in registration order;
3. passes through the producer's context and one immutable event value;
4. allows subscriptions to change during a callback without deadlocking.

Subscriber errors are handled by the subscriber because the callback cannot
return an error. A panic propagates to the producer. Long-running reactions
must own their own queue rather than blocking publisher dispatch.

## Adding a subscriber

Implement the single callback and register the service at the process
composition root after all of its dependencies have been constructed:

```go
type auditSubscriber struct{}

func (auditSubscriber) OnUpdate(ctx context.Context, event lifecycle.UpdateEvent) {
    // React only to states owned by this subscriber.
}

unsubscribe := updateLifecycle.Subscribe(auditSubscriber{})
defer unsubscribe()
```

The returned cleanup function is idempotent. Do not add an aggregate registry,
binding catalog, or package-level singleton for a fixed composition
relationship.

## Adding a transition

When a new self-update transition has the same payload:

1. add its `UpdateState` constant;
2. add the matching publish method to `UpdatePublisher`;
3. add only that publish method to the producer-owned port;
4. publish it at the service operation that owns the transition.

If a transition needs different data, give it a separate precise event type
instead of adding optional fields to `UpdateEvent`.

## Verification

```bash
go test -race ./internal/lifecycle ./internal/service/selfupdate
go test ./...
go vet ./...
```
