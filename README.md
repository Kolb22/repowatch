# RepoWatch

RepoWatch is a small Go CLI that safely synchronizes a local Git repository with a configured remote branch.

It is designed for a simple workflow:

```text
develop locally
push to GitHub
systemd timer runs on the Linux VM
RepoWatch checks the configured Git remote
RepoWatch fast-forwards the local repository when safe
```

RepoWatch does not replace GitHub Actions, Argo CD, Flux, or Kubernetes git-sync.

Its purpose is intentionally narrow: manage local Git synchronization on Linux while refusing unsafe updates.

## Features

* Safe Git synchronization using fast-forward only
* Detects dirty working trees
* Detects local commits ahead of the remote
* Detects diverged histories
* Verifies the configured branch is currently checked out
* Refuses synchronization on detached HEAD
* Updates only to the exact remote commit that was inspected
* systemd timer installation and management
* Multiple independent repository installations
* Precompiled Linux binaries for AMD64 and ARM64
* SHA-256 release checksums
* Dedicated exit codes for automation and monitoring

## Install

The Linux machine needs:

* Git
* systemd
* The target repository already cloned
* Working Git authentication for the configured remote

### Download the latest release

Debian and Ubuntu can detect the current architecture automatically:

```bash
cd /tmp

ARCH=$(dpkg --print-architecture)
BINARY=repowatch-linux-${ARCH}

curl -fLO "https://github.com/Kolb22/repowatch/releases/latest/download/${BINARY}"
curl -fLO "https://github.com/Kolb22/repowatch/releases/latest/download/SHA256SUMS"

grep " ${BINARY}$" SHA256SUMS | sha256sum --check

sudo install -m 0755 "${BINARY}" /usr/local/bin/repowatch
```

Verify the installation:

```bash
repowatch --help
```

### Build from source

Building from source requires Go 1.27 or newer.

```bash
git clone https://github.com/Kolb22/repowatch.git
cd repowatch

go build -o repowatch ./cmd/repowatch

sudo install -m 0755 repowatch /usr/local/bin/repowatch
```

## Configure automatic synchronization

Install a systemd service and timer:

```bash
sudo repowatch install \
  --repo /opt/my-app \
  --remote origin \
  --branch main \
  --interval 30s
```

This creates the required systemd units, reloads systemd, enables the timer, and starts polling.

The service runs as the user who invoked `sudo`. Use `--user` when the repository belongs to another Linux user.

Example:

```bash
sudo repowatch install \
  --repo /home/deploy/my-app \
  --branch main \
  --interval 30s \
  --user deploy
```

## Verify the systemd installation

Set the repository name:

```bash
REPO_NAME=my-app
```

Check the timer:

```bash
systemctl list-timers "repowatch-${REPO_NAME}.timer"
```

Run synchronization manually through systemd:

```bash
sudo systemctl start "repowatch-${REPO_NAME}.service"
```

Inspect its status:

```bash
sudo systemctl status "repowatch-${REPO_NAME}.service"
```

Inspect recent logs:

```bash
journalctl \
  -u "repowatch-${REPO_NAME}.service" \
  -n 30 \
  --no-pager
```

## Manage installations

RepoWatch can manage multiple repositories through independent systemd services and timers.

List managed repositories:

```bash
repowatch list
```

Example:

```text
NAME          SERVICE                         TIMER
my-app        repowatch-my-app.service        repowatch-my-app.timer
nodesentinel  repowatch-nodesentinel.service  repowatch-nodesentinel.timer
```

Remove one RepoWatch installation:

```bash
sudo repowatch uninstall my-app
```

This disables the associated timer and removes only:

```text
repowatch-my-app.service
repowatch-my-app.timer
```

The Git repository and its files are never removed.

## Manual synchronization

RepoWatch can also run without systemd:

```bash
repowatch sync \
  --repo /opt/my-app \
  --remote origin \
  --branch main
```

### Sync flags

| Flag        | Default  | Description                             |
| ----------- | -------- | --------------------------------------- |
| `--repo`    | required | Path to the local Git repository        |
| `--remote`  | `origin` | Git remote                              |
| `--branch`  | `main`   | Branch to synchronize                   |
| `--timeout` | `30s`    | Maximum time allowed for Git operations |
| `--quiet`   | `false`  | Suppress successful output              |

## Synchronization behavior

RepoWatch evaluates the repository before making any update.

```text
Validate repository
        |
        v
Verify current branch
        |
        v
Check working tree
        |
        v
Fetch configured remote branch
        |
        v
Compare local and remote commits
        |
        v
Determine Git history relationship
        |
        v
Fast-forward to inspected SHA
```

Possible states include:

```text
UP_TO_DATE
BEHIND
AHEAD
DIVERGED
DIRTY
WRONG_BRANCH
```

When the repository is safely behind the remote, RepoWatch updates it using:

```bash
git merge --ff-only <inspected-remote-sha>
```

The SHA used for the update is the exact commit that RepoWatch evaluated after fetching.

RepoWatch does not run another network fetch between the safety decision and the local fast-forward.

## Exit codes

| Code | Meaning                             |
| ---: | ----------------------------------- |
|  `0` | Up to date or successfully updated  |
|  `1` | Operational or internal error       |
|  `2` | Dirty working tree                  |
|  `3` | Local repository is ahead           |
|  `4` | Local and remote histories diverged |
|  `5` | Wrong branch or detached HEAD       |

These exit codes make RepoWatch suitable for systemd, scripts, monitoring, and other automation.

## Safety

RepoWatch is intentionally conservative.

Synchronization stops when:

* The working tree contains local changes
* The checked-out branch does not match the configured branch
* HEAD is detached
* The local repository contains commits not present remotely
* Local and remote histories have diverged

RepoWatch never automatically runs destructive Git recovery commands.

It does not use:

```text
git reset --hard
git clean
git stash
git rebase
git checkout --force
git pull --force
```

It also never discards local changes or creates merge commits automatically.

## Polling with systemd

The normal deployment model is:

```text
systemd timer
      |
      v
repowatch-<name>.service
      |
      v
repowatch sync
      |
      v
Git repository
```

RepoWatch remains a one-shot process.

systemd is responsible for:

* Scheduling
* Process lifecycle
* Logging
* Service supervision

RepoWatch is responsible for:

* Repository validation
* Git state inspection
* Safety decisions
* Fast-forward synchronization

No inbound VM access, webhook server, or GitHub Actions deployment secret is required.

## Architecture

```text
cmd/repowatch
    CLI parsing
    commands
    output
    exit codes

internal/git
    Git CLI wrapper
    process execution
    repository inspection

internal/sync
    synchronization state machine
    safety decisions

internal/systemd
    systemd unit generation
    installation
    listing
    removal
```

The project intentionally uses the Go standard library and delegates Git operations to the installed Git executable.

## Development

Run all local checks:

```bash
gofmt -w .
go vet ./...
go test ./...
go build ./...
```

GitHub Actions runs formatting validation, static analysis, tests, and builds automatically.

## Releases

Version tags trigger the release workflow.

Example:

```bash
git tag v0.1.2
git push origin v0.1.2
```

GitHub Actions builds:

```text
repowatch-linux-amd64
repowatch-linux-arm64
SHA256SUMS
```

and attaches them to the corresponding GitHub Release.

## Limitations

RepoWatch synchronizes one repository per service invocation.

Multiple repositories are supported by creating independent systemd installations.

RepoWatch currently does not:

* Run as a daemon
* Receive webhooks
* Deploy applications
* Execute post-sync commands
* Manage rollback
* Resolve merge conflicts
* Automatically recover unsafe Git states

These limitations are intentional. RepoWatch focuses only on safe repository synchronization.

## License

MIT

