// Package domaincli is the `cks domain` command group: the
// domain-knowledge toolchain (corpus export, engine-view sync, glossary
// generation, entry verification, inventory checks, anchor refresh, and
// promotion worksheets).
//
// --project is a persistent flag on the group — every tool here operates
// on a project directory (project.yaml, subsystems.yaml, entries/), so it
// is declared once and inherited.
package domaincli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/0xmhha/knowledge-system/internal/system/domainexport"
	"github.com/0xmhha/knowledge-system/internal/system/inventory"
	"github.com/0xmhha/knowledge-system/internal/system/vocab"
)

// NewCmd builds the `cks domain` command group.
func NewCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Domain-knowledge toolchain (export, sync, verify, ...)",
	}
	cmd.PersistentFlags().StringVar(&project, "project", "", "project directory (contains project.yaml, subsystems.yaml, entries/)")
	cmd.AddCommand(
		newExportCmd(&project),
		newSyncCmd(&project),
		newGlossaryGenCmd(&project),
		newVerifyCmd(&project),
		newCheckCmd(&project),
		newAnchorsCmd(&project),
		newWorksheetCmd(&project),
	)
	return cmd
}

func requireProject(project *string) error {
	if *project == "" {
		return fmt.Errorf("--project is required")
	}
	return nil
}

func newExportCmd(project *string) *cobra.Command {
	var outDir, codeRoot string
	var allowMissingDocs bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Render the domain-knowledge entries into a markdown embedding corpus",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireProject(project); err != nil {
				return err
			}
			if outDir == "" {
				return fmt.Errorf("--out is required")
			}
			p, err := inventory.LoadProject(*project)
			if err != nil {
				return err
			}
			if codeRoot != "" {
				p.CodeRoot = codeRoot
			}
			res, err := domainexport.Export(p, outDir)
			if err != nil {
				return err
			}
			for _, w := range res.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "domain export: warning: %s\n", w)
			}
			if want := len(p.AuthoritativeDocs); want > res.DocsCopied && !allowMissingDocs {
				return fmt.Errorf("%d of %d authoritative_docs could not be resolved"+
					" (code_root=%q). The corpus would ship without them; pass --code-root, or"+
					" --allow-missing-docs to accept the gap", want-res.DocsCopied, want, p.CodeRoot)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "domain export: %d entries, %d docs -> %s\n", res.EntriesWritten, res.DocsCopied, outDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "output corpus directory")
	cmd.Flags().StringVar(&codeRoot, "code-root", "", "working tree the authoritative_docs resolve against (overrides CKS_CODE_ROOT and project.yaml code_root)")
	cmd.Flags().BoolVar(&allowMissingDocs, "allow-missing-docs", false, "downgrade unresolvable authoritative_docs from an error to a warning")
	return cmd
}

func newSyncCmd(project *string) *cobra.Command {
	var ckvOut, ckgOut string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Derive the ckv/ckg policy views from the verified entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireProject(project); err != nil {
				return err
			}
			return runSync(*project, ckvOut, ckgOut)
		},
	}
	cmd.Flags().StringVar(&ckvOut, "ckv-out", "", "write the ckv view here (default: stdout)")
	cmd.Flags().StringVar(&ckgOut, "ckg-out", "", "write the ckg policy view here (default: stdout)")
	return cmd
}

func newGlossaryGenCmd(project *string) *cobra.Command {
	var outPath, statusGate string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "glossary-gen",
		Short: "Generate the alias glossary from the domain entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireProject(project); err != nil {
				return err
			}
			out := outPath
			if out == "" {
				out = filepath.Join(*project, "glossary.yaml")
			}
			return runGlossaryGen(*project, out, statusGate, dryRun, cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "output glossary.yaml path (default: <project>/glossary.yaml)")
	cmd.Flags().StringVar(&statusGate, "status", "verified", "include only entries with this status (verified | needs_verification | draft | all)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print stats to stderr without writing the output file")
	return cmd
}

func runGlossaryGen(projectDir, outPath, statusGate string, dryRun bool, stderr io.Writer) error {
	entriesDir := filepath.Join(projectDir, "entries")
	files, err := filepath.Glob(filepath.Join(entriesDir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no entries found at %s", entriesDir)
	}
	sort.Strings(files)

	out := glossaryFile{Version: 1}
	var skipped, included int
	for _, f := range files {
		var e entryFile
		buf, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if err := yaml.Unmarshal(buf, &e); err != nil {
			return fmt.Errorf("parse %s: %w", f, err)
		}
		if !statusIncluded(e.Status, statusGate) {
			skipped++
			continue
		}
		ge, ok := buildGlossaryEntry(e)
		if !ok {
			skipped++
			continue
		}
		out.Entries = append(out.Entries, ge)
		included++
	}

	fmt.Fprintf(stderr, "domain glossary-gen: included=%d skipped=%d (gate=%s)\n", included, skipped, statusGate)

	// Round-trip validate: feed the result through vocab.New so any
	// generator bug that produces a malformed glossary surfaces here,
	// not at the next server boot.
	check := vocab.Glossary{Version: out.Version}
	for _, ge := range out.Entries {
		check.Entries = append(check.Entries, vocab.Entry{
			Aliases:      ge.Aliases,
			Canonical:    ge.Canonical,
			CodeKeywords: ge.CodeKeywords,
		})
	}
	if _, err := vocab.New(check); err != nil {
		return fmt.Errorf("vocab.New round-trip failed: %w", err)
	}

	if dryRun {
		return nil
	}

	yamlOut, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	header := "# Generated by cks domain glossary-gen. Do not hand-edit — re-run\n" +
		"# the generator instead. Each glossary entry is sourced from the\n" +
		"# domain knowledge entry named under canonical.\n"
	// The setup plan writes into a derived tree that may not exist yet.
	if dir := filepath.Dir(outPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
	}
	if err := os.WriteFile(outPath, append([]byte(header), yamlOut...), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

func newVerifyCmd(project *string) *cobra.Command {
	var entryRef, reviewer, date string
	var skipInventory bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Promote one entry to status: verified",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireProject(project); err != nil {
				return err
			}
			if entryRef == "" || reviewer == "" {
				return fmt.Errorf("--entry and --by are required")
			}
			if date == "" {
				date = time.Now().UTC().Format("2006-01-02")
			}
			return runVerify(*project, entryRef, reviewer, date, skipInventory, cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&entryRef, "entry", "", "entry id (e.g. A1.wbft_core.quorum_calc) or path to entry YAML")
	cmd.Flags().StringVar(&reviewer, "by", "", "reviewer handle to record under verified_by")
	cmd.Flags().StringVar(&date, "date", "", "verification date YYYY-MM-DD (default: today UTC)")
	cmd.Flags().BoolVar(&skipInventory, "skip-inventory", false, "do not rewrite inventory.md after the promotion")
	return cmd
}

func runVerify(projectDir, entryRef, reviewer, date string, skipInventory bool, stderr io.Writer) error {
	p, err := inventory.LoadProject(projectDir)
	if err != nil {
		return err
	}

	entryID, sourcePath, ok := resolveEntry(p, entryRef)
	if !ok {
		return fmt.Errorf("entry %q not found in project %s", entryRef, p.ID)
	}

	original := p.Entries[entryID]
	mutated := original
	mutated.Status = "verified"
	mutated.LastVerifiedAt = date
	mutated.VerifiedBy = reviewer

	// Simulate the post-promotion project so ValidateEntry sees the
	// new fields. Subsystems and other entries are shared with p, so
	// cross-reference resolution works the same.
	simulated := *p
	simulated.Entries = make(map[string]inventory.Entry, len(p.Entries))
	for k, v := range p.Entries {
		simulated.Entries[k] = v
	}
	simulated.Entries[entryID] = mutated

	issues := inventory.ValidateEntry(&simulated, mutated)
	errCount := 0
	for _, iss := range issues {
		switch iss.Severity {
		case inventory.SeverityError:
			errCount++
			fmt.Fprintf(stderr, "%s: error: %s: %s\n", iss.File, iss.EntryID, iss.Message)
		case inventory.SeverityWarning:
			fmt.Fprintf(stderr, "%s: warning: %s: %s\n", iss.File, iss.EntryID, iss.Message)
		}
	}
	if errCount > 0 {
		return fmt.Errorf("%d errors blocking verified transition; entry unchanged", errCount)
	}

	if err := inventory.MarkVerified(sourcePath, date, reviewer); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "domain verify: %s -> verified (by=%s date=%s)\n", entryID, reviewer, date)

	if skipInventory {
		return nil
	}
	// Reload so the inventory.md update sees the new status. This is
	// cheap (small file count) and avoids any risk of stale in-memory
	// state lagging behind disk after MarkVerified.
	p2, err := inventory.LoadProject(projectDir)
	if err != nil {
		return fmt.Errorf("reload project: %w", err)
	}
	invPath := filepath.Join(p2.Dir, "inventory.md")
	if err := inventory.UpdateInventoryCounts(invPath, p2); err != nil {
		return fmt.Errorf("update inventory.md: %w", err)
	}
	fmt.Fprintf(stderr, "domain verify: refreshed %s\n", invPath)
	return nil
}

func newCheckCmd(project *string) *cobra.Command {
	var updateInventory bool
	var graphPath string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate the inventory (and optionally def-anchor resolution against ckg)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireProject(project); err != nil {
				return err
			}
			return runCheck(*project, updateInventory, graphPath)
		},
	}
	cmd.Flags().BoolVar(&updateInventory, "update-inventory", false, "after validation, rewrite <project>/inventory.md's count tables")
	cmd.Flags().StringVar(&graphPath, "graph", "", "optional ckg graph.db path; when set, assert every def code-anchor symbol resolves uniquely in ckg")
	return cmd
}

func runCheck(projectDir string, updateInventory bool, graphPath string) error {
	p, err := inventory.LoadProject(projectDir)
	if err != nil {
		return err
	}

	issues := inventory.ValidateProject(p)
	errCount, warnCount := reportIssues(os.Stderr, issues)

	// When a ckg graph is provided, assert every def anchor's symbol
	// resolves to exactly one definition. def anchors promise a uniquely
	// resolvable symbol; a 0- or multi-match means the symbol is wrong or
	// too short and would resolve ambiguously at query time.
	if graphPath != "" {
		de, dw := checkDefAnchorResolution(os.Stderr, p, graphPath)
		errCount += de
		warnCount += dw
	}

	fmt.Fprintf(os.Stderr, "domain check: %d entries, %d errors, %d warnings\n",
		len(p.Entries), errCount, warnCount)

	if updateInventory && errCount == 0 {
		invPath := filepath.Join(p.Dir, "inventory.md")
		if err := inventory.UpdateInventoryCounts(invPath, p); err != nil {
			return fmt.Errorf("update inventory.md: %w", err)
		}
		fmt.Fprintf(os.Stderr, "domain check: updated %s\n", invPath)
	} else if updateInventory && errCount > 0 {
		fmt.Fprintln(os.Stderr, "domain check: skipping inventory.md update because errors are present")
	}

	if errCount > 0 {
		return fmt.Errorf("%d errors", errCount)
	}
	return nil
}

func newAnchorsCmd(project *string) *cobra.Command {
	var graphPath string
	var check bool
	var maxShift int
	cmd := &cobra.Command{
		Use:   "anchors",
		Short: "Refresh entry code anchors against the current graph",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireProject(project); err != nil {
				return err
			}
			if graphPath == "" {
				return fmt.Errorf("--graph is required")
			}
			return runAnchors(*project, graphPath, check, maxShift)
		},
	}
	cmd.Flags().StringVar(&graphPath, "graph", "", "path to the ckg graph.db the anchors resolve against")
	cmd.Flags().BoolVar(&check, "check", false, "report drift without writing; fail if any anchor needs attention")
	cmd.Flags().IntVar(&maxShift, "max-shift", 15, "only auto-apply when the line moved by at most this many lines; larger moves are reported as REVIEW")
	return cmd
}

func newWorksheetCmd(project *string) *cobra.Command {
	var status, priority, outPath string
	cmd := &cobra.Command{
		Use:   "worksheet",
		Short: "Emit a promotion worksheet for entries awaiting verification",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireProject(project); err != nil {
				return err
			}
			p, err := inventory.LoadProject(*project)
			if err != nil {
				return err
			}
			entries := filterAndSort(p, status, priority)

			var w io.Writer = cmd.OutOrStdout()
			if outPath != "" {
				f, err := os.Create(outPath)
				if err != nil {
					return err
				}
				defer f.Close()
				w = f
			}

			renderHeader(w, p, status, priority, len(entries))
			for _, e := range entries {
				renderEntry(w, p, e)
			}
			renderFooter(w, p)

			fmt.Fprintf(cmd.ErrOrStderr(), "domain worksheet: %d entries emitted (status=%s priority=%s)\n",
				len(entries), status, displayOrAll(priority))
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "needs_verification", "entry status to include")
	cmd.Flags().StringVar(&priority, "priority", "", "optional priority filter (P0|P1|P2|P3); empty = all")
	cmd.Flags().StringVar(&outPath, "out", "", "output path; empty = stdout")
	return cmd
}
