# Global settings, users, and providers

Open **Settings** from the account footer or the collapsed sidebar gear. The page has four tabs: **Appearance**, **Agents**, **Users**, and **Info**.

## Appearance

Choose how Remote looks on the current device:

- **System** follows the browser or operating-system preference.
- **Dark** always uses the dark theme.
- **Light** always uses the light theme.

The preference is saved to your Remote user settings. Wait for the **Saved** state before leaving if the network is slow.

## Agents

Agent authentication is host-wide and administrator-managed. Sign in once on the parent host; Remote then seeds the provider credentials into project containers.

![Administrator view of Claude, Codex, and Kimi authentication](/assets/docs/screenshots/03-agent-authentication-01m05s.webp)

### Connect Claude

1. Open **Settings → Agents**.
2. Under **Claude authentication**, choose **Sign in with Claude**.
3. Open the displayed Anthropic authorization link.
4. Complete sign-in in the new tab.
5. Paste the returned code into Remote.
6. Choose **Submit code**.
7. Wait for **Subscription signed in**.

Use **Refresh Claude login** to replace an expired or unwanted host credential.

### Connect Codex

1. Under **Codex authentication**, choose the device-login action.
2. Open the verification URL.
3. Enter the displayed device code.
4. Approve the OpenAI account.
5. Return to Remote and wait for the connected state.

Remote may also detect a configured API key, but subscription/device authentication is the intended shared-host flow.

### Connect Kimi

1. Under **Kimi authentication**, start device login.
2. Open the verification URL.
3. Enter the displayed code.
4. Approve the account.
5. Return to Remote and wait for the connected state.

### Shared-provider implications

- Every user and project shares the same host provider accounts and their quotas.
- Provider credentials are copied into project credential locations.
- An agent that can read its project credential files can act with that provider authority.
- Claude, Codex, and Kimi homes are durable but separate by provider format, not separate security principals.
- Re-authentication can affect every project.

Non-admins can use connected providers but cannot connect or refresh them.

## Users

Remote has one local password administrator. Additional users sign in through Google OAuth and must be registered before they can enter.

### Configure Google OAuth

Administrator steps:

1. Create an OAuth web client in Google Cloud.
2. Add the callback URL shown in **Settings → Users**.
3. Copy the Google client ID and client secret into Remote.
4. Save and confirm that Google sign-in is enabled.

The Google client secret is stored on the host in plaintext with restrictive file permissions.

> **Current authentication caveat:** Remote authorizes Google users by normalized email, does not check Google's `verified_email` claim, does not bind invitations to the immutable Google `sub`, and does not enforce a hosted-domain (`hd`) restriction. Treat invitations on custom domains with care and read the [threat model](../threat-model.md) before enabling multi-user access.

### Add a user

1. Open **Settings → Users**.
2. Enter the user's exact Google-account email.
3. Choose **member** or **admin**.
4. Choose **Add**.
5. Add a member to specific projects through each project's **Sharing** tab.

There is no public sign-up. A successful Google identity that is not in the user directory is denied.

### Change a role

Administrators can promote a member to admin or demote an invited admin to member.

- An **admin** can manage global providers, Google OAuth, users, all projects, resource limits, and deletion.
- A **member** sees only projects where their email is a member, plus any loose chats.

The local administrator cannot be removed or demoted. Remote also prevents removal or demotion of the final administrator.

### Remove a user

1. Find the user under **Users**.
2. Choose the remove action.
3. Confirm that access should end.

Removal blocks future authenticated requests for that email. Sessions are stateless 30-day tokens and have no individual revocation UI; deleting a user is the practical access-control lever.

## Info

Use **Info** to inspect the parent host rather than one project container.

The page reports:

- current account email and role;
- server URL and collection time;
- CPU model, usage, load, and core counts;
- total, used, available, cached memory, and swap;
- filesystem mounts, capacity, used space, and free space;
- interfaces, addresses, and traffic counters;
- operating system, kernel, architecture, Go version, and uptime;
- backend PID, goroutines, file handles, heap, and system memory;
- configured application and storage paths.

Choose **Refresh** for a new point-in-time sample. These readings are operational observations, not performance guarantees.

## Sign out

Use the sign-out control in the account footer. This clears the platform session cookie from the browser. It does not disconnect host-wide agent-provider accounts.

## Access summary

| Settings action | Admin | Member |
| --- | ---: | ---: |
| Change own appearance | Yes | Yes |
| View own account and server information | Yes | Yes |
| Connect or refresh agent providers | Yes | No |
| Configure Google OAuth | Yes | No |
| Add, remove, promote, or demote users | Yes | No |

For the full sign-in state machine and proxy checks, see [Authentication, users, and access](../02-workspaces/02-auth-users-and-access.md).
