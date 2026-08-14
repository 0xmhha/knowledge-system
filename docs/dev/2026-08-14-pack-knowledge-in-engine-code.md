# A pack's domain knowledge is compiled into an engine command

2026-08-14. Found while adding the boundary rule that forbids a pack's name in
engine string literals. The rule flags one file it cannot honestly fix, so the
exception is recorded here with what fixing it would take.

## What it is

`cmd/cks/domaincli/worksheet.go` carries `catalog`, the ranked list the
promotion worksheet scores entries against. All eight items are one project's
domain:

```
1  stake-weighted voting / slashing
2  reorg / probabilistic-finality (forker, Td inert under WBFT)
3  ETH-denominated assumptions (WKRC, base-fee redistribution)
4  quorum reimplementation (ceil(N-F) split-brain)
5  feepayer sigHash payload
6  missing blacklist enforcement point
7  concurrency (Core.current off mutex, txpool mutation)
8  breaking cherry-pick-ability (interleave StableNet into geth)
```

Only item 8 names the pack, so the boundary rule sees one violation. Renaming
it would clear the rule and change nothing: WBFT, WKRC, feepayer and geth are
the same violation wearing vocabulary the rule cannot recognise. Filing the
name off would make the check assert a cleanliness this file does not have,
which is worse than the leak.

## Why it is not fixed here

The catalog belongs in the pack, beside the domain entries it ranks. Moving it
is a feature change, not a rename:

- it needs a format and a location — the natural one is a YAML beside the
  pack's `domain-knowledge/`, which `cks domain worksheet` already receives as
  `--project`;
- it needs a decision about absence. A pack with no catalog either scores
  nothing, or the command refuses. Both are defensible and neither is today's
  behaviour;
- the keyword lists are the scoring input, so the format has to carry them
  without turning into a second policy language.

Bundling that into a lint change would hide a design decision inside a
mechanical one.

## What holds until then

`scripts/check-boundaries.sh` exempts this file by name, next to the reason.
The exemption is the record: it is visible in the check, it names this
document, and it fails loudly if someone deletes the file it points at rather
than quietly passing.

The other exemption, `pkg/mcp/namespace.go`, is different in kind and stays:
it documents the branding mechanism, and an example of a branded namespace has
to name a brand to be an example.
