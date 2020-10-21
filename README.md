# Barry
## Send your backups into the clouds

This client-server tool will watch a local directory for new backups, and
upload them to a cold-storage Swift ("OVH-flavored") service.

### Install

Make sure Go is installed, then install/update Barry: `go get -u github.com/OnitiFR/barry`

Copy and modify `install/barry.service` to your `/etc/systemd/system`, 
then reload systemd with `systemctl daemon-reload`.

You can now manage the service (ex: `systemctl start barry`).

### Development

Use the `dev` tag for (extremly) reduced timing : 
- `go run -tags dev . -trace -pretty`
- `go build -tags dev`
- …
