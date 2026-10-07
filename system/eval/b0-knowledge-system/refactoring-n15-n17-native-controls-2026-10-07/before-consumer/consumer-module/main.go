// This is compiled in a separate module. It imports only the public contract.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func validate(raw json.RawMessage, format string) error {
	if format == "v1" {
		var p contract.EvidencePack
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if !p.IsValid() || len(p.Citations) == 0 {
			return errors.New("invalid legacy public pack")
		}
		ok, err := contract.VerifyIntegrity(p)
		if err != nil || !ok {
			return errors.New("legacy integrity mismatch")
		}
		return nil
	}
	var p contract.EvidencePackV2
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	c := p.Coordinates
	if p.FormatVersion != 2 || c.ProjectID == "" || len(c.DatasetID) != 64 || len(c.SnapshotID) != 64 || len(p.Citations) == 0 || len(p.Citations) > 12 || len(p.Metadata.IntegrityHash) != 64 || p.Metadata.IntegrityHashAlgo != "sha256-v2" {
		return errors.New("invalid public v2 identity")
	}
	if p.EvidenceState != "complete" && p.EvidenceState != "partial" {
		return errors.New("invalid v2 state")
	}
	seen := map[contract.CitationKeyV2]bool{}
	cited := map[contract.CitationV2]bool{}
	for _, ref := range p.Citations {
		if ref.ProjectID != c.ProjectID || ref.DatasetID != c.DatasetID || ref.SnapshotID != c.SnapshotID || ref.BaseCommit != c.BaseCommit || ref.SourceMode != c.SourceMode || ref.StartLine < 1 || ref.EndLine < ref.StartLine || len(ref.FileSHA256) != 64 || len(ref.ContentSHA256) != 64 || seen[ref.KeyV2()] {
			return errors.New("invalid public v2 citation")
		}
		seen[ref.KeyV2()] = true
		cited[ref] = true
	}
	size := 0
	for _, b := range p.Bodies {
		if !cited[b.Citation] {
			return errors.New("uncited public v2 body")
		}
		size += len(b.Text)
	}
	if size > 32000 || (p.EvidenceState == "complete" && len(p.Bodies) != len(p.Citations)) {
		return errors.New("invalid public v2 budget/completeness")
	}
	// Raw v2 JCS integrity and retained bytes are independently audited in Python.
	return nil
}

func run() error {
	binary := flag.String("binary", "", "installed cks executable")
	config := flag.String("config", "", "stdio configuration")
	prompt := flag.String("prompt", "Where is the committed guide?", "structural probe only")
	format := flag.String("format", "v1", "v1 or v2")
	input := flag.String("pack-file", "", "offline public decoding control")
	flag.Parse()
	if *format != "v1" && *format != "v2" {
		return errors.New("unsupported format")
	}
	if *input != "" {
		raw, err := os.ReadFile(*input)
		if err != nil {
			return err
		}
		if err = validate(raw, *format); err != nil {
			return err
		}
		_, err = os.Stdout.Write(append(raw, '\n'))
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, *binary, "mcp", "--config", *config)
	cmd.WaitDelay = 3 * time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return err
	}
	defer func() { in.Close(); cancel(); cmd.Wait() }()
	enc, dec := json.NewEncoder(in), json.NewDecoder(out)
	request := func(id int, method string, params any) (json.RawMessage, error) {
		if err := enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for {
			var msg struct {
				ID     json.RawMessage `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := dec.Decode(&msg); err != nil {
				return nil, err
			}
			if string(msg.ID) != fmt.Sprint(id) {
				continue
			}
			if len(msg.Error) > 0 && !bytes.Equal(msg.Error, []byte("null")) {
				return nil, errors.New("MCP JSON-RPC error")
			}
			return msg.Result, nil
		}
	}
	_, err = request(1, "initialize", map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "external-public-contract-consumer", "version": "1"}})
	if err != nil {
		return err
	}
	if err = enc.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return err
	}
	tool := "cks.context.get_for_task"
	if *format == "v2" {
		tool += "_v2"
	}
	result, err := request(2, "tools/call", map[string]any{"name": tool, "arguments": map[string]string{"prompt": *prompt}})
	if err != nil {
		return err
	}
	var call struct {
		IsError          bool            `json:"isError"`
		Structured       json.RawMessage `json:"structuredContent"`
		LegacyStructured json.RawMessage `json:"structured_content"`
	}
	if err = json.Unmarshal(result, &call); err != nil {
		return err
	}
	if call.IsError {
		return errors.New("MCP tool error")
	}
	raw := call.Structured
	if len(raw) == 0 {
		raw = call.LegacyStructured
	}
	if err = validate(raw, *format); err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(raw, '\n'))
	return err
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
