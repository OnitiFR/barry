# Barry
## Send your backups into the clouds

This client-server tool will watch a local directory for new backups, and
upload them to a cold-storage Swift ("OVH-flavored") service.

This is a work-in-progress, see the TODO file.

### Install

Make sure Go is installed, then install/update Barry:
`go get -u github.com/OnitiFR/cmd/...`

Copy and modify `install/barry.service` to your `/etc/systemd/system`, 
then reload systemd with `systemctl daemon-reload`.

You can configure alerts using samples in `etc/alerts` directory. Install `jq` utility
if you want the use the sample `slack.sh` alert.

You can now manage the service (ex: `systemctl start barry`).

### Development

Use the `dev` tag for (extremely) reduced timing : 
- `go run -tags dev . -trace -pretty`
- `go build -tags dev`
- …
