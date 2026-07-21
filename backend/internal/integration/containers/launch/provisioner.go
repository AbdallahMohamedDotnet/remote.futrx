// Package launch preserves the former integration import while launch
// orchestration moves to the application layer.
package launch

import servicelaunch "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/launch"

type RegisteredCredentialEnsurer = servicelaunch.RegisteredCredentialEnsurer
type BrowserProvisioner = servicelaunch.BrowserProvisioner
type WorkspaceProvisioner = servicelaunch.WorkspaceProvisioner
type CodeServerProvisioner = servicelaunch.CodeServerProvisioner
type Provisioner = servicelaunch.Provisioner

var NewProvisioner = servicelaunch.NewProvisioner
