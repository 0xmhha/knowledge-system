package knowledgepack

import (
	"fmt"
	"sort"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

// ProjectRetainedKnowledge derives v4 tuple-scoped types only from a locked,
// verified source archive. The default semantic builder stays at v3 unless
// the caller explicitly requests this extension.
func ProjectRetainedKnowledge(versionDir string) (*semantic.KnowledgeProjection, error) {
	_, lock, err := LoadRetainedProjectInstances(versionDir)
	if err != nil {
		return nil, err
	}
	result := &semantic.KnowledgeProjection{LockDigest: lock.LockDigest, Packs: []semantic.PackProjection{}}
	for _, pin := range lock.Packs {
		buf, _, _, err := setup.ReadRetainedFile(versionDir, pin.OriginID, "pack.yaml")
		if err != nil {
			return nil, err
		}
		pack, err := Decode(buf)
		if err != nil {
			return nil, err
		}
		if pack.PackID != pin.PackID || pack.Version != pin.Version {
			return nil, fmt.Errorf("pack_incompatible: locked pack source identity differs")
		}
		projected, err := semantic.ProjectPackSource(buf, pin.PackID, pin.Version, pin.Digest, pin.OriginID)
		if err != nil {
			return nil, err
		}
		result.Packs = append(result.Packs, projected)
	}
	sort.Slice(result.Packs, func(a, b int) bool { return result.Packs[a].PackID < result.Packs[b].PackID })
	return result, nil
}
