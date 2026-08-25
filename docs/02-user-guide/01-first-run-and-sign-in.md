# First run and sign-in

Remote opens behind two gates:

1. A registered platform user must sign in.
2. At least one coding-agent provider must be connected.

The first person to open a new installation claims the local administrator
account and connects the first provider. Later users sign in with an invited
Google account.

## Claim a new server

Only do this on a server you are authorized to administer. The claim form is
public until the first administrator exists.

1. Open the Remote server URL.
2. On **Create your admin account**, enter an **Admin email**.
3. Enter a password of at least 12 characters in **Password**.
4. Enter it again in **Confirm password**.
5. Select **Create admin account**.

**Outcome:** Remote creates the server's one local administrator identity,
signs that administrator in, and moves to agent-provider setup.

The password is stored as a salted Argon2id hash. It cannot be displayed or
recovered, so store it in an appropriate password manager.

> A legacy installation may show **Secure your admin account** instead. Its
> existing administrator must sign in once and create the local password. Other
> users see a waiting screen until that is complete.

## Connect the first agent provider

The **Connect an AI provider** screen shows Claude, Codex, and Kimi. Connect at
least one to enter the workspace; the others can be connected later.

Antigravity is also available in project chats, but it does not use this
host-wide onboarding screen and does not satisfy the initial provider gate.
First connect Claude, Codex, or Kimi. You can then sign in to Antigravity from
the Terminal of each project where you want to use it, then choose **Refresh
models** in that project's chat picker.

![Agent authentication cards for Claude, Codex, and Kimi](/assets/docs/screenshots/03-agent-authentication-01m05s.webp)

### Claude

1. In **Claude authentication**, select **Sign in with Claude**.
2. Open the authorization link.
3. Complete the Anthropic sign-in.
4. Paste the returned value into **Paste your code here**.
5. Select **Submit code**.
6. Wait for **Subscription signed in**.

Use **Cancel** to abandon an in-progress authorization. Once connected, the
button changes to **Refresh Claude login**.

### Codex

1. In **Codex authentication**, select **Sign in with ChatGPT**.
2. In the device-login panel, copy the displayed code.
3. Select **Open**, sign in with ChatGPT, and enter the code.
4. Return to Remote and wait for **ChatGPT signed in**.

The code expires at the time shown in the panel. Start the flow again if it
expires. Remote expects ChatGPT subscription authentication; an API-key-only
Codex login is shown as **API-key login detected** and is not treated as the
desired subscription login.

### Kimi

1. In **Kimi authentication**, select **Sign in with Kimi**.
2. Copy the displayed device code.
3. Select **Open** and confirm the code with your Kimi account.
4. Return to Remote and wait for **Subscription signed in**.

**Outcome:** as soon as any provider is authenticated, the workspace opens.
The administrator can connect or refresh the other providers later through
**Settings** → **Agents**.

For Antigravity's separate per-project sign-in, see
[Use Antigravity](10-global-settings-users-providers.md#use-antigravity).

## Sign in after setup

### Local administrator

1. Open the Remote server URL.
2. On **Sign in**, enter the configured **Admin email** and **Password**.
3. Select **Sign in as administrator**.

### Invited user

1. Ask an administrator to register your email in **Settings** → **Users**.
2. Open the Remote server URL.
3. Select **Sign in with Google**.
4. Choose the exact Google account whose email was invited.

Google sign-in appears only after an administrator has configured Google OAuth.
There is no public self-registration. An unregistered account receives the
message that the account is not invited.

## Roles and availability

| Capability | Local or invited administrator | Invited member |
| --- | ---: | ---: |
| Sign in and use assigned projects | Yes | Yes |
| Access every project | Yes | No |
| Connect or refresh agent providers | Yes | No |
| Configure Google OAuth and global users | Yes | No |
| Change project resource limits | Yes | No |
| Delete a project | Yes | No |

If no provider is connected, administrators see the provider cards and members
see a waiting screen. A member cannot complete provider authentication.

## Important trust boundary

Provider credentials are host-wide, not personal to a user or project. An
administrator signs in once on the parent host, and Remote seeds those
credentials into project containers. Every authorized user who can run that
provider therefore consumes the same connected provider account and quota.

This shared-credential rule applies to the Claude, Codex, and Kimi onboarding
cards. Antigravity is different: its CLI sign-in is performed inside one
project and remains in that container's replaceable root filesystem.

Platform sessions use a secure, HTTP-only cookie and currently last 30 days.
There is no user-facing session-revocation screen; sign out with the **Sign
out** control in the sidebar footer when using a shared browser.

## Next steps

Continue with [Projects and the sidebar](02-projects-and-sidebar.md).

For the underlying access model, see
[Authentication, users, and access](../02-workspaces/02-auth-users-and-access.md).
For the complete trust analysis, see [Threat model](../threat-model.md).
