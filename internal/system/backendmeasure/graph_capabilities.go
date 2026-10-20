package backendmeasure

import "github.com/0xmhha/knowledge-system/internal/system/ckgclient"

// preserveGraphCapabilities keeps the exact optional method set of the provider.
// Embedding Client alone would hide retained analysis from MCP when measurement
// is enabled; embedding every capability would invent support on older backends.
func preserveGraphCapabilities(g *graph, base ckgclient.Client) ckgclient.Client {
	mask := 0
	resolver, ok := base.(ckgclient.CitationResolver)
	if ok {
		mask |= 1
	}
	analysis, ok := base.(ckgclient.ValueAnalysisClient)
	if ok {
		mask |= 2
	}
	expansion, ok := base.(ckgclient.ValueExpansionClient)
	if ok {
		mask |= 4
	}
	annotation, ok := base.(ckgclient.AnnotationClient)
	if ok {
		mask |= 8
	}
	if resolver != nil {
		resolver = &graphWithResolver{graph: g, resolver: resolver}
	}
	switch mask {
	case 1:
		return &struct {
			*graph
			ckgclient.CitationResolver
		}{g, resolver}
	case 2:
		return &struct {
			*graph
			ckgclient.ValueAnalysisClient
		}{g, analysis}
	case 3:
		return &struct {
			*graph
			ckgclient.CitationResolver
			ckgclient.ValueAnalysisClient
		}{g, resolver, analysis}
	case 4:
		return &struct {
			*graph
			ckgclient.ValueExpansionClient
		}{g, expansion}
	case 5:
		return &struct {
			*graph
			ckgclient.CitationResolver
			ckgclient.ValueExpansionClient
		}{g, resolver, expansion}
	case 6:
		return &struct {
			*graph
			ckgclient.ValueAnalysisClient
			ckgclient.ValueExpansionClient
		}{g, analysis, expansion}
	case 7:
		return &struct {
			*graph
			ckgclient.CitationResolver
			ckgclient.ValueAnalysisClient
			ckgclient.ValueExpansionClient
		}{g, resolver, analysis, expansion}
	case 8:
		return &struct {
			*graph
			ckgclient.AnnotationClient
		}{g, annotation}
	case 9:
		return &struct {
			*graph
			ckgclient.CitationResolver
			ckgclient.AnnotationClient
		}{g, resolver, annotation}
	case 10:
		return &struct {
			*graph
			ckgclient.ValueAnalysisClient
			ckgclient.AnnotationClient
		}{g, analysis, annotation}
	case 11:
		return &struct {
			*graph
			ckgclient.CitationResolver
			ckgclient.ValueAnalysisClient
			ckgclient.AnnotationClient
		}{g, resolver, analysis, annotation}
	case 12:
		return &struct {
			*graph
			ckgclient.ValueExpansionClient
			ckgclient.AnnotationClient
		}{g, expansion, annotation}
	case 13:
		return &struct {
			*graph
			ckgclient.CitationResolver
			ckgclient.ValueExpansionClient
			ckgclient.AnnotationClient
		}{g, resolver, expansion, annotation}
	case 14:
		return &struct {
			*graph
			ckgclient.ValueAnalysisClient
			ckgclient.ValueExpansionClient
			ckgclient.AnnotationClient
		}{g, analysis, expansion, annotation}
	case 15:
		return &struct {
			*graph
			ckgclient.CitationResolver
			ckgclient.ValueAnalysisClient
			ckgclient.ValueExpansionClient
			ckgclient.AnnotationClient
		}{g, resolver, analysis, expansion, annotation}
	default:
		return g
	}
}
