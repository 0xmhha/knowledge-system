package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var ToolNameCollectionStatus = toolName("collection.status")
var ToolNameCollectionContext = toolName("collection.get_context")

type CollectionStatus struct {
	State         string                  `json:"state"`
	CatalogDigest string                  `json:"catalog_digest"`
	Collections   []collection.Collection `json:"collections"`
	MemberCount   int                     `json:"member_count"`
	WriteEnabled  bool                    `json:"write_enabled"`
}

type CollectionFactory func(context.Context, collection.Version) (Deps, func(), error)

// CollectionService owns per-version composers and one set of reader leases.
// A factory opens configured backends only after all archive pins are checked.
type CollectionService struct {
	session *collection.Session
	members map[contract.V2Coordinates]Deps
	status  CollectionStatus
	closers []func()
	closed  atomic.Bool
	once    sync.Once
	err     error
}

func NewCollectionService(ctx context.Context, catalog collection.Catalog, cleaner *sanitize.Engine, factory CollectionFactory) (*CollectionService, error) {
	if factory == nil {
		return nil, collection.ErrCatalogInvalid
	}
	raw, err := catalog.CanonicalBytes()
	if err != nil {
		return nil, err
	}
	owned, err := collection.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	session, err := collection.Open(ctx, owned, cleaner)
	if err != nil {
		return nil, err
	}
	s := &CollectionService{session: session, members: map[contract.V2Coordinates]Deps{},
		status: CollectionStatus{State: "configured", CatalogDigest: session.Digest(), Collections: owned.Collections, MemberCount: len(owned.Versions)}}
	for _, version := range owned.Versions {
		deps, closeBackend, err := factory(ctx, version)
		if closeBackend != nil {
			s.closers = append(s.closers, closeBackend)
		}
		if err != nil || deps.Composer == nil || deps.CKG == nil || deps.CKV == nil || deps.Collection != nil ||
			filepath.Clean(deps.EvidenceVersionDir) != version.VersionDir || deps.EvidenceSanitizer != cleaner {
			_ = s.Close()
			return nil, collection.ErrDatasetMismatch
		}
		s.members[version.Coordinates] = deps
	}
	return s, nil
}

func (s *CollectionService) Close() error {
	s.once.Do(func() {
		s.closed.Store(true)
		// Wait for active Session.Read calls before closing backend stores.
		s.err = s.session.Close()
		for i := len(s.closers) - 1; i >= 0; i-- {
			s.closers[i]()
		}
	})
	return s.err
}

func (s *CollectionService) read(ctx context.Context, request collection.Request) (collection.Result, error) {
	if s.closed.Load() {
		return collection.Result{}, collection.ErrClosed
	}
	return s.session.Read(ctx, request, func(ctx context.Context, version collection.Version, query string) (contract.EvidencePackV2, error) {
		deps, ok := s.members[version.Coordinates]
		if !ok {
			return contract.EvidencePackV2{}, collection.ErrScopeMismatch
		}
		var req mcpgo.CallToolRequest
		req.Params.Arguments = map[string]any{"prompt": query}
		result, err := handleGetForTaskV2(ctx, deps, req)
		if err != nil || result == nil || result.IsError {
			return contract.EvidencePackV2{}, collection.ErrFetchFailed
		}
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return contract.EvidencePackV2{}, collection.ErrEvidenceMismatch
		}
		var pack contract.EvidencePackV2
		if err := json.Unmarshal(raw, &pack); err != nil {
			return pack, collection.ErrEvidenceMismatch
		}
		return pack, nil
	})
}

func registerCollection(server *mcpserver.MCPServer, service *CollectionService) {
	server.AddTool(mcpgo.NewTool(ToolNameCollectionStatus, mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDescription("List explicitly configured immutable collection members and catalog digest. Does not expose local paths, enable writes or certify retrieval quality."),
		mcpgo.WithOutputSchema[CollectionStatus]()), func(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		raw, err := json.Marshal(req.GetRawArguments())
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		var empty struct{}
		if err != nil || decoder.Decode(&empty) != nil {
			return collectionToolError(collection.ErrScopeMismatch), nil
		}
		if service.closed.Load() {
			return collectionToolError(collection.ErrClosed), nil
		}
		return mcpgo.NewToolResultStructured(service.status, "configured immutable collection membership"), nil
	})
	server.AddTool(mcpgo.NewTool(ToolNameCollectionContext, mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDescription("Read evidence from explicit immutable members. implementation requires one target; comparison requires 2..16 participants and returns separate groups. Pin catalog_digest from collection.status. No path/current inference, writes, industry mapping or superiority assessment."),
		mcpgo.WithInputSchema[collection.Request](), mcpgo.WithOutputSchema[collection.Result]()), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		raw, err := json.Marshal(req.GetRawArguments())
		if err != nil || len(raw) > 64<<10 {
			return collectionToolError(collection.ErrScopeMismatch), nil
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		var query collection.Request
		if err := decoder.Decode(&query); err != nil {
			return collectionToolError(collection.ErrScopeMismatch), nil
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return collectionToolError(collection.ErrScopeMismatch), nil
		}
		out, err := service.read(ctx, query)
		if err != nil {
			return collectionToolError(err), nil
		}
		return mcpgo.NewToolResultStructured(out, "project-separated retained evidence"), nil
	})
}

func collectionToolError(err error) *mcpgo.CallToolResult {
	code := "operation_failed"
	for name, sentinel := range map[string]error{"scope_mismatch": collection.ErrScopeMismatch,
		"target_required": collection.ErrTargetRequired, "stale_catalog": collection.ErrStaleCatalog,
		"unsupported_mode": collection.ErrUnsupportedMode, "dataset_mismatch": collection.ErrDatasetMismatch,
		"evidence_mismatch": collection.ErrEvidenceMismatch, "closed": collection.ErrClosed,
		"fetch_failed": collection.ErrFetchFailed, "canceled": context.Canceled, "deadline": context.DeadlineExceeded} {
		if errors.Is(err, sentinel) {
			code = name
			break
		}
	}
	result := mcpgo.NewToolResultError("collection request refused: " + code)
	result.StructuredContent = map[string]string{"code": code, "message": "The collection request could not be verified or completed."}
	return result
}
