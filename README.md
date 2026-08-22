# RepoWatch

RepoWatch is a small Go CLI that safely synchronizes a local Git repository with a configured remote branch.

It is built for a common workflow:

```text
develop on Windows
push to GitHub
systemd timer runs on the Linux VM
RepoWatch checks the configured Git remote
RepoWatch fast-forwards the local repo when safe
```

RepoWatch does not replace GitHub Actions, Argo CD, Flux, or Kubernetes git-sync. Its job is intentionally narrow: inspect one local repository and fast-forward it only when the repository is clean and the remote history is safe.

## Install

The VM needs Git, systemd, and the target repository already cloned with working Git authentication.

### Download a release

Set the release version. Debian detects the architecture automatically (`amd64` for `x86_64`, `arm64` for `aarch64`):

```bash
cd /tmp

VERSION=v0.1.1
ARCH=$(dpkg --print-architecture)
BINARY=repowatch-linux-${ARCH}

curl -fLO "https://github.com/Kolb22/repowatch/releases/download/${VERSION}/${BINARY}"
curl -fLO "https://github.com/Kolb22/repowatch/releases/download/${VERSION}/SHA256SUMS"

grep " ${BINARY}$" SHA256SUMS | sha256sum --check

sudo install -m 0755 "${BINARY}" /usr/local/bin/repowatch
```

### Build from source

This option requires Go 1.27 or newer.

```bash
git clone https://github.com/Kolb22/repowatch.git
cd repowatch
go build -o repowatch ./cmd/repowatch
sudo install -m 0755 repowatch /usr/local/bin/repowatch
```

Install and enable polling:

```bash
sudo repowatch install \
  --repo /opt/my-app \
  --remote origin \
  --branch main \
  --interval 30s
```

This writes the systemd units, reloads systemd, enables and starts the timer, and prints its status. The service runs as the user who invoked `sudo`; use `--user` when the repository belongs to another Linux user.

### Verify the installation

Set `REPO_NAME` to the target repository directory name (for example, `nodesentinel`):

```bash
REPO_NAME=my-app

systemctl list-timers "repowatch-${REPO_NAME}.timer"
sudo systemctl start "repowatch-${REPO_NAME}.service"
sudo systemctl status "repowatch-${REPO_NAME}.service"
journalctl -u "repowatch-${REPO_NAME}.service" -n 30 --no-pager
```

## Manual Sync

```bash
repowatch sync \
  --repo /opt/my-app \
  --remote origin \
  --branch main
```

Flags:

| Flag | Default | Description |
|---|---|---|
| `--repo` | required | Path to the local Git repository |
| `--remote` | `origin` | Git remote |
| `--branch` | `main` | Remote branch |
| `--timeout` | `30s` | Maximum time allowed for Git operations |
| `--quiet` | `false` | Suppress successful output |

## Exit Codes

| Code | Meaning |
|---:|---|
| 0 | Up to date or successfully updated |
| 1 | Operational or internal error |
| 2 | Dirty working tree |
| 3 | Local repository is ahead |
| 4 | Local and remote histories diverged |
| 5 | Wrong branch or detached HEAD |

## Safety

RepoWatch verifies that the configured branch is checked out before fetching. A different branch or detached HEAD is refused.

After fetching, RepoWatch evaluates the remote commit and fast-forwards only to that exact SHA with `git merge --ff-only`. It never runs destructive Git recovery automatically: no reset, clean, stash, rebase, force checkout, or force pull.

When the local repository is dirty, ahead, diverged, or on the wrong branch, RepoWatch stops and returns a non-zero exit code.

## Polling With systemd

The recommended deployment setup is:

```text
systemd timer (every 30 seconds)
    -> repowatch-my-app.service
    -> repowatch sync
```

The Go CLI remains a one-shot command. `systemd` owns scheduling and process supervision, while RepoWatch owns safe Git synchronization. No inbound VM access or GitHub Actions secrets are required.

## Development

```bash
gofmt -w .
go vet ./...
go test ./...
go build ./...
```

## Architecture

```text
cmd/repowatch
    CLI parsing, output, exit codes

internal/git
    process runner and Git CLI wrapper

internal/sync
    synchronization state machine

internal/systemd
    polling unit generation and installation
```

## Limitations

RepoWatch currently supports one repository per invocation. It does not run as a daemon, receive webhooks, deploy applications, execute post-sync commands, or manage rollback.

