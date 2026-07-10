package tmux

import (
	"errors"
	"reflect"
	"testing"
)

func TestServiceCreateValidatesAndDelegates(t *testing.T) {
	client := newFakeClient()
	service := New(client)

	name, err := service.Create(" demo ")
	if err != nil {
		t.Fatal(err)
	}
	if name != "demo" {
		t.Fatalf("name = %q", name)
	}
	if !client.sessions["demo"] {
		t.Fatal("session was not created")
	}

	if _, err := service.Create("bad/name"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("invalid name error = %v", err)
	}
	if _, err := service.Create("demo"); !errors.Is(err, ErrSessionExists) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestServiceUploadAndSendRequireExistingSession(t *testing.T) {
	client := newFakeClient()
	service := New(client)

	if _, err := service.UploadTarget("missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("upload missing error = %v", err)
	}
	if err := service.SendText("missing", "hello", true); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("send missing error = %v", err)
	}

	client.sessions["demo"] = true
	cwd, err := service.UploadTarget("demo")
	if err != nil {
		t.Fatal(err)
	}
	if cwd != "/workspace/demo" {
		t.Fatalf("cwd = %q", cwd)
	}
	if err := service.SendText("demo", "hello", false); err != nil {
		t.Fatal(err)
	}
	if got, want := client.sent, []string{"demo:hello:false"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
}

func TestServiceAttachCreatesMissingSession(t *testing.T) {
	client := newFakeClient()
	service := New(client)

	term, err := service.Attach("fresh")
	if err != nil {
		t.Fatal(err)
	}
	if term == nil {
		t.Fatal("expected terminal")
	}
	if !client.sessions["fresh"] {
		t.Fatal("session was not created")
	}
	if client.attached != "fresh" {
		t.Fatalf("attached = %q", client.attached)
	}
}

type fakeClient struct {
	sessions map[string]bool
	sent     []string
	attached string
}

func newFakeClient() *fakeClient {
	return &fakeClient{sessions: map[string]bool{}}
}

func (c *fakeClient) List() []Session {
	out := make([]Session, 0, len(c.sessions))
	for name := range c.sessions {
		out = append(out, Session{Name: name})
	}
	return out
}

func (c *fakeClient) Create(name string) error {
	c.sessions[name] = true
	return nil
}

func (c *fakeClient) Kill(name string) error {
	delete(c.sessions, name)
	return nil
}

func (c *fakeClient) Has(name string) bool {
	return c.sessions[name]
}

func (c *fakeClient) Cwd(session string) (string, error) {
	if !c.sessions[session] {
		return "", errors.New("not found")
	}
	return "/workspace/" + session, nil
}

func (c *fakeClient) SendText(session, text string, pressEnter bool) error {
	c.sent = append(c.sent, session+":"+text+":"+boolString(pressEnter))
	return nil
}

func (c *fakeClient) Attach(session string) (Terminal, error) {
	c.attached = session
	return fakeTerminal{}, nil
}

type fakeTerminal struct{}

func (fakeTerminal) Read([]byte) (int, error) {
	return 0, nil
}

func (fakeTerminal) Write(p []byte) (int, error) {
	return len(p), nil
}

func (fakeTerminal) Resize(cols, rows uint16) error {
	return nil
}

func (fakeTerminal) Close() error {
	return nil
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
