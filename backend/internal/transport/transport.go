package transport

import (
	"io/fs"
	"net/http"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/claudelogin"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/codexauth"
	service "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
	httphandlers "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http/handlers"
	wstransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/ws"
)

type TmuxClient interface {
	httphandlers.TmuxClient
	wstransport.TmuxSessionClient
}

type Dependencies struct {
	Services   service.Services
	TmuxClient TmuxClient
	Static     fs.FS
	DataDir    string
}

func NewHTTPHandler(deps Dependencies) (http.Handler, error) {
	var auth httptransport.AuthRegistrar
	if deps.Services.Auth != nil {
		auth = httphandlers.NewAuthHandler(deps.Services.Auth)
	}

	uploads, err := httptransport.NewUploadHandler(deps.Services.Chats, deps.DataDir)
	if err != nil {
		return nil, err
	}

	return httptransport.NewHandler(httptransport.Handlers{
		Sessions:   httphandlers.NewTmuxHandler(deps.TmuxClient),
		Chats:      httphandlers.NewChatHandler(deps.Services.Chats),
		Projects:   httphandlers.NewProjectHandler(deps.Services.Projects),
		ClaudeAuth: httphandlers.NewClaudeAuthHandler(claudelogin.New()),
		CodexAuth:  httphandlers.NewCodexAuthHandler(codexauth.New()),
		UserSettings: httphandlers.NewUserSettingsHandler(
			deps.Services.UserSettings,
			deps.Services.Auth,
		),
		Uploads:    uploads,
		TmuxWS:     wstransport.NewTmuxSocket(deps.TmuxClient),
		TerminalWS: wstransport.NewContainerTerminalSocket(deps.Services.Chats, deps.Services.Projects),
		ChatWS:     wstransport.NewChatSocket(deps.Services.Chats, deps.Services.Runs, deps.Services.Prompt),
		WorkspaceWS: wstransport.NewWorkspaceSocket(
			deps.Services.Chats,
			deps.Services.Projects,
			deps.Services.Workspace,
		),
		Auth:   auth,
		Static: httptransport.NewStaticHandler(deps.Static),
	}), nil
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return httptransport.NewServer(addr, handler)
}
