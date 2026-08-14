# Answers on the launchd label: four of five observations were right

2026-08-14. Answers the five questions downstream raised about the label
change in #91. Short version: nothing in that account was misread, three of
the five describe a gap this side left, and the gap is now closed rather than
explained.

## 1. What fills `Deployment.LabelPrefix`? Nothing did

That is the correct reading. The field and its accessor existed; no code
assigned it. Worse, its comment said a deployment "sets
Deployment.LabelPrefix from its config" — describing a path that did not
exist. That sentence is why the question had to be asked at all, and it was
upstream's to answer, not downstream's to build.

`service.label_prefix` is now a config field, read into the deployment, and
writable through `gen-config --service-label-prefix`.

**Not the namespace stamp**, though the observation that it would satisfy
`check-boundaries.sh` is correct. Two reasons:

- Tool names and launchd labels are different axes with different spellings.
  This distribution already carries both — `stablenet_knowledge` for one,
  `com.stablenet.knowledge` for the other — so binding them forces one to
  change to suit the other.
- More decisively, it would not have avoided the migration the field exists to
  avoid. `BuildRoot` would make the label `stablenet_knowledge.go-stablenet`,
  which matches neither installed agent. Only a config value can name the
  prefix that is already there.

## 2. Which name do the installed agents converge on? The one they already have

Keep `com.stablenet.knowledge`. One line in the config:

```yaml
service:
    label_prefix: com.stablenet.knowledge
```

There is no migration to design, because there is no migration. The reasoning
in the question is right — with the prefix changed, `service uninstall`
computes a label it cannot find and the old agents can only be removed by hand
— which is an argument against changing it on a host that already has agents,
not an argument for a better migration procedure.

Verified on the live host before porting anything, read-only, same binary,
config the only difference:

```
without service.label_prefix   knowledge-system.go-stablenet          not loaded
                               knowledge-system.go-stablenet.watchdog not loaded
with    service.label_prefix   com.stablenet.knowledge.go-stablenet          loaded
                               com.stablenet.knowledge.go-stablenet.watchdog loaded
```

## 3. Is this transitional? It is worse than transitional, and the report understates it

Not a half-finished migration — a live regression, and the exit code named in
the question is the evidence:

```
com.stablenet.knowledge.go-stablenet.watchdog   runs = 3191, last exit code = 1
run/go-stablenet.watchdog.log:
  cks: knowledge-system.go-stablenet is not loaded — install it first
```

Every tick since the binaries were rebuilt. What that costs is worth stating
precisely, because most of the deployment is fine and that is what makes it
easy to miss:

| | state |
|---|---|
| Answering requests | fine — the server never reads its own label |
| Respawn after a crash | fine — launchd `KeepAlive`, independent of this code |
| **Detecting up-but-not-serving** | **gone** — that is the watchdog's whole job |
| **The embedder recovery ladder** | **gone** — it runs inside that path |
| `service status` | misreports a running instance as not loaded |
| `service uninstall` | cannot find the agents that are running |

So: no outage, and the thing that would have caught one is off. The same shape
as the defects this pair of repositories has been finding all week — a failure
that does not look like one.

The double-install hazard in the question is real too, and worth spelling out
for anyone reading later: `service install` in this state adds a second set of
agents contending for port 8930, and `--takeover` would then stop a "foreign"
process that is in fact this deployment's own server.

## 4. Is `knowledge-system` the software's name? Yes, and the rule reading is right

The model is exactly as described: the prefix names the software, the suffix
names the instance, and which project an instance serves is already carried by
that instance name — which the registry keeps unique per host and the pidfiles
already key on.

And the boundary rule does not conflict. It forbids **pack** names, read from
`projects/`. `knowledge-system` is the engine's name, not a pack's.

Worth adding, since it makes the separation concrete — with the stamp set,
upstream computes:

```
KNOWLEDGE_MCP_NAMESPACE=stablenet_knowledge  →  knowledge-system.stablenet_knowledge
```

The prefix does not move. The stamp reaches only the *default instance name*,
and not even that when the config names one. The two axes stay apart.

## 5. Missed context? The "not loaded" report is not a signal

It is the side effect of an unwired override, not a deliberate way of showing
an operator that the label moved. There was no intent behind it to reconstruct.

The judgement it turned on — whether that report was designed or accidental —
was the right thing to ask before acting on it, and asking rather than
migrating was the better call. Migrating would have solved a problem that did
not need to exist.

## What this side got wrong, plainly

The direction was sound and stands: a distribution's name does not belong in
engine source, and the label names the software. What was wrong was shipping
half of it. A default that changes for everyone landed in the same commit as
an escape hatch that worked for no one, with a comment claiming otherwise. The
override should have been wired in the change that moved the default; there
was never a window where the new default was safe without it.

One more thing the port turned up, in the same family: the boundary rule from
#91 broke on contact with this tree, because a distribution's module path
contains the pack name and every import of its own packages matched. It passed
upstream only because that module path carries no pack name — the rule had
been exercised against one of the two shapes it has to work in. Fixed in both
trees.
