# Security policy

remote.futrx runs AI agents with root privileges inside per-project containers and manages provider credentials and project secrets on a self-hosted box. We take its security seriously and appreciate reports that help us improve it.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report privately to **contact@futrx.com**. If you can, use the subject line `SECURITY: <short description>` so it routes quickly.

Include as much as you can:

- The component and version/commit affected (`git rev-parse HEAD` if you're on a checkout).
- A description of the issue and its impact — what an attacker gains, and the preconditions (e.g. "requires an invited non-admin account").
- Steps to reproduce, or a proof of concept.
- Any suggested remediation.

If you need to send sensitive details, say so in your first message and we'll arrange an encrypted channel.

### What to expect

- **Acknowledgement** within 5 business days.
- An initial **assessment and severity** within 10 business days.
- Progress updates as we work on a fix, and credit in the release notes when the fix ships (unless you prefer to remain anonymous).

We ask that you give us a reasonable window to remediate before any public disclosure, and that your testing does not harm other users or exfiltrate data beyond what's needed to demonstrate the issue. We will not pursue action against good-faith research that follows this policy.

## Supported versions

remote.futrx is distributed as source and deployed from the tip of `main`; the installer and updater track `origin/main` (see [ARCHITECTURE.md](ARCHITECTURE.md) and [README.md](README.md)). There are no long-term support branches. **Security fixes land on `main`**, so the supported version is always the latest `main`. Keep your deployment current with:

```bash
sudo bash /opt/remote.futrx/infra/update.sh
```

## Scope

In scope — the code in this repository:

- The Go backend (`backend/`) — HTTP/WebSocket transport, services, file stores, integrations.
- The Preact frontend (`frontend/`).
- The installer, updater, and templates (`infra/`) and the CI workflows (`.github/`).
- The base-image build and agent-provisioning recipes.

Out of scope — please report these to their respective projects, not here:

- Vulnerabilities in dependencies we don't control (LXD, Caddy, the Linux kernel, Node, Go, and the Claude/Codex/Kimi agent CLIs). If a dependency issue is exploitable *specifically because of how remote.futrx configures or uses it*, that configuration is in scope — tell us.
- Issues that require pre-existing host root or physical access to the server.
- The single-server, no-HA, no-backup design and other documented constraints in [known-limitations.md](docs/known-limitations.md) — these are known trade-offs, though concrete exploits that go beyond them are welcome.

## Known security-relevant design facts

Before reporting, please review the [**threat model**](docs/threat-model.md). Several sharp edges are already documented there and in [known-limitations.md](docs/known-limitations.md), including: agents run as root inside containers with approvals disabled; the backend runs as root on the host; provider credentials are host-wide singletons; project secrets are plaintext at rest and readable by any project member; the per-project IDE host class does not enforce project membership; and the container bridge is not segmented by default. A report that these *exist* tells us what we already know — a report that shows a **new** way to exploit them, or an issue not yet listed there, is exactly what we want.

## Operator hardening checklist

If you run remote.futrx, these steps reduce your exposure to the issues in the threat model:

- **Only invite people you trust.** "Any registered user" can reach powerful surfaces (host shells via loose chats and the tmux socket, any project's IDE). Treat an invitation as granting substantial access to the box.
- **Restrict network access to the admin surfaces** where you can (e.g. IP-allowlist the main host at the firewall or a VPN), since several privileged paths are gated only by "registered user."
- **Back up `DATA_DIR` and root's home encrypted**, and keep them off the box. They contain the session key, OAuth secret, admin hash, and all project secrets and provider tokens.
- **Keep the server updated** so security fixes on `main` reach you.
- **Watch host disk and resource use** — a single workspace can exhaust disk (no default quota).
- **Rotate provider and project credentials** if you suspect a container was compromised, since tokens are shared host-wide.

Reporting anything that lets an attacker bypass these mitigations is very much in scope.
