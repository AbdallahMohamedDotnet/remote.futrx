// Package module defines the provider-neutral contract implemented by every
// agent integration. A module combines static behavior metadata with the
// factory that creates its runtime provider and optional auth binding.
package module

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

var (
	ErrInvalidFactory = errors.New("invalid agent module factory")
)

type ExecutionScope string

const (
	ScopeHost    ExecutionScope = "host"
	ScopeProject ExecutionScope = "project"
)

type AuthMode string

const (
	AuthManagedCode   AuthMode = "managed-code"
	AuthManagedDevice AuthMode = "managed-device"
	AuthExternal      AuthMode = "external"
	AuthNone          AuthMode = "none"
)

type SkillStrategy string

const (
	SkillsNone          SkillStrategy = "none"
	SkillsSlashCommand  SkillStrategy = "slash-command"
	SkillsDollarMention SkillStrategy = "dollar-mention"
	SkillsInstructions  SkillStrategy = "instructions"
)

// SessionSupport describes which provider-native conversation operations the
// orchestration layer may use. Fork implies resume support.
type SessionSupport struct {
	Resume bool
	Fork   bool
}

// Features describes optional behavior the platform may expose for an agent.
// The provider still owns the concrete CLI flags and protocol translation.
type Features struct {
	Sessions       SessionSupport
	Skills         SkillStrategy
	BrowserTools   bool
	ScheduledTools bool
}

// Descriptor is the stable, provider-neutral declaration consumed by runtime
// policy and presentation. Installation and filesystem policy is deliberately
// kept in the factory's separate provisioning profile.
type Descriptor struct {
	ID                  agent.ProviderID
	Label               string
	Default             bool
	ExecutionScopes     []ExecutionScope
	Auth                AuthMode
	AuthInstructions    string
	SatisfiesAccessGate bool
	LegacySkillRoots    []string
	Features            Features
}

type Dependencies struct {
	Projects              agent.ProjectResolver
	Containers            provisioning.ContainerDependencies
	CredentialSyncTimeout time.Duration
}

// Components are the runtime objects produced by a module. Auth is nil only
// when the descriptor explicitly declares AuthNone.
type Components struct {
	Provider agent.Provider
	Auth     *agentauth.Binding
}

// BuildFunc creates one provider runtime from application dependencies and the
// exact provisioning profile already validated by the factory. The profile is
// nil only for a host-only module that requires no local CLI installation.
type BuildFunc func(Dependencies, *provisioning.Profile) (Components, error)

// FactoryBuilder is the compile-time constructor contract implemented by each
// provider package and consumed by the explicit config composition root.
type FactoryBuilder func() (Factory, error)

// Factory is immutable after construction. Descriptor and provisioning profile
// data are cloned at the boundary so callers cannot mutate registered policy.
type Factory struct {
	descriptor Descriptor
	profile    *provisioning.Profile
	build      BuildFunc
}

func NewFactory(
	descriptor Descriptor,
	profile *provisioning.Profile,
	build BuildFunc,
) (Factory, error) {
	descriptor = cloneDescriptor(descriptor)
	profile = cloneProfile(profile)
	if err := validateDescriptor(descriptor, profile); err != nil {
		return Factory{}, err
	}
	if build == nil {
		return Factory{}, fmt.Errorf("%w: provider %q has no builder", ErrInvalidFactory, descriptor.ID)
	}
	return Factory{descriptor: descriptor, profile: profile, build: build}, nil
}

func (f Factory) Descriptor() Descriptor {
	return cloneDescriptor(f.descriptor)
}

func (f Factory) buildComponents(deps Dependencies) (Components, error) {
	if f.build == nil {
		return Components{}, fmt.Errorf("%w: provider %q has no builder", ErrInvalidFactory, f.descriptor.ID)
	}
	var profile *provisioning.Profile
	if f.profile != nil {
		cloned := f.profile.Clone()
		profile = &cloned
	}
	components, err := f.build(deps, profile)
	if err != nil {
		return Components{}, fmt.Errorf("build agent %q: %w", f.descriptor.ID, err)
	}
	if components.Provider == nil {
		return Components{}, fmt.Errorf("%w: provider %q builder returned nil", ErrInvalidFactory, f.descriptor.ID)
	}
	if components.Provider.ID() != f.descriptor.ID {
		return Components{}, fmt.Errorf(
			"%w: descriptor %q built provider %q",
			ErrInvalidFactory,
			f.descriptor.ID,
			components.Provider.ID(),
		)
	}
	if err := validateAuth(f.descriptor, components.Auth); err != nil {
		return Components{}, err
	}
	return components, nil
}

func validateDescriptor(descriptor Descriptor, profile *provisioning.Profile) error {
	id := string(descriptor.ID)
	if !agent.ValidProviderID(descriptor.ID) {
		return fmt.Errorf("%w: provider ID %q is invalid", ErrInvalidFactory, id)
	}
	if strings.TrimSpace(descriptor.Label) == "" {
		return fmt.Errorf("%w: provider %q has no label", ErrInvalidFactory, descriptor.ID)
	}
	if err := validateScopes(descriptor, profile); err != nil {
		return err
	}
	if err := validateAuthMode(descriptor); err != nil {
		return err
	}
	if descriptor.Features.Sessions.Fork && !descriptor.Features.Sessions.Resume {
		return fmt.Errorf("%w: provider %q declares fork without resume", ErrInvalidFactory, descriptor.ID)
	}
	switch descriptor.Features.Skills {
	case SkillsNone, SkillsSlashCommand, SkillsDollarMention, SkillsInstructions:
	default:
		return fmt.Errorf("%w: provider %q has unknown skill strategy %q", ErrInvalidFactory, descriptor.ID, descriptor.Features.Skills)
	}
	if descriptor.Features.Skills == SkillsNone && len(descriptor.LegacySkillRoots) > 0 {
		return fmt.Errorf("%w: provider %q has skill roots but disables skills", ErrInvalidFactory, descriptor.ID)
	}
	for _, root := range descriptor.LegacySkillRoots {
		if strings.TrimSpace(root) == "" {
			return fmt.Errorf("%w: provider %q has an empty legacy skill root", ErrInvalidFactory, descriptor.ID)
		}
	}
	return nil
}

func validateScopes(descriptor Descriptor, profile *provisioning.Profile) error {
	if len(descriptor.ExecutionScopes) == 0 {
		return fmt.Errorf("%w: provider %q has no execution scope", ErrInvalidFactory, descriptor.ID)
	}
	seen := make(map[ExecutionScope]bool, len(descriptor.ExecutionScopes))
	projectScoped := false
	for _, scope := range descriptor.ExecutionScopes {
		if scope != ScopeHost && scope != ScopeProject {
			return fmt.Errorf("%w: provider %q has unknown execution scope %q", ErrInvalidFactory, descriptor.ID, scope)
		}
		if seen[scope] {
			return fmt.Errorf("%w: provider %q repeats execution scope %q", ErrInvalidFactory, descriptor.ID, scope)
		}
		seen[scope] = true
		projectScoped = projectScoped || scope == ScopeProject
	}
	if descriptor.Default && !seen[ScopeHost] {
		return fmt.Errorf("%w: default provider %q does not support host execution", ErrInvalidFactory, descriptor.ID)
	}
	if projectScoped && profile == nil {
		return fmt.Errorf("%w: project provider %q has no profile", ErrInvalidFactory, descriptor.ID)
	}
	if profile != nil {
		return validateProfile(descriptor.ID, profile, projectScoped)
	}
	return nil
}

func validateProfile(id agent.ProviderID, profile *provisioning.Profile, projectScoped bool) error {
	if profile.ID != string(id) {
		return fmt.Errorf("%w: provider %q has profile %q", ErrInvalidFactory, id, profile.ID)
	}
	cli := profile.CLI
	if strings.TrimSpace(cli.Name) == "" || strings.TrimSpace(cli.Binary) == "" || strings.TrimSpace(cli.Version) == "" {
		return fmt.Errorf("%w: provider %q has incomplete CLI policy", ErrInvalidFactory, id)
	}
	if !provisioning.ValidCLIVersion(cli.Version) {
		return fmt.Errorf("%w: provider %q has invalid CLI version %q", ErrInvalidFactory, id, cli.Version)
	}
	if cli.InstallTimeout <= 0 {
		return fmt.Errorf("%w: provider %q has a non-positive CLI install timeout", ErrInvalidFactory, id)
	}
	if cli.WaitTimeout <= 0 {
		return fmt.Errorf("%w: provider %q has a non-positive CLI wait timeout", ErrInvalidFactory, id)
	}
	if (cli.CheckVersion || cli.ReportVersion) && len(cli.VersionArgs) == 0 {
		return fmt.Errorf("%w: provider %q has no CLI version arguments", ErrInvalidFactory, id)
	}
	for _, argument := range cli.VersionArgs {
		if strings.TrimSpace(argument) == "" || strings.ContainsRune(argument, '\x00') {
			return fmt.Errorf("%w: provider %q has an invalid CLI version argument", ErrInvalidFactory, id)
		}
	}
	if projectScoped && strings.TrimSpace(cli.ImageLabel) == "" {
		return fmt.Errorf("%w: project provider %q has no image label", ErrInvalidFactory, id)
	}
	switch cli.InstallMode {
	case provisioning.InstallWithNPM, provisioning.InstallWithImageRepair:
		if strings.TrimSpace(cli.PackageName) == "" {
			return fmt.Errorf("%w: provider %q has no CLI package", ErrInvalidFactory, id)
		}
	case provisioning.InstallWithScript:
		if strings.TrimSpace(cli.InstallScript) == "" {
			return fmt.Errorf("%w: provider %q has no install script", ErrInvalidFactory, id)
		}
	default:
		return fmt.Errorf("%w: provider %q has unknown install mode %q", ErrInvalidFactory, id, cli.InstallMode)
	}
	seenDevices := make(map[string]bool, len(profile.PersistentState))
	seenHosts := make(map[string]bool, len(profile.PersistentState))
	seenTargets := make(map[string]bool, len(profile.PersistentState))
	for _, state := range profile.PersistentState {
		if err := state.Validate(); err != nil {
			return fmt.Errorf("%w: provider %q has invalid persistent state: %v", ErrInvalidFactory, id, err)
		}
		if seenDevices[state.Device] || seenHosts[state.HostDirectory] || seenTargets[state.ContainerPath] {
			return fmt.Errorf("%w: provider %q repeats a persistent-state mount", ErrInvalidFactory, id)
		}
		seenDevices[state.Device] = true
		seenHosts[state.HostDirectory] = true
		seenTargets[state.ContainerPath] = true
	}
	return nil
}

func validateAuthMode(descriptor Descriptor) error {
	switch descriptor.Auth {
	case AuthManagedCode, AuthManagedDevice, AuthExternal:
		if strings.TrimSpace(descriptor.AuthInstructions) == "" {
			return fmt.Errorf("%w: provider %q auth has no instructions", ErrInvalidFactory, descriptor.ID)
		}
		if descriptor.Auth == AuthExternal && descriptor.SatisfiesAccessGate {
			return fmt.Errorf("%w: provider %q external auth cannot satisfy the access gate", ErrInvalidFactory, descriptor.ID)
		}
	case AuthNone:
		if strings.TrimSpace(descriptor.AuthInstructions) != "" {
			return fmt.Errorf("%w: provider %q has instructions for auth mode %q", ErrInvalidFactory, descriptor.ID, descriptor.Auth)
		}
	default:
		return fmt.Errorf("%w: provider %q has unknown auth mode %q", ErrInvalidFactory, descriptor.ID, descriptor.Auth)
	}
	return nil
}

func validateAuth(descriptor Descriptor, binding *agentauth.Binding) error {
	if descriptor.Auth == AuthNone {
		if binding != nil {
			return fmt.Errorf("%w: provider %q declares no auth but built a binding", ErrInvalidFactory, descriptor.ID)
		}
		return nil
	}
	if binding == nil {
		return fmt.Errorf("%w: provider %q did not build an auth binding", ErrInvalidFactory, descriptor.ID)
	}
	if binding.ID() != descriptor.ID {
		return fmt.Errorf("%w: provider %q built auth binding %q", ErrInvalidFactory, descriptor.ID, binding.ID())
	}
	wantFlow := map[AuthMode]agentauth.Flow{
		AuthManagedCode:   agentauth.FlowCode,
		AuthManagedDevice: agentauth.FlowDevice,
		AuthExternal:      agentauth.FlowExternal,
	}[descriptor.Auth]
	if binding.Flow() != wantFlow {
		return fmt.Errorf("%w: provider %q auth flow is %q, want %q", ErrInvalidFactory, descriptor.ID, binding.Flow(), wantFlow)
	}
	if descriptor.Auth == AuthExternal && binding.Available() {
		return fmt.Errorf("%w: provider %q external auth exposes a managed service", ErrInvalidFactory, descriptor.ID)
	}
	if descriptor.Auth != AuthExternal && !binding.Available() {
		return fmt.Errorf("%w: provider %q managed auth is unavailable", ErrInvalidFactory, descriptor.ID)
	}
	return nil
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	descriptor.ExecutionScopes = append([]ExecutionScope(nil), descriptor.ExecutionScopes...)
	descriptor.LegacySkillRoots = append([]string(nil), descriptor.LegacySkillRoots...)
	return descriptor
}

func cloneProfile(profile *provisioning.Profile) *provisioning.Profile {
	if profile == nil {
		return nil
	}
	cloned := profile.Clone()
	return &cloned
}
