# systemd Example

Install RepoWatch:

```bash
go build -o repowatch ./cmd/repowatch
sudo install -m 0755 repowatch /usr/local/bin/repowatch
```

Create a dedicated user if needed:

```bash
sudo useradd --system --create-home --shell /usr/sbin/nologin repowatch
```

Clone the application repository on the VM:

```bash
sudo mkdir -p /opt/my-app
sudo chown repowatch:repowatch /opt/my-app
sudo -u repowatch git clone git@github.com:OWNER/my-app.git /opt/my-app
```

Install the service:

```bash
sudo cp examples/systemd/repowatch-my-app.service /etc/systemd/system/
sudo cp examples/systemd/repowatch-my-app.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now repowatch-my-app.timer
systemctl list-timers repowatch-my-app.timer
sudo journalctl -u repowatch-my-app.service -n 50
```

Run a synchronization immediately when troubleshooting:

```bash
sudo systemctl start repowatch-my-app.service
```

The timer checks the remote every 30 seconds. Change `OnUnitActiveSec` in the timer if a different interval is more appropriate.

