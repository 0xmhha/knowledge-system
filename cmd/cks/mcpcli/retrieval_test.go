package mcpcli

import (
	"context"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/budget"
	"github.com/0xmhha/knowledge-system/internal/system/composer/intent"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage1"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	"gopkg.in/yaml.v3"
)

func TestConfiguredRecallKReachesComposerAndPreservesKnowledgeBudget(t *testing.T) {
	for _, k := range []int{0, 10, 3} {
		cfg := config.Default()
		cfg.Retrieval.RecallK = k
		vector := &ckvclient.Fake{}
		graph := &ckgclient.Fake{}
		c, err := buildComposer(context.Background(), graph, vector, &intent.FakeEmbedder{}, &budget.FakeFetcher{}, &config.SanitizeRuleset{}, nil, nil, true, cfg.Retrieval.EffectiveRecallK())
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.ComposeWithIntent(context.Background(), "Alpha implementation", contract.IntentBugFix)
		if err != nil {
			t.Fatal(err)
		}
		calls := vector.Calls.SemanticSearch
		if len(calls) < 2 {
			t.Fatal("missing raw recall and knowledge pass")
		}
		for _, call := range calls[:len(calls)-1] {
			if call.Opts.K != cfg.Retrieval.EffectiveRecallK() || !call.Opts.BM25Rerank {
				t.Fatalf("wrong raw search opts: %+v", call.Opts)
			}
		}
		if calls[len(calls)-1].Opts.K != stage1.DefaultConfig().KnowledgeK {
			t.Fatal("knowledge budget changed")
		}
	}
	raw, err := yaml.Marshal(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "retrieval:") {
		t.Fatal("zero config changed generated YAML")
	}
}
