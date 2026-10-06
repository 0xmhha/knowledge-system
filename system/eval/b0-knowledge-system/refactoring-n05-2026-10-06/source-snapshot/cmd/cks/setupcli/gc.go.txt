package setupcli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/spf13/cobra"
)

func NewGCCmd() *cobra.Command {
	var out, planFile string
	var dryRun, apply, resume bool
	var keep int
	var minAge time.Duration
	var maxBytes int64
	var protect []string
	c := &cobra.Command{Use: "gc", Short: "Review a version retention plan, apply that exact plan, or resume interrupted collection", RunE: func(cmd *cobra.Command, _ []string) error {
		if (apply && resume) || ((apply || resume) && cmd.Flags().Changed("dry-run") && dryRun) {
			return fmt.Errorf("choose dry-run, apply, or resume")
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if resume {
			if err := setup.ResumeGC(ctx, out); err != nil {
				return err
			}
			return encoder.Encode(map[string]string{"status": "resumed"})
		}
		if apply {
			if planFile == "" {
				return fmt.Errorf("apply requires a reviewed plan-file")
			}
			for _, name := range []string{"keep-recent", "min-age", "max-bytes", "protect-version"} {
				if cmd.Flags().Changed(name) {
					return fmt.Errorf("apply takes policy from the reviewed plan; create a fresh dry-run to change it")
				}
			}
			data, err := os.ReadFile(planFile)
			if err != nil {
				return err
			}
			var plan setup.GCPlan
			if err := json.Unmarshal(data, &plan); err != nil {
				return err
			}
			if err := setup.ApplyGC(ctx, out, plan); err != nil {
				return err
			}
			return encoder.Encode(map[string]any{"status": "applied", "plan_digest": plan.Digest, "reclaimed_bytes": plan.ReclaimBytes})
		}
		if !dryRun {
			return fmt.Errorf("use apply with a reviewed plan-file to collect versions")
		}
		plan, err := setup.PlanGC(ctx, out, setup.GCPolicy{KeepRecent: keep, MinAge: minAge, MaxBytes: maxBytes, ProtectVersions: protect}, time.Now())
		if err != nil {
			return err
		}
		if planFile != "" {
			data, err := json.MarshalIndent(plan, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(planFile, append(data, '\n'), 0600); err != nil {
				return err
			}
		}
		return encoder.Encode(plan)
	}}
	c.Flags().StringVar(&out, "out", "", "dataset root")
	c.Flags().StringVar(&planFile, "plan-file", "", "write a dry-run plan here, or load the reviewed plan for apply")
	c.Flags().BoolVar(&dryRun, "dry-run", true, "report retention and protection without deleting versions (default)")
	c.Flags().BoolVar(&apply, "apply", false, "apply exactly the reviewed plan-file; stale references or bytes reject it")
	c.Flags().BoolVar(&resume, "resume", false, "verify and resume only interrupted GC journals")
	c.Flags().IntVar(&keep, "keep-recent", 2, "minimum number of most recently written versions to retain (at least 2)")
	c.Flags().DurationVar(&minAge, "min-age", 30*24*time.Hour, "minimum version age eligible for collection")
	c.Flags().Int64Var(&maxBytes, "max-bytes", 0, "capacity target in bytes; protected versions can keep usage above it")
	c.Flags().StringSliceVar(&protect, "protect-version", nil, "explicit rollback or other version labels to retain")
	_ = c.MarkFlagRequired("out")
	return c
}
