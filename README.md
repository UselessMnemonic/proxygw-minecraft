# Minecraft Plugin

`proxygw-minecraft` is an external plugin module that provides Minecraft Java
Edition TCP frontends for `proxygw`.

## Plugin Setup

Add the plugin to the `proxygw` daemon build in the main `proxygw` repository:

1. Add this module as a dependency:

```sh
go get github.com/UselessMnemonic/proxygw-minecraft@latest
```

2. Register the plugin in `plugin.yaml`:

```yaml
plugins:
  github.com/UselessMnemonic/proxygw-minecraft: minecraft
```

3. Regenerate the plugin import file and rebuild the daemon:

```sh
go generate ./cmd/proxygw
make proxygw
```

The plugin registers under the module path
`github.com/UselessMnemonic/proxygw-minecraft` and uses the `minecraft`
namespace, so frontend kinds are referenced as `minecraft:...`.

## Exported Kinds

Frontends:

- `status`: answers Minecraft server-list status pings and never emits
  warm signals.
- `server`: emits a warm signal when a client connects, sends a configurable
  disconnect message, and closes the connection.

There is no plugin-level configuration for this plugin.

## status Frontend

Example:

```yaml
frontends:
  - name: minecraft-status
    kind: minecraft:status
    protocol: tcp
    listen: 0.0.0.0:25565
    flow_timeout: 30s
    target: minecraft:game
    options:
      status: "Server is sleeping"
```

Options:

- `status`: optional server-list description text. Defaults to
  `Proxy Gateway`.

## server Frontend

Example:

```yaml
frontends:
  - name: minecraft-server
    kind: minecraft:server
    protocol: tcp
    listen: 0.0.0.0:25566
    flow_timeout: 30s
    target: minecraft:game
    options:
      login: "Server is starting, please reconnect in a moment."
      motd: "Server is sleeping"
```

Options:

- `login`: optional login disconnect text.
- `motd`: optional server-list description text when this frontend receives a
  status ping.
