package module

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

type testProvider struct {
	id agent.ProviderID
}

func (p testProvider) ID() agent.ProviderID { return p.id }

func (p testProvider) Capabilities(context.Context, agent.CapabilityRequest) (agent.Capabilities, error) {
	return agent.Capabilities{Provider: p.id}, nil
}

func (p testProvider) Run(context.Context, agent.RunRequest, func(agent.Event)) error {
	return nil
}

func TestCatalogBuildsFakeFifthAgentWithoutProviderSwitches(t *testing.T) {
	factory := newTestFactory(t, "minimax")
	catalog, err := NewCatalog(factory)
	if err != nil {
		t.Fatal(err)
	}

	runtime, err := catalog.Build(Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Providers.Lookup("minimax") == nil {
		t.Fatal("fake fifth provider was not registered")
	}
	if binding, ok := runtime.Auth.Lookup("minimax"); !ok || binding.Flow() != agentauth.FlowExternal {
		t.Fatalf("fake fifth auth binding = (%#v, %t)", binding, ok)
	}
	profiles := catalog.Profiles()
	if len(profiles) != 1 || profiles[0].ID != "minimax" {
		t.Fatalf("profiles = %#v", profiles)
	}
}

func TestNewFactoryRejectsInvalidDeclarations(t *testing.T) {
	valid := testDescriptor("future-agent")
	tests := map[string]Descriptor{
		"empty ID": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.ID = ""
			return descriptor
		}(),
		"unsafe ID": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.ID = "Future Agent"
			return descriptor
		}(),
		"blank label": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.Label = "  "
			return descriptor
		}(),
		"managed auth without instructions": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.Auth = AuthManagedCode
			descriptor.AuthInstructions = ""
			return descriptor
		}(),
		"external auth gate": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.SatisfiesAccessGate = true
			return descriptor
		}(),
		"project-only default": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.Default = true
			descriptor.ExecutionScopes = []ExecutionScope{ScopeProject}
			return descriptor
		}(),
		"missing scope": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.ExecutionScopes = nil
			return descriptor
		}(),
		"duplicate scope": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.ExecutionScopes = []ExecutionScope{ScopeProject, ScopeProject}
			return descriptor
		}(),
		"profile mismatch": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.Profile.ID = "other"
			return descriptor
		}(),
		"incomplete CLI": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.Profile.CLI.Binary = ""
			return descriptor
		}(),
		"fork without resume": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.Features.Sessions = SessionSupport{Fork: true}
			return descriptor
		}(),
		"unsafe persistent host": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.Profile.PersistentState = []provisioning.PersistentDirectory{{
				Device: "future-home", HostDirectory: "../future", ContainerPath: "/root/.future",
			}}
			return descriptor
		}(),
		"relative persistent target": func() Descriptor {
			descriptor := cloneDescriptor(valid)
			descriptor.Profile.PersistentState = []provisioning.PersistentDirectory{{
				Device: "future-home", HostDirectory: "future", ContainerPath: "root/.future",
			}}
			return descriptor
		}(),
	}
	for name, descriptor := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewFactory(descriptor, testBuild(descriptor.ID)); !errors.Is(err, ErrInvalidFactory) {
				t.Fatalf("NewFactory error = %v, want ErrInvalidFactory", err)
			}
		})
	}
	if _, err := NewFactory(valid, nil); !errors.Is(err, ErrInvalidFactory) {
		t.Fatalf("nil builder error = %v, want ErrInvalidFactory", err)
	}
}

func TestCatalogRejectsPersistentStateCollisions(t *testing.T) {
	first := testDescriptor("first-agent")
	first.Profile.PersistentState = []provisioning.PersistentDirectory{{
		Device: "shared-home", HostDirectory: "first", ContainerPath: "/root/.first",
	}}
	second := testDescriptor("second-agent")
	second.Profile.PersistentState = []provisioning.PersistentDirectory{{
		Device: "shared-home", HostDirectory: "second", ContainerPath: "/root/.second",
	}}
	firstFactory, err := NewFactory(first, testBuild(first.ID))
	if err != nil {
		t.Fatal(err)
	}
	secondFactory, err := NewFactory(second, testBuild(second.ID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCatalog(firstFactory, secondFactory); !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("NewCatalog error = %v, want ErrInvalidCatalog", err)
	}
}

func TestCatalogRejectsDuplicateFactories(t *testing.T) {
	factory := newTestFactory(t, "future-agent")
	if _, err := NewCatalog(factory, factory); !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("NewCatalog error = %v, want ErrInvalidCatalog", err)
	}
}

func TestCatalogRejectsMultipleDefaultProviders(t *testing.T) {
	first := testDescriptor("first-agent")
	first.Default = true
	second := testDescriptor("second-agent")
	second.Default = true
	firstFactory, err := NewFactory(first, testBuild(first.ID))
	if err != nil {
		t.Fatal(err)
	}
	secondFactory, err := NewFactory(second, testBuild(second.ID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCatalog(firstFactory, secondFactory); !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("NewCatalog error = %v, want ErrInvalidCatalog", err)
	}
}

func TestFactoryRejectsRuntimeIdentityAndAuthMismatches(t *testing.T) {
	tests := map[string]BuildFunc{
		"provider ID": func(Dependencies) (Components, error) {
			binding := agentauth.NewExternalBinding("future-agent")
			return Components{Provider: testProvider{id: "other"}, Auth: &binding}, nil
		},
		"auth ID": func(Dependencies) (Components, error) {
			binding := agentauth.NewExternalBinding("other")
			return Components{Provider: testProvider{id: "future-agent"}, Auth: &binding}, nil
		},
		"auth flow": func(Dependencies) (Components, error) {
			binding := agentauth.NewCodeBinding("future-agent", agentauth.NewCodeService(agentauth.CodeConfig{}))
			return Components{Provider: testProvider{id: "future-agent"}, Auth: &binding}, nil
		},
		"missing auth": func(Dependencies) (Components, error) {
			return Components{Provider: testProvider{id: "future-agent"}}, nil
		},
	}
	for name, build := range tests {
		t.Run(name, func(t *testing.T) {
			factory, err := NewFactory(testDescriptor("future-agent"), build)
			if err != nil {
				t.Fatal(err)
			}
			catalog, err := NewCatalog(factory)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := catalog.Build(Dependencies{}); !errors.Is(err, ErrInvalidFactory) {
				t.Fatalf("Build error = %v, want ErrInvalidFactory", err)
			}
		})
	}
}

func TestFactoryRejectsUnavailableManagedAuth(t *testing.T) {
	descriptor := testDescriptor("future-agent")
	descriptor.Auth = AuthManagedCode
	descriptor.AuthInstructions = "Complete the code flow."
	factory, err := NewFactory(descriptor, func(Dependencies) (Components, error) {
		binding := agentauth.NewCodeBinding("future-agent", nil)
		return Components{Provider: testProvider{id: "future-agent"}, Auth: &binding}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := NewCatalog(factory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Build(Dependencies{}); !errors.Is(err, ErrInvalidFactory) {
		t.Fatalf("Build error = %v, want ErrInvalidFactory", err)
	}
}

func TestCatalogSelectsDefaultsAndEvaluatesAccessGate(t *testing.T) {
	authenticated := false
	managed := testDescriptor("managed-agent")
	managed.Default = true
	managed.Auth = AuthManagedDevice
	managed.AuthInstructions = "Complete the device flow."
	managed.SatisfiesAccessGate = true
	managedFactory, err := NewFactory(managed, func(Dependencies) (Components, error) {
		service := agentauth.NewDeviceService(agentauth.DeviceConfig[bindingTestAuthStatus]{
			Authenticated: func() bool { return authenticated },
			BuildStatus: func() agentauth.DeviceStatusBuilder[bindingTestAuthStatus] {
				return func(state agentauth.DeviceState) bindingTestAuthStatus {
					return bindingTestAuthStatus{Authenticated: authenticated, DeviceLogin: state}
				}
			},
		})
		binding := agentauth.NewDeviceBinding(managed.ID, service)
		return Components{Provider: testProvider{id: managed.ID}, Auth: &binding}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	externalFactory := newTestFactory(t, "external-agent")
	catalog, err := NewCatalog(externalFactory, managedFactory)
	if err != nil {
		t.Fatal(err)
	}
	if got := catalog.DefaultProvider(ScopeHost); got != managed.ID {
		t.Fatalf("default provider = %q, want %q", got, managed.ID)
	}
	runtime, err := catalog.Build(Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if catalog.AccessReady(runtime.Auth) {
		t.Fatal("access gate opened before managed authentication")
	}
	authenticated = true
	if !catalog.AccessReady(runtime.Auth) {
		t.Fatal("access gate stayed closed after managed authentication")
	}

	noAuth := testDescriptor("no-auth-agent")
	noAuth.Auth = AuthNone
	noAuth.AuthInstructions = ""
	noAuth.SatisfiesAccessGate = true
	noAuthFactory, err := NewFactory(noAuth, func(Dependencies) (Components, error) {
		return Components{Provider: testProvider{id: noAuth.ID}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	noAuthCatalog, err := NewCatalog(noAuthFactory)
	if err != nil {
		t.Fatal(err)
	}
	if !noAuthCatalog.AccessReady(nil) {
		t.Fatal("no-auth gate provider was not immediately ready")
	}
}

type bindingTestAuthStatus struct {
	Authenticated bool                  `json:"authenticated"`
	DeviceLogin   agentauth.DeviceState `json:"deviceLogin"`
}

func TestCatalogReturnsDefensiveOrderedSnapshots(t *testing.T) {
	firstDescriptor := testDescriptor("first-agent")
	firstDescriptor.LegacySkillRoots = []string{"/root/.first/skills"}
	firstDescriptor.Profile.Credentials.Files = []provisioning.CredentialFile{{HostPath: "original"}}
	firstDescriptor.Profile.PersistentState = []provisioning.PersistentDirectory{{
		Device: "first-home", HostDirectory: "first", ContainerPath: "/root/.first",
	}}
	firstDescriptor.Profile.BrowserMCPTemplates = []provisioning.TemplateFile{{Content: []byte("original")}}
	firstFactory, err := NewFactory(firstDescriptor, testBuild(firstDescriptor.ID))
	if err != nil {
		t.Fatal(err)
	}
	secondFactory := newTestFactory(t, "second-agent")
	catalog, err := NewCatalog(firstFactory, secondFactory)
	if err != nil {
		t.Fatal(err)
	}

	firstDescriptor.ExecutionScopes[0] = "changed"
	firstDescriptor.LegacySkillRoots[0] = "/changed"
	firstDescriptor.Profile.Credentials.Files[0].HostPath = "changed"
	firstDescriptor.Profile.PersistentState[0].ContainerPath = "/changed"
	firstDescriptor.Profile.BrowserMCPTemplates[0].Content[0] = 'x'

	descriptors := catalog.Descriptors()
	if got := []agent.ProviderID{descriptors[0].ID, descriptors[1].ID}; !slices.Equal(got, []agent.ProviderID{"first-agent", "second-agent"}) {
		t.Fatalf("descriptor order = %v", got)
	}
	descriptors[0].ExecutionScopes[0] = "changed-again"
	descriptors[0].LegacySkillRoots[0] = "/changed-again"
	descriptors[0].Profile.Credentials.Files[0].HostPath = "changed-again"
	descriptors[0].Profile.PersistentState[0].ContainerPath = "/changed-again"
	descriptors[0].Profile.BrowserMCPTemplates[0].Content[0] = 'y'

	fresh := catalog.Descriptors()[0]
	if fresh.ExecutionScopes[0] != ScopeHost || fresh.LegacySkillRoots[0] != "/root/.first/skills" ||
		fresh.Profile.Credentials.Files[0].HostPath != "original" ||
		fresh.Profile.PersistentState[0].ContainerPath != "/root/.first" ||
		string(fresh.Profile.BrowserMCPTemplates[0].Content) != "original" {
		t.Fatalf("catalog descriptor mutated through a snapshot: %#v", fresh)
	}
	profiles := catalog.Profiles()
	profiles[0].Credentials.Files[0].HostPath = "profile-change"
	if got := catalog.Profiles()[0].Credentials.Files[0].HostPath; got != "original" {
		t.Fatalf("catalog profile mutated through a snapshot: %q", got)
	}
}

func TestCatalogEnforcesDeclaredExecutionScopes(t *testing.T) {
	host := testDescriptor("host-agent")
	host.ExecutionScopes = []ExecutionScope{ScopeHost}
	host.Profile = nil
	project := testDescriptor("project-agent")
	project.ExecutionScopes = []ExecutionScope{ScopeProject}
	hostFactory, err := NewFactory(host, testBuild(host.ID))
	if err != nil {
		t.Fatal(err)
	}
	projectFactory, err := NewFactory(project, testBuild(project.ID))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := NewCatalog(hostFactory, projectFactory)
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.SupportsScope("host-agent", ScopeHost) || catalog.SupportsScope("host-agent", ScopeProject) {
		t.Fatal("host-agent scope policy is incorrect")
	}
	if !catalog.SupportsScope("project-agent", ScopeProject) || catalog.SupportsScope("project-agent", ScopeHost) {
		t.Fatal("project-agent scope policy is incorrect")
	}
}

func newTestFactory(t *testing.T, id agent.ProviderID) Factory {
	t.Helper()
	factory, err := NewFactory(testDescriptor(id), testBuild(id))
	if err != nil {
		t.Fatal(err)
	}
	return factory
}

func testBuild(id agent.ProviderID) BuildFunc {
	return func(Dependencies) (Components, error) {
		binding := agentauth.NewExternalBinding(id)
		return Components{Provider: testProvider{id: id}, Auth: &binding}, nil
	}
}

func testDescriptor(id agent.ProviderID) Descriptor {
	profile := provisioning.Profile{
		ID: string(id),
		CLI: provisioning.CLISpec{
			Name:        "Future Agent",
			ImageLabel:  "future-agent",
			Binary:      "future-agent",
			PackageName: "future-agent-cli",
			Version:     "1.0.0",
			InstallMode: provisioning.InstallWithNPM,
		},
	}
	return Descriptor{
		ID:               id,
		Label:            strings.ToUpper(string(id[:1])) + string(id[1:]),
		ExecutionScopes:  []ExecutionScope{ScopeHost, ScopeProject},
		Auth:             AuthExternal,
		AuthInstructions: "Run the provider login command.",
		Features: Features{
			Sessions:       SessionSupport{Resume: true},
			Skills:         SkillsInstructions,
			ScheduledTools: true,
		},
		Profile: &profile,
	}
}
