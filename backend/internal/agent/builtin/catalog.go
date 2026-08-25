// Package builtin is the explicit composition root for Remote's compiled-in
// agent integrations. Providers own their complete factory; this package owns
// only the reviewed registration order and has no construction details.
package builtin

import (
	antigravityagent "github.com/futrx-com/remote.futrx.com/internal/agent/antigravity"
	claudeagent "github.com/futrx-com/remote.futrx.com/internal/agent/claude"
	codexagent "github.com/futrx-com/remote.futrx.com/internal/agent/codex"
	kimiagent "github.com/futrx-com/remote.futrx.com/internal/agent/kimi"
	agentmodule "github.com/futrx-com/remote.futrx.com/internal/agent/module"
)

func Catalog() (*agentmodule.Catalog, error) {
	builders := []agentmodule.FactoryBuilder{
		claudeagent.Factory,
		codexagent.Factory,
		kimiagent.Factory,
		antigravityagent.Factory,
	}
	factories := make([]agentmodule.Factory, 0, len(builders))
	for _, build := range builders {
		factory, err := build()
		if err != nil {
			return nil, err
		}
		factories = append(factories, factory)
	}
	return agentmodule.NewCatalog(factories...)
}
