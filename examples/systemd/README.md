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
sudo systemctl daemon-reload
sudo systemctl start repowatch-my-app.service
sudo journalctl -u repowatch-my-app.service -n 50
```

GitHub Actions can start the service over SSH:

```bash
sudo systemctl start repowatch-my-app.service
```

