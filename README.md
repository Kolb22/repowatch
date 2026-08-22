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

```bash
git clone https://github.com/Kolb22/repowatch.git
cd repowatch
go build -o repowatch ./cmd/repowatch
sudo install -m 0755 repowatch /usr/local/bin/repowatch
```

## Usage

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

## Safety

RepoWatch never runs destructive Git commands automatically. It does not reset, clean, stash, merge, rebase, or force-pull.

When the local repository is dirty, ahead, or diverged, RepoWatch stops and returns a non-zero exit code.

## Polling With systemd

The recommended deployment setup is:

```text
systemd timer (every 30 seconds)
    -> repowatch-my-app.service
    -> repowatch sync
```

The Go CLI remains a one-shot command. `systemd` owns scheduling and process supervision, while RepoWatch owns safe Git synchronization. No inbound VM access or GitHub Actions secrets are required.

The Linux VM should have:

- Git installed
- RepoWatch installed at `/usr/local/bin/repowatch`
- the target repository already cloned
- Git authentication configured through SSH keys or normal Git credentials
- the example service and timer from `examples/systemd/`

Install the units:

```bash
sudo cp examples/systemd/repowatch-my-app.service /etc/systemd/system/
sudo cp examples/systemd/repowatch-my-app.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now repowatch-my-app.timer
systemctl list-timers repowatch-my-app.timer
```

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
```

## Limitations

RepoWatch currently supports one repository per invocation. It does not run as a daemon, receive webhooks, deploy applications, execute post-sync commands, or manage rollback.

