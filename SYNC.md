# Folder sync between File Browser instances

File Browser can share a folder with another File Browser and keep both in
sync. The syncing itself is done by [Syncthing](https://syncthing.net) running
next to File Browser; File Browser is the interface (admin only).

## Set up (each machine)

1. Create `.env` with a random key shared by the two containers:
   `echo "SYNCTHING_API_KEY=$(openssl rand -hex 24)" > .env`
2. `docker compose -f docker-compose.yml up -d`.
   The compose file starts Syncthing, publishes its sync port on the Tailscale
   address only, and sets `FB_SYNCTHING_*` for File Browser.
3. Open **Settings → Sync**. The **Sync** tab only exists when
   `FB_SYNCTHING_URL` is set.

## Ports

Syncthing is reachable on the Tailscale address at two host ports, set in
`.env` (the values below are the defaults):

| Setting | Default | Purpose |
|---|---|---|
| `SYNC_PORT` | `23000` | TCP + UDP (QUIC) port other devices connect to |
| `DISCOVERY_PORT` | `23001` | UDP local-discovery port (idle: discovery is turned off) |

Use the same `SYNC_PORT` on every machine: it is the port assumed when you add a
device by address without a port (`FB_SYNCTHING_PORT`). Only the host side is
configurable; the container keeps using 22000 and 21027. Restart with
`docker compose -f docker-compose.yml up -d` after changing `.env`.

File Browser and Syncthing must see the files at the same path (both mount the
files at `/srv`), because Syncthing is given the real path of the folder.

## Use

1. **Pair the devices (once).** On machine A open Settings → Sync, copy
   *This device* ID. On machine B add it under *Add device* (name, and the
   address of A, e.g. its Tailscale IP). Machine A then shows B under
   *Waiting for you*: press **Accept**.
2. **Share a folder.** In the file list select a folder and press the sync
   icon (**Sync folder**). Pick the device(s) and the mode:
   - **One-way**: this folder is the source, the other side only receives.
   - **Two-way**: changes on either side are synced to the other.
3. **The other side accepts.** On the other machine, Settings → Sync shows the
   offer under *Waiting for you*. Choose where to save it (created if it does
   not exist) and whether to only receive or sync both ways, then **Accept**.
   Nothing syncs before this.
4. **Manage.** *Synced folders* lists status, pause/resume and stop. Stopping
   only ends the syncing; files stay on both sides.

## Notes

- Deleted or replaced files go to a trash (`.stversions`, kept 30 days) unless
  you untick that option. With two-way sync a delete on one side is a delete on
  the other, so keep it on.
- A `.stfolder` marker folder is created inside every synced folder; do not
  delete it.
- The scope root cannot be synced, and neither can a folder that is, or is
  inside, a symbolic link.
- Favorites and folder colors are per instance and are not synced.
- Do not sync File Browser's own `/database` or `/config`.
- Discovery, relays and NAT traversal are turned off automatically
  (`FB_SYNCTHING_PRIVATE_NETWORK=true`), so devices only talk to the addresses
  you enter.
