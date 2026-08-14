# Keeping the server reachable

A `cks mcp` started from a shell is available only as long as that shell, that
login session, and that host's willingness to stay awake last. This document
covers the deployment shape that removes all three dependencies on a macOS
host, and the recovery path an operator on another machine uses when something
still goes wrong.

Three failure modes, three defences. They are independent — none of them
substitutes for another:

| What fails | What catches it |
|---|---|
| The host goes idle and sleeps; the listener disappears from the network | `pmset` policy, plus a `caffeinate` assertion held by the server process itself |
| The server process exits — crash, `OOM`, an operator closing the terminal | launchd `KeepAlive` restarts it, and `RunAtLoad` starts it at login |
| The process is alive but not serving — wedged, dataset not serviceable | a watchdog agent probes `/healthz` on a timer and restarts it |

## Install

The launchd agents are generated from the config, so the instance name, port
and dataset all come from one place.

```sh
bin/cks mcp service install --config cks.yaml
```

This writes three user agents to `~/Library/LaunchAgents` and loads them:

- `<prefix>.<instance>` — the server, run as
  `caffeinate -s -i bin/cks mcp --config <config>`, with `KeepAlive` and
  `RunAtLoad`. caffeinate is the parent process, so the no-sleep assertion
  exists exactly as long as the server does and is released the moment it
  stops.
- `<prefix>.<instance>.watchdog` — a timer job running
  `cks mcp service recover` every minute (`--watchdog-interval` changes it).
- `<prefix>.<instance>.network` — a timer job running
  `cks mcp service watch-network --once`, which republishes the URL when the
  host moves to a different network (`--interval` changes it).

`<prefix>` defaults to `knowledge-system`, the engine's name: one build serves
any pack, and which one an instance serves is already carried by the instance
name the label ends with. `service.label_prefix` in the config overrides it,
for the two cases that need it — a host running two distributions of this
software, and a deployment whose agents are already installed under another
prefix. The second is a real constraint rather than a preference: a label is
how launchd finds a job, so changing it does not rename what is running, it
loses the handle on it.

A port already served by a process the agent does not own is refused, because
two servers on one port turns into a launchd restart loop against a bind error.
Take the port over deliberately:

```sh
bin/cks mcp service install --config cks.yaml --takeover
```

Because these are *user* agents in the login session's domain, the host has to
reach a logged-in session for the server to start after a reboot. On a machine
dedicated to serving, enable automatic login (System Settings → Users & Groups
→ Automatic login). Without it, a reboot leaves the server down until someone
logs in — which the remote recovery path below cannot fix, since it depends on
the same session.

## Power policy

`install` and `status` both adjudicate the host's live `pmset` settings against
what a continuously-serving host needs, and print the one command that fixes
every violation:

```
power policy: the host may go unreachable —
  - sleep is 1, needs 0: idle system sleep takes the listener down; a client sees a dead port, not a slow one
  - autorestart is 0, needs 1: after a power cut the host must come back without someone pressing the button
  fix with: sudo /usr/bin/pmset -a sleep 0 autorestart 1
```

The command is printed rather than run: changing power policy needs root, and a
server binary should not be asking for it. Four settings are required, each for
a stated reason — `sleep 0`, `standby 0`, `womp 1` (a magic packet is the only
remote wake), `autorestart 1`. A setting the hardware does not expose is not
reported.

`caffeinate` is the second line of defence here, not the first: it keeps this
host awake even if someone later resets `pmset`, but it only covers the window
in which the server is running.

## Verify

```sh
bin/cks mcp service status --config cks.yaml
```

reports both agents' load state, whether the address is actually serving, and
the power verdict. The server's own output is in `run/<instance>.launchd.log`;
the watchdog logs only when it acts, in `run/<instance>.watchdog.log`.

## Remote recovery from another machine

When the watchdog cannot fix it, an operator on another machine triggers
recovery over SSH. The key they use is pinned to a forced command, so the
connection can request recovery and nothing else — a leaked key buys an
attacker one restart of a server that is meant to be running, not a login.

On the serving host, once:

```sh
sudo systemsetup -setremotelogin on
system/ops/install-recovery-key.sh ~/ops-operator.pub --from 172.20.0.0/16
```

`--from` restricts which addresses may use the key and is worth setting; the
forced command additionally denies a tty, port forwarding, agent forwarding and
the user's own rc files.

From the operator's machine:

```sh
ssh -i ~/.ssh/cks_recovery <user>@<host>            # recover if unhealthy
ssh -i ~/.ssh/cks_recovery <user>@<host> status     # report, change nothing
ssh -i ~/.ssh/cks_recovery <user>@<host> force      # restart unconditionally
```

Those three words are the entire vocabulary; anything else is rejected and
logged. Every request — accepted or rejected — is appended to
`run/remote-recovery.log` with the client address. The exit status is the
verdict: zero means the instance is serving.

If the host is asleep rather than broken, wake it with a magic packet before
connecting (this is what `womp 1` is for), then run the same command.

## What recovery actually does

`cks mcp service recover` probes `/healthz`, and when the instance is not
serving it restarts the launchd job with `launchctl kickstart -k` and polls
until the instance serves again or `--timeout` (default 90s) expires. A serving
instance is left alone unless `--force` is given — the watchdog runs this every
minute, so doing nothing has to be the normal case.

The restart goes through launchd rather than the pid, because signalling the
process directly races with the `KeepAlive` respawn.

### The dependency comes first

Before the instance is touched, recovery checks the embedding daemon.
`serviceable()` requires the model to be reachable, so a daemon that is down
makes `/healthz` report 503 while the server itself is perfectly fine —
restarting the server there is downtime that ends in another 503.

This is not hypothetical. It is what this deployment produces: on the host
these tools were written for, the MCP server had been up 1d21h without
interruption while the embedding daemon had started minutes after the machine
woke from a 1h42m sleep. The daemon is a GUI app with no launchd agent of its
own; it does not survive a sleep, and the server does.

So the ladder is: probe the daemon, start it if it is down, wait for it to
answer, and re-probe health. If that recovered the instance, the server is
never restarted — it reconnects on its own. Only if the daemon was already up,
or restoring it did not help, does the restart happen. The report names which
rung acted, so a log says whether the server was bounced at all.

`--force` skips the ladder: it means "restart the instance", not "diagnose it".

### Restarts are rate limited

An instance that cannot come back would otherwise be restarted every watchdog
tick, each one paying the dataset load. A restart within `Cooldown` (default
5m) of the previous one is suppressed and reported as `suppressed` rather than
`failed` — the instance is still not serving, so the exit status is non-zero
either way, but "we chose not to try" and "we tried and it did not work" call
for different next moves.

The last restart is persisted under the run dir, because each watchdog tick is
a fresh process; a cooldown held in memory would be no cooldown at all.
`--force` ignores it.

Recovery requires the agent to be loaded; it restarts a deployment, it does not
create one. If the label is not loaded, `recover` says so and points at
`service install`.

## Uninstall

```sh
bin/cks mcp service uninstall --config cks.yaml
```

unloads both agents and removes their definitions. The power policy is left as
it is — it is host configuration, not this deployment's.
