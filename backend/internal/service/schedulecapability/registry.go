package schedulecapability

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	serviceproject "github.com/futrx-com/remote.futrx.com/internal/service/project"
	serviceprompt "github.com/futrx-com/remote.futrx.com/internal/service/prompt"
)

var ErrInvalidCapability = errors.New("invalid or expired schedule capability")

type Scope string

const (
	ScopeManage       Scope = "manage"
	ScopeCompleteSelf Scope = "complete-self"
)

type Grant struct {
	OwnerEmail      string
	IsAdmin         bool
	ChatID          servicechat.ID
	ProjectID       serviceproject.ID
	ScheduledTaskID string
	ScheduledRunID  string
	Scope           Scope
	ExpiresAt       time.Time
}

type Registry struct {
	mu     sync.Mutex
	apiURL string
	ttl    time.Duration
	now    func() time.Time
	grants map[string]Grant
}

func New(baseURL string) *Registry {
	return &Registry{
		apiURL: strings.TrimRight(baseURL, "/") + "/agent-api/schedules",
		ttl:    4 * time.Hour,
		now:    time.Now,
		grants: map[string]Grant{},
	}
}

func (r *Registry) IssueScheduleTool(
	_ context.Context,
	request serviceprompt.ScheduleToolRequest,
) (serviceprompt.ScheduleToolAccess, error) {
	email := strings.ToLower(strings.TrimSpace(request.Actor.Email))
	if email == "" || request.ChatID == "" || request.ProjectID == "" {
		return serviceprompt.ScheduleToolAccess{}, errors.New("schedule capability requires an authenticated project chat")
	}
	if (request.ScheduledTaskID == "") != (request.ScheduledRunID == "") {
		return serviceprompt.ScheduleToolAccess{}, errors.New("scheduled completion capability requires both task and run IDs")
	}
	token, err := newToken()
	if err != nil {
		return serviceprompt.ScheduleToolAccess{}, err
	}
	scope := ScopeManage
	if request.ScheduledTaskID != "" {
		scope = ScopeCompleteSelf
	}
	r.mu.Lock()
	r.deleteExpiredLocked()
	r.grants[token] = Grant{
		OwnerEmail:      email,
		IsAdmin:         request.Actor.IsAdmin,
		ChatID:          request.ChatID,
		ProjectID:       request.ProjectID,
		ScheduledTaskID: request.ScheduledTaskID,
		ScheduledRunID:  request.ScheduledRunID,
		Scope:           scope,
		ExpiresAt:       r.now().Add(r.ttl),
	}
	r.mu.Unlock()

	return serviceprompt.ScheduleToolAccess{
		APIURL: r.apiURL,
		Token:  token,
		Revoke: func() {
			r.Revoke(token)
		},
	}, nil
}

func (r *Registry) Resolve(token string) (Grant, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Grant{}, ErrInvalidCapability
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleteExpiredLocked()
	grant, ok := r.grants[token]
	if !ok {
		return Grant{}, ErrInvalidCapability
	}
	return grant, nil
}

func (r *Registry) Revoke(token string) {
	r.mu.Lock()
	delete(r.grants, token)
	r.mu.Unlock()
}

func (r *Registry) deleteExpiredLocked() {
	now := r.now()
	for token, grant := range r.grants {
		if !grant.ExpiresAt.After(now) {
			delete(r.grants, token)
		}
	}
}

func newToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
