package solidity

// Sol C3 linearization for inheritance
// slot offset deduplication.
//
// A plain depth-first walk of the inheritance adjacency that cumulates
// parent slot counts is wrong for diamonds: diamond inheritance (two parents that
// both extend the same grandparent) over-counted the shared
// ancestor's slots. Solidity solves this with C3 linearization,
// the same algorithm Python and Dylan use for multiple-
// inheritance method resolution order (MRO). Sol's reference
// compiler applies C3 to storage layout: the storage of the most-
// base class lands first, then each next class in the
// linearization, then the derived contract's own state vars.
//
// computeC3LinearizationWithFallbacks returns each contract's
// linearisation (itself first, then its bases from the most derived
// to the most base-like). Name lookup walks the list in order;
// storage layout sums slotCount(c) over the list without the
// contract itself.
//
// Reference: Barrett et al. "A Monotonic Superclass Linearization
// for Dylan" (OOPSLA '96) — the algorithm Python adopted in
// PEP 3119 and Solidity's reference implementation in libsolidity.
//
// Fallback behaviour: if a class hierarchy is inconsistent
// (no C3 linearization exists — Sol's compiler rejects this, but
// the grammar accepts it), the function falls back to the plain
// depth-first order so storage layout stays computable. The
// caller's tests still observe a deterministic answer.

// computeC3LinearizationWithFallbacks returns mro[contractID] = [self,
// last base listed, …, most base-like ancestor] for every contract
// reachable from parents, and the set of contracts whose hierarchy has no
// consistent linearisation and fell back to a depth-first walk. Those are
// flagged (HasInheritanceMROFallback): the compiler rejects them.
func computeC3LinearizationWithFallbacks(parents map[string][]string) (map[string][]string, map[string]bool) {
	out := make(map[string][]string, len(parents))
	fallback := map[string]bool{}
	visiting := map[string]bool{}
	var lin func(id string) []string
	lin = func(id string) []string {
		if cached, ok := out[id]; ok {
			return cached
		}
		if visiting[id] {
			// Cycle defence: degrade to the trivial walk (just self).
			return []string{id}
		}
		visiting[id] = true
		defer delete(visiting, id)

		ps := parents[id]
		if len(ps) == 0 {
			res := []string{id}
			out[id] = res
			return res
		}
		// Build the list of parent linearizations plus the bare
		// parent order — the canonical C3 inputs — with the bases
		// taken from the last one listed to the first. Solidity lists
		// bases from the most base-like to the most derived (`contract
		// C is Base, Derived`), the reverse of Python, so the last base
		// is searched first. Taken in source order, a hierarchy like
		// `NativeCoinAdapter is AbstractFiatToken, Mintable, EIP3009,
		// EIP2612` (EIP3009 itself extends AbstractFiatToken) has no
		// linearisation, and a contract that compiles was flagged.
		derivedFirst := make([]string, 0, len(ps))
		for i := len(ps) - 1; i >= 0; i-- {
			derivedFirst = append(derivedFirst, ps[i])
		}
		lists := make([][]string, 0, len(ps)+1)
		for _, p := range derivedFirst {
			lists = append(lists, append([]string(nil), lin(p)...))
		}
		lists = append(lists, derivedFirst)

		merged := mergeC3(lists)
		if merged == nil {
			// Inconsistent hierarchy. Fall back to the plain depth-
			// first walk (self + every reachable ancestor in
			// source order, no dedup) and flag the diagnostic.
			merged = depthFirstAncestors(id, parents)
			fallback[id] = true
		}
		res := append([]string{id}, merged...)
		out[id] = res
		return res
	}
	for id := range parents {
		lin(id)
	}
	return out, fallback
}

// mergeC3 runs the C3 merge step over a list of candidate lists.
// At each iteration it picks a "good head" — the first element of
// some list that doesn't appear in the tail of any other list —
// appends it to the result, and removes it from the front of any
// list whose head matches. Returns nil when no good head exists
// (inconsistent MRO).
func mergeC3(lists [][]string) []string {
	var out []string
	for {
		// Drop empty lists.
		filtered := lists[:0]
		for _, L := range lists {
			if len(L) > 0 {
				filtered = append(filtered, L)
			}
		}
		lists = filtered
		if len(lists) == 0 {
			return out
		}
		var head string
		good := false
		for _, L := range lists {
			cand := L[0]
			inTail := false
			for _, K := range lists {
				for i := 1; i < len(K); i++ {
					if K[i] == cand {
						inTail = true
						break
					}
				}
				if inTail {
					break
				}
			}
			if !inTail {
				head = cand
				good = true
				break
			}
		}
		if !good {
			return nil
		}
		out = append(out, head)
		// Remove `head` from the front of every list that starts
		// with it.
		for i, L := range lists {
			if len(L) > 0 && L[0] == head {
				lists[i] = L[1:]
			}
		}
	}
}

// depthFirstAncestors walks parents[id] recursively in source
// order, returning every reachable ancestor (excluding self). Used
// as the C3 fallback when no consistent linearization exists; it
// reproduces the plain depth-first walk order so the
// inconsistent-hierarchy case stays deterministic.
func depthFirstAncestors(id string, parents map[string][]string) []string {
	var out []string
	seen := map[string]bool{id: true}
	var rec func(string)
	rec = func(cid string) {
		for _, p := range parents[cid] {
			if seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
			rec(p)
		}
	}
	rec(id)
	return out
}
