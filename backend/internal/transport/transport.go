package transport

import (
	"io/fs"
	"net/http"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/claudelogin"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/runhub"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/prompt"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
	httphandlers "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http/handlers"
	wstransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/ws"
)

type TmuxClient interface {
	httphandlers.TmuxClient
	wstransport.TmuxSessionClient
	prompt.TmuxClient
}

type Dependencies struct {
	ChatStore        servicechat.Repository
	ChatService      *servicechat.Service
	ProjectService   *serviceproject.Service
	TmuxClient       TmuxClient
	RunHub           *runhub.Hub
	ContainerManager prompt.ContainerPreparer
	Auth             httptransport.AuthRegistrar
	Static           fs.FS
}

func NewHTTPHandler(deps Dependencies) http.Handler {
	promptService := prompt.New(
		deps.ChatStore,
		deps.TmuxClient,
		deps.ProjectService,
		deps.ContainerManager,
		deps.RunHub,
	)

	return httptransport.NewHandler(httptransport.Handlers{
		Sessions:   httphandlers.NewTmuxHandler(deps.TmuxClient),
		Chats:      httphandlers.NewChatHandler(deps.ChatService),
		Projects:   httphandlers.NewProjectHandler(deps.ProjectService),
		ClaudeAuth: httphandlers.NewClaudeAuthHandler(claudelogin.New()),
		TmuxWS:     wstransport.NewTmuxSocket(deps.TmuxClient),
		ChatWS:     wstransport.NewChatSocket(deps.ChatStore, deps.RunHub, promptService),
		Auth:       deps.Auth,
		Static:     httptransport.NewStaticHandler(deps.Static),
	})
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return httptransport.NewServer(addr, handler)
}
