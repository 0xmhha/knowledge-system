// Command cks-domain-export renders a project's domain-knowledge entries
// (status verified/needs_verification) plus its authoritative_docs into a
// markdown corpus that `ckv build --docs <out>` embeds. This is the
// producer side of channel ②.
//
// Usage:
//
//	cks-domain-export -project projects/stablenet/domain-knowledge \
//	  -out generated/domain-corpus/go-stablenet
//
// code_root for authoritative_docs resolves via -code-root, CKS_CODE_ROOT, or
// the project.yaml ${GO_STABLENET_ROOT} env, same as the validator.
//
// A project that declares authoritative_docs but cannot resolve them is an
// error here, not a warning: the corpus would silently ship without the
// project's own documentation, and the only symptom downstream is a smaller
// chunk count that nobody is looking at. Pass -allow-missing-docs to keep the
// old lenient behaviour for a tree that genuinely has no docs checked out.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/0xmhha/knowledge-system/internal/system/domainexport"
	"github.com/0xmhha/knowledge-system/internal/system/inventory"
)

func main() {
	projectDir := flag.String("project", "", "project directory (contains project.yaml, subsystems.yaml, entries/)")
	outDir := flag.String("out", "", "output corpus directory")
	codeRoot := flag.String("code-root", "", "working tree the authoritative_docs resolve against (overrides CKS_CODE_ROOT and project.yaml code_root)")
	allowMissingDocs := flag.Bool("allow-missing-docs", false, "downgrade unresolvable authoritative_docs from an error to a warning")
	flag.Parse()

	if *projectDir == "" || *outDir == "" {
		fmt.Fprintln(os.Stderr, "cks-domain-export: -project and -out are required")
		flag.Usage()
		os.Exit(2)
	}

	p, err := inventory.LoadProject(*projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cks-domain-export: %v\n", err)
		os.Exit(1)
	}
	if *codeRoot != "" {
		p.CodeRoot = *codeRoot
	}
	res, err := domainexport.Export(p, *outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cks-domain-export: %v\n", err)
		os.Exit(1)
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(os.Stderr, "cks-domain-export: warning: %s\n", w)
	}
	if want := len(p.AuthoritativeDocs); want > res.DocsCopied && !*allowMissingDocs {
		fmt.Fprintf(os.Stderr, "cks-domain-export: %d of %d authoritative_docs could not be resolved"+
			" (code_root=%q). The corpus would ship without them; pass -code-root, or"+
			" -allow-missing-docs to accept the gap.\n", want-res.DocsCopied, want, p.CodeRoot)
		os.Exit(1)
	}
	fmt.Printf("cks-domain-export: %d entries, %d docs -> %s\n", res.EntriesWritten, res.DocsCopied, *outDir)
}
