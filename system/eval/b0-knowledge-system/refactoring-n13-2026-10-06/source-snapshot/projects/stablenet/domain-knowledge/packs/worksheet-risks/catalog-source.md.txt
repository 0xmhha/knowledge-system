# Archived worksheet review references

Origin: cmd/cks/domaincli/worksheet.go at fecb6f51fb12ea62205f0dfd8b49d3a46f426333.
Origin file SHA-256: f4dec0c15b2fd95dd0436435b6b3060976a34678e3fd87f8d21323092ec54579.
Authority: copied legacy keyword heuristic; human review required, not verified operating policy.
Original reference: coding-agent/docs/r1-refactor/07-domain-knowledge-curation.md section 9.
Original reference: coding-agent/docs/r1-refactor/08-p0c-foundations-t2-and-internalization.md section 4.

1. stake-weighted voting / slashing; keywords: stake, weight, weighted, slashing, slash, equal, power, validator.
2. reorg / probabilistic-finality (forker, Td inert under WBFT); keywords: reorg, finality, forker, totaldifficulty, probabilistic, td, fork.
3. ETH-denominated assumptions (WKRC, base-fee redistribution); keywords: eth, wkrc, basefee, base-fee, burn, redistribut, ether, wei, denominat.
4. quorum reimplementation (ceil(N−F) split-brain); keywords: quorum, ceil, n-f, split-brain, supermajority, count.
5. feepayer sigHash payload; keywords: feepayer, sighash, payload, fee-delegation, fee_delegation, sign, rlp.
6. missing blacklist enforcement point; keywords: blacklist, transfer, blacklisted, blocked, denylist.
7. concurrency (Core.current off mutex, txpool mutation); keywords: concurrenc, mutex, rwmutex, lock, race, txpool, goroutine.
8. breaking cherry-pick-ability (interleave StableNet into geth); keywords: cherry, cherrypick, cherry-pick, upstream, isolate, interleave, geth-origin.
