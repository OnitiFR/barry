# Barry
## Send your backups into the clouds

This client-server tool will watch a local directory for new backups, and
upload them to a cold-storage Swift ("OVH-flavored") service.

This is a work-in-progress, see the TODO file.

### Install

Make sure Go is installed, then install/update Barry:
`go get -u github.com/OnitiFR/cmd/...`

Copy and modify `install/barryd.service` to your `/etc/systemd/system`, 
then reload systemd with `systemctl daemon-reload`.

You can configure alerts using samples in `etc/alerts` directory. Install `jq` utility
if you want the use the sample `slack.sh` alert.

You can now manage the service (ex: `systemctl start barryd`).

### Development

For the server, use the `dev` tag for (extremely) reduced timings, examples :
- `go run -tags dev ./cmd/barryd/ -trace -pretty`
- `go build -tags dev ./cmd/barryd/`
- …
