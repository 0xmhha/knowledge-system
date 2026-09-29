package main

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// This fixture makes a legacy-shaped, integrity-stamped pack so the smoke can
// exercise annotation against the actual indexed CKG/CKV/CKS dataset.
func main() {
	if len(os.Args) != 6 {
		panic("usage: make-pack FILE START END COMMIT OUT")
	}
	start, err := strconv.Atoi(os.Args[2])
	if err != nil {
		panic(err)
	}
	end, err := strconv.Atoi(os.Args[3])
	if err != nil {
		panic(err)
	}
	pack := contract.EvidencePack{Intent: contract.IntentUnknown, Query: "find Alpha",
		Citations: []contract.Citation{{File: os.Args[1], StartLine: start, EndLine: end, CommitHash: os.Args[4]}}}
	if err := contract.StampIntegrity(&pack); err != nil {
		panic(err)
	}
	data, err := json.Marshal(pack)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[5], data, 0600); err != nil {
		panic(err)
	}
}
