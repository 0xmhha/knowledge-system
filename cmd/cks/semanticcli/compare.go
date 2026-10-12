package semanticcli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	"github.com/spf13/cobra"
)

func newCompareResponsesCmd() *cobra.Command {
	var off, on string
	cmd := &cobra.Command{Use: "compare-responses", Short: "Compare saved off/on v2 outputs without claiming quality or latency", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		a, err := readComparisonPack(off)
		if err != nil {
			return fmt.Errorf("read off response: %w", err)
		}
		b, err := readComparisonPack(on)
		if err != nil {
			return fmt.Errorf("read on response: %w", err)
		}
		result, err := evidencev2.CompareResponses(a, b)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}}
	cmd.Flags().StringVar(&off, "off", "", "saved off EvidencePackV2 JSON (up to 4 MiB)")
	cmd.Flags().StringVar(&on, "on", "", "saved on EvidencePackV2 JSON (up to 4 MiB)")
	_ = cmd.MarkFlagRequired("off")
	_ = cmd.MarkFlagRequired("on")
	return cmd
}

func readComparisonPack(path string) (contract.EvidencePackV2, error) {
	var pack contract.EvidencePackV2
	f, err := os.Open(path)
	if err != nil {
		return pack, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return pack, fmt.Errorf("response must be a regular file up to 4 MiB")
	}
	d := json.NewDecoder(io.LimitReader(f, (4<<20)+1))
	d.UseNumber()
	if err := d.Decode(&pack); err != nil {
		return pack, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return pack, fmt.Errorf("response must contain exactly one JSON value")
	}
	return pack, nil
}
