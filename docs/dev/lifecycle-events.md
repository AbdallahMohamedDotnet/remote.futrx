# Lifecycle events

## Runtime

```mermaid
flowchart LR
    Main["main.go"] --> Registry["lifecycle.Registry"]
    Registry --> Core["publishers.Core"]
    Main --> Bindings["lifecycle.Bindings"]
    Bindings --> Core
    Producer["selfupdate.Service"] -->|"PublishUpdateStarted"| Core
    Core -->|"OnUpdateStarted"| Auth["auth.Service"]
```

```mermaid
sequenceDiagram
    participant P as Producer
    participant C as Core publisher
    participant S1 as Subscriber 1
    participant S2 as Subscriber 2
    P->>C: PublishEvent(ctx, data)
    C->>S1: OnEvent(ctx, event)
    S1-->>C: return
    C->>S2: OnEvent(ctx, event)
    S2-->>C: return
    C-->>P: return
```

## New event · existing publisher

```mermaid
flowchart TD
    E["1 · Event struct"] --> I["2 · Subscriber callback"]
    I --> P["3 · Publish method"]
    P --> O["4 · Producer port"]
    O --> C["5 · Producer call"]
    I --> S["6 · Subscriber implementation"]
    S --> B["7 · Binding"]
```

### `backend/internal/lifecycle/publishers/core.go`

```go
type UpdateCancelledEvent struct {
    Target    string
    Kind      string
    StartedBy string
}

type CoreSubscriber interface {
    OnUpdateStarted(context.Context, UpdateStartedEvent)
    OnUpdateSucceeded(context.Context, UpdateSucceededEvent)
    OnUpdateFailed(context.Context, UpdateFailedEvent)
    OnUpdateCancelled(context.Context, UpdateCancelledEvent)
}

func (c *Core) PublishUpdateCancelled(ctx context.Context, target, kind, startedBy string) {
    event := UpdateCancelledEvent{Target: target, Kind: kind, StartedBy: startedBy}
    for _, subscription := range c.snapshot() {
        subscription.subscriber.OnUpdateCancelled(ctx, event)
    }
}
```

### `backend/internal/service/<producer>/ports.go`

```go
type UpdateLifecyclePublisher interface {
    PublishUpdateCancelled(context.Context, string, string, string)
}
```

### `backend/internal/service/<producer>/service.go`

```go
s.lifecycle.PublishUpdateCancelled(ctx, target, kind, startedBy)
```

### `backend/internal/service/<subscriber>/update_lifecycle.go`

```go
var _ publishers.CoreSubscriber = (*Service)(nil)

func (s *Service) OnUpdateCancelled(
    ctx context.Context,
    event publishers.UpdateCancelledEvent,
) {
    // subscriber-owned reaction
}
```

### `backend/cmd/remote/lifecycle.go`

```go
Core: []publishers.CoreSubscriber{
    services.Auth,
    services.Audit,
},
```

## New publisher

```mermaid
flowchart TD
    Publisher["1 · publishers/jobs.go"] --> Registry["2 · Registry.Jobs"]
    Registry --> BindingType["3 · Bindings.Jobs"]
    BindingType --> Bind["4 · Bind: Jobs.Subscribe"]
    Registry --> Producer["5 · Inject registry.Jobs"]
    Bind --> Composition["6 · cmd/remote/lifecycle.go"]
    Producer --> Dispatch["PublishJobStarted"]
    Composition --> Dispatch
```

### `backend/internal/lifecycle/publishers/jobs.go`

```go
type JobStartedEvent struct {
    ID string
}

type JobsSubscriber interface {
    OnJobStarted(context.Context, JobStartedEvent)
}

type Jobs struct {
    // Subscribe + snapshot ownership: core.go
}

func NewJobs() *Jobs
func (j *Jobs) Subscribe(JobsSubscriber) func()
func (j *Jobs) PublishJobStarted(context.Context, string)
```

### `backend/internal/lifecycle/registry.go`

```go
type Registry struct {
    Core *publishers.Core
    Jobs *publishers.Jobs
}

func NewRegistry() *Registry {
    return &Registry{
        Core: publishers.NewCore(),
        Jobs: publishers.NewJobs(),
    }
}
```

### `backend/internal/lifecycle/bindings.go`

```go
type Bindings struct {
    Core []publishers.CoreSubscriber
    Jobs []publishers.JobsSubscriber
}

for _, subscriber := range bindings.Jobs {
    unsubscribes = append(unsubscribes, registry.Jobs.Subscribe(subscriber))
}
```

### `backend/cmd/remote/lifecycle.go`

```go
return lifecycle.Bind(registry, lifecycle.Bindings{
    Core: []publishers.CoreSubscriber{
        services.Auth,
    },
    Jobs: []publishers.JobsSubscriber{
        services.Audit,
    },
})
```

## Verification

```bash
go test -race ./internal/lifecycle/... ./internal/service/<producer> ./internal/service/<subscriber>
go test ./...
go vet ./...
```
