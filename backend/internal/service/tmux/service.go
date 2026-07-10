package tmux

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidName     = errors.New("invalid tmux session name")
	ErrSessionExists   = errors.New("tmux session exists")
	ErrSessionNotFound = errors.New("tmux session not found")
	ErrCwdUnavailable  = errors.New("could not resolve session cwd")
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

type Session struct {
	Name     string `json:"name"`
	Created  int64  `json:"created"`
	Attached bool   `json:"attached"`
	Windows  int    `json:"windows"`
	Cwd      string `json:"cwd"`
}

type Terminal interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Resize(cols, rows uint16) error
	Close() error
}

type Client interface {
	List() []Session
	Create(name string) error
	Kill(name string) error
	Has(name string) bool
	Cwd(session string) (string, error)
	SendText(session, text string, pressEnter bool) error
	Attach(session string) (Terminal, error)
}

type Service struct {
	client Client
}

func New(client Client) *Service {
	return &Service{client: client}
}

func ValidName(name string) bool {
	return validName.MatchString(name)
}

func (s *Service) ValidName(name string) bool {
	return ValidName(name)
}

func (s *Service) List() []Session {
	return s.client.List()
}

func (s *Service) Create(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !ValidName(name) {
		return "", ErrInvalidName
	}
	if s.client.Has(name) {
		return "", ErrSessionExists
	}
	return name, s.client.Create(name)
}

func (s *Service) Delete(name string) error {
	if !ValidName(name) {
		return ErrInvalidName
	}
	if !s.client.Has(name) {
		return ErrSessionNotFound
	}
	return s.client.Kill(name)
}

func (s *Service) Cwd(session string) (string, error) {
	return s.client.Cwd(session)
}

func (s *Service) UploadTarget(name string) (string, error) {
	if !ValidName(name) {
		return "", ErrInvalidName
	}
	if !s.client.Has(name) {
		return "", ErrSessionNotFound
	}
	cwd, err := s.client.Cwd(name)
	if err != nil || cwd == "" {
		return "", ErrCwdUnavailable
	}
	return cwd, nil
}

func (s *Service) SendText(name, text string, pressEnter bool) error {
	if !ValidName(name) {
		return ErrInvalidName
	}
	if !s.client.Has(name) {
		return ErrSessionNotFound
	}
	return s.client.SendText(name, text, pressEnter)
}

func (s *Service) Attach(name string) (Terminal, error) {
	if !ValidName(name) {
		return nil, ErrInvalidName
	}
	if !s.client.Has(name) {
		if err := s.client.Create(name); err != nil {
			return nil, err
		}
	}
	return s.client.Attach(name)
}
