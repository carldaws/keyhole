# keyhole

Read 1Password secrets from inside a VM, through the 1Password app on your Mac.

The 1Password app only talks to programs on the same machine. keyhole runs on the Mac, signs in to one account through the app, and answers requests for secrets on a Unix socket. Forward that socket into the VM over SSH and programs there can read secrets without a copy of them or a service account token. 1Password asks you to approve keyhole when it first connects. While the app is locked, requests fail until you unlock it.

Anything that can reach the socket can read every secret in the account, so forward it only where you mean to.

## On the Mac

```sh
brew install carldaws/tap/keyhole
```

Turn on Settings → Developer → Integrate with other apps in the 1Password app, then install a keyhole for each account:

```sh
keyhole install personal -account <account>
```

`<account>` is the account's name as the app's sidebar shows it, or its UUID. The keyhole serves the account on `~/.keyhole/personal.sock`, starts now and whenever you log in, and logs to `~/Library/Logs/keyhole-personal.log`. Run `keyhole install` again with new flags to change them, and `keyhole uninstall personal` to remove it. The keyhole runs the `keyhole` you installed it with, so install it from the one on your `PATH`. `keyhole serve` runs one in the foreground instead, with the same flags and a `-socket`.

The socket's directory must be private to you; keyhole creates it if it's missing.

One keyhole serves one account: the 1Password SDK sends every request in a process as the first account it connected to.

## In the VM

With [Lima](https://lima-vm.io), forward the socket in the VM's config and Lima sets it up whenever the VM starts:

```yaml
portForwards:
- guestSocket: "/run/user/{{.UID}}/keyhole.sock"
  hostSocket: "{{.Home}}/.keyhole/personal.sock"
  reverse: true
```

Install the keyhole before you first start the VM. Otherwise Lima makes the socket's directory, too open for keyhole, and you'll need to `chmod 700` it.

Over plain SSH, set `StreamLocalBindUnlink yes` in the VM's `/etc/ssh/sshd_config`, so a reconnection can replace the socket the last one left behind. Then forward the socket when you connect, where `<uid>` is your user ID in the VM (`id -u`):

```sh
ssh -R /run/user/<uid>/keyhole.sock:$HOME/.keyhole/personal.sock vm
```

Read secrets with `curl` and `jq`:

```sh
keyhole_read() {
  jq -n --arg ref "$1" '{secret: $ref}' |
    curl -sS --fail-with-body --unix-socket "$KEYHOLE_SOCKET" --data @- http://keyhole/ |
    jq -er '.secret // error(.error)'
}

KEYHOLE_SOCKET=/run/user/<uid>/keyhole.sock keyhole_read op://Personal/Postgres/password
```

Name vaults by their title as the 1Password app shows it.

## API

keyhole speaks HTTP over the socket. `POST /` with names for the secret references you want:

```json
{"db": "op://Personal/Postgres/password", "mail": "op://Personal/Mail/token"}
```

replies with a secret for each name:

```json
{"db": "…", "mail": "…"}
```

Errors reply `{"error": "…"}`, with status 400 for a request keyhole doesn't understand and 502 when 1Password can't resolve every reference. A request gets all its secrets or none.

For a client that expects a different shape, `-path` moves the API and `-wrap` takes references wrapped in a key. With `-path /api/secrets -wrap secrets`, keyhole answers `POST /api/secrets` with `{"secrets": {"db": "op://…"}}`, and replies flat as before.

## Building

keyhole needs cgo, because the 1Password SDK loads a library from the 1Password app, so build it on a Mac with the Xcode command line tools:

```sh
go build
```
