package solidity

import (
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Storage layout of state variables (Solidity "Layout of State Variables in
// Storage"). Pass 2 owns it, and reads everything from nodes: the type of a
// state variable and of a struct member is on the node (Signature), an enum's
// size follows from its value nodes, and a struct's size from its member
// nodes. Pass 1 used to assign slots per file with size tables of that file
// only, and Pass 2 repaired cross-file structs with a table collected while
// parsing. On an incremental build, where only the changed file is parsed,
// that table was empty and a variable typed as a struct of an unchanged file
// was laid out as one slot. Read from nodes, a full and an incremental build
// lay out the same.

// addressBytes is the storage size of an address, and so of a variable typed
// as a contract or an interface.
const addressBytes = 20

// mappingKeyword starts the type of a mapping.
const mappingKeyword = "mapping"

// typeSizes holds the storage size in bytes of the user-defined types of the
// graph, by unqualified type name. Two types of the same name in different
// files share an entry; the larger size is kept.
type typeSizes struct {
	// structs: a struct occupies whole slots, so its size is a multiple of
	// the slot size.
	structs map[string]int
	// enums: one byte for up to 256 values.
	enums map[string]int
	// containers: contracts and interfaces, whose variables hold an address.
	containers map[string]bool
}

// typeSizesFromNodes derives the sizes of every enum, struct and container
// type from the nodes of the graph.
func typeSizesFromNodes(nodes []types.Node) typeSizes {
	ts := typeSizes{structs: map[string]int{}, enums: map[string]int{}, containers: map[string]bool{}}
	enumValues := map[string]int{}
	members := map[string][]*types.Node{}
	structName := map[string]string{}
	for i := range nodes {
		n := &nodes[i]
		switch {
		case isContainerType(n.Type):
			ts.containers[n.Name] = true
		case n.Type == types.NodeStruct:
			structName[n.FilePath+refSep+n.QualifiedName] = n.Name
		case n.SubKind == subKindEnumValue:
			enumValues[n.FilePath+refSep+parentQName(n.QualifiedName)]++
		case n.SubKind == subKindStructMember:
			key := n.FilePath + refSep + parentQName(n.QualifiedName)
			members[key] = append(members[key], n)
		}
	}
	for key, count := range enumValues {
		name := lastSegment(key[strings.Index(key, refSep)+1:])
		if width := enumByteWidth(count); width > ts.enums[name] {
			ts.enums[name] = width
		}
	}
	for _, list := range members {
		sort.Slice(list, func(a, b int) bool { return list[a].StartByte < list[b].StartByte })
	}
	// A struct can hold another struct, so sizes are computed until no
	// further struct can be sized. A struct that never gets a size (it holds
	// a type the graph does not declare) is left out, and a variable of that
	// struct takes one slot. Structs are visited in a fixed order so that
	// two structs of one name always settle on the same (larger) size.
	keys := make([]string, 0, len(structName))
	for key := range structName {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sized := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, key := range keys {
			if sized[key] {
				continue
			}
			size, ok := ts.structBytes(members[key])
			if !ok {
				continue
			}
			sized[key] = true
			changed = true
			if name := structName[key]; size >= ts.structs[name] {
				ts.structs[name] = size
			}
		}
	}
	return ts
}

// parentQName drops the last segment of a qualified name.
func parentQName(qname string) string {
	if dot := strings.LastIndex(qname, qnameSep); dot >= 0 {
		return qname[:dot]
	}
	return ""
}

// lastSegment returns the part of a dotted name after its last dot.
func lastSegment(name string) string {
	if dot := strings.LastIndex(name, qnameSep); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

// structBytes returns the bytes a struct with these members occupies. ok is
// false while a member's type has no known size.
func (ts typeSizes) structBytes(members []*types.Node) (int, bool) {
	state := slotState{}
	for _, m := range members {
		var ok bool
		if _, state, ok = ts.place(state, m.Signature); !ok {
			return 0, false
		}
	}
	if state.used > 0 {
		state.slot++
	}
	return state.slot * slotBytes, true
}

// place puts one variable of the given type after state and returns the slot
// it starts in. ok is false when the type's size is not known: a struct not
// sized yet, or a type the graph does not declare.
func (ts typeSizes) place(state slotState, typeName string) (slot int, next slotState, ok bool) {
	typeName = strings.TrimSpace(typeName)
	if strings.HasPrefix(typeName, mappingKeyword) {
		slot, next = advanceForMapping(state)
		return slot, next, true
	}
	if arrayBytes, isArray := solFixedArrayBytes(typeName); isArray {
		slot, next = advanceForArrayField(state, arrayBytes)
		return slot, next, true
	}
	if size, isValue := solValueTypeSize(typeName); isValue {
		slot, next = advanceForField(state, size)
		return slot, next, true
	}
	if isDynamicSlotType(typeName) {
		slot, next = advanceForField(state, slotBytes)
		return slot, next, true
	}
	name := lastSegment(typeName)
	if width, isEnum := ts.enums[name]; isEnum {
		slot, next = advanceForField(state, width)
		return slot, next, true
	}
	if size, isStruct := ts.structs[name]; isStruct {
		slot, next = advanceForArrayField(state, size)
		return slot, next, true
	}
	if ts.containers[name] {
		slot, next = advanceForField(state, addressBytes)
		return slot, next, true
	}
	return 0, state, false
}

// slotsOccupied returns how many storage slots a state variable takes from
// its first slot on: a fixed-size array or a struct takes as many as its
// bytes need, everything else (a value that packs, a mapping, a dynamic
// array, a string) one.
func (ts typeSizes) slotsOccupied(n *types.Node) int {
	size := 0
	if arrayBytes, ok := solFixedArrayBytes(n.Signature); ok {
		size = arrayBytes
	} else if structBytes, ok := ts.structs[lastSegment(n.Signature)]; ok && n.Type == types.NodeField {
		size = structBytes
	}
	return max((size+slotBytes-1)/slotBytes, 1)
}

// layoutStateVariables assigns each state variable of a freshly parsed file
// its slot within the container that declares it, in declaration order, with
// the packing rules of the language: values smaller than a slot share one,
// a mapping, a struct and an array start a new one. A constant and an
// immutable are compiled into the code and take none. owner tells which
// container declares a variable, so two contracts of the same name in two
// files are laid out separately. The stored nodes of an unchanged file are
// final and are left alone. applyInheritanceSlotOffset then adds the slots of
// the bases.
func layoutStateVariables(nodes []types.Node, owner map[string]string, fresh map[string]bool, ts typeSizes) {
	byContainer := map[string][]*types.Node{}
	for i := range nodes {
		n := &nodes[i]
		cid, owned := owner[n.ID]
		if !owned || !isStateVariable(n) || !fresh[n.FilePath] {
			continue
		}
		byContainer[cid] = append(byContainer[cid], n)
	}
	for _, vars := range byContainer {
		sort.Slice(vars, func(a, b int) bool { return vars[a].StartByte < vars[b].StartByte })
		state := slotState{}
		for _, n := range vars {
			// Slot 0 is a slot: the marker tells it from "no slot".
			n.HasStorageSlot = true
			switch {
			case n.Type == types.NodeMapping:
				n.SlotIndex, state = advanceForMapping(state)
			case n.IsFunctionTyped:
				// A function-typed variable stores an address and a
				// selector (24 bytes) and takes the slot.
				n.SlotIndex, state = advanceForField(state, slotBytes)
			default:
				slot, next, ok := ts.place(state, n.Signature)
				if !ok {
					// A type the graph does not declare: one whole slot.
					slot, next = advanceForField(state, slotBytes)
				}
				n.SlotIndex, state = slot, next
			}
		}
	}
}
