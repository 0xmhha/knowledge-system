package setup

import (
	"context"
	"fmt"

	"github.com/0xmhha/knowledge-system/pkg/sourcepath"
)

// PrepareRetainedCorpora renders domain text before source identity is pinned.
// The same ordered document roots used by CKV are retained with distinct origins.
// Subsequent build steps must not regenerate their bytes after capture.
func PrepareRetainedCorpora(ctx context.Context, o *Options, runner Runner, emit func(Event)) error {
	if o.SkipVector || o.RetainedCorporaPrepared {
		return nil
	}
	if o.DomainKnowledge != "" {
		binary := o.CksBin
		if binary == "" {
			binary = "cks"
		}
		argv := []string{binary, "domain", "export", "--project", o.DomainKnowledge, "--out", o.DomainCorpusDir()}
		if o.CodeRoot != "" {
			argv = append(argv, "--code-root", o.CodeRoot)
		}
		if err := Execute(ctx, Plan{Steps: []Step{{ID: "domain-export", Title: "Render domain text before retaining source identity", Cmd: argv}}}, runner, emit); err != nil {
			return err
		}
	}
	roots, err := RetainedDocumentRoots(*o)
	if err != nil {
		return err
	}
	for index, root := range roots {
		id := sourcepath.Origin(index)
		for _, existing := range o.ExternalOrigins {
			if existing.ID == id {
				return fmt.Errorf("duplicate corpus origin %s", id)
			}
		}
		o.ExternalOrigins = append(o.ExternalOrigins, CaptureOrigin{ID: id, Root: root})
	}
	o.RetainedCorporaPrepared = true
	return nil
}
