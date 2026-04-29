# minecraft Plugin

`minecraft` provides Minecraft Java Edition TCP frontends for warming targets from real connection attempts while keeping server-list status probes local.

Frontends:

- `minecraft:status`: answers Minecraft status pings locally and never emits warm signals.
- `minecraft:server`: emits a warm signal when a client connects, sends a configurable disconnect message, and closes the connection.

## minecraft:status Frontend

`minecraft:status` requires TCP. It handles server-list status requests and optional ping packets. It intentionally returns no `ShouldWarm` channel, so polling the server list does not warm the target.

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

- `status`: server-list description text. Defaults to `Proxy Gateway`.

## minecraft:server Frontend

`minecraft:server` requires TCP. Each accepted connection queues a warm signal, receives a Minecraft disconnect packet with the configured message when it attempts login, and is closed.

```yaml
frontends:
  - name: minecraft-server
    kind: minecraft:server
    protocol: tcp
    listen: 0.0.0.0:25566
    flow_timeout: 30s
    target: minecraft:game
    options:
      message: "Server is starting, please reconnect in a moment."
```

Options:

- `message`: login disconnect text. Defaults to `Server is starting, please try again soon.`
- `status`: server-list description text when this frontend receives a status ping. Defaults to `Proxy Gateway`.
