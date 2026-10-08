// Read-only reproduction of nonfatal calls in the synthetic telemetry fixture.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func main() {
	graph := flag.String("graph", "", "existing fixture graph.db")
	capture := flag.String("capture", "", "existing baseline capture JSON")
	flag.Parse()
	b, err := os.ReadFile(*capture)
	must(err)
	var input struct {
		Rows []struct {
			Call struct {
				Response struct {
					StructuredContent struct {
						Citations []contract.Citation `json:"citations"`
					} `json:"structuredContent"`
				} `json:"response"`
			} `json:"call"`
		} `json:"rows"`
	}
	must(json.Unmarshal(b, &input))
	client, err := ckgclient.NewReal(*graph)
	must(err)
	defer client.Close()
	_, err = client.BM25Search(context.Background(), "<convention>", ckgclient.SearchOpts{K: 5})
	fmt.Printf("synthetic pseudo-keyword: %v\n", err)
	for _, c := range input.Rows[0].Call.Response.StructuredContent.Citations {
		n, err := client.Neighbors(context.Background(), c, ckgclient.NeighborsOpts{Relations: []contract.Relation{contract.RelationCalls}, Hops: 2, MaxTotal: 50})
		fmt.Printf("%s:%d-%d neighbors=%d error=%v\n", c.File, c.StartLine, c.EndLine, len(n), err)
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
