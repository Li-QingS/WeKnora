package service

import (
	"sort"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// TestMergeCitationsIntoItems_PopulatesSourceChunksOnCandidates verifies that
// citations returned by the chunk-classification pass are attached back onto
// the matching candidate items while non-cited candidates are left untouched.
func TestMergeCitationsIntoItems_PopulatesSourceChunksOnCandidates(t *testing.T) {
	entities := []extractedItem{
		{Name: "Acme", Slug: "entity/acme"},
		{Name: "Beta", Slug: "entity/beta"},
	}
	concepts := []extractedItem{
		{Name: "RAG", Slug: "concept/rag"},
	}
	citations := map[string][]string{
		"entity/acme": {"chunk-1", "chunk-3"},
		"concept/rag": {"chunk-2"},
	}

	gotE, gotC, uncited := mergeCitationsIntoItems(entities, concepts, citations, nil)

	if len(gotE) != 2 || len(gotC) != 1 {
		t.Fatalf("expected 2 entities + 1 concept, got %d + %d", len(gotE), len(gotC))
	}
	acme := findBySlug(gotE, "entity/acme")
	if acme == nil {
		t.Fatalf("entity/acme missing from result")
	}
	if !equalStrings(acme.SourceChunks, []string{"chunk-1", "chunk-3"}) {
		t.Errorf("entity/acme source_chunks = %v, want [chunk-1 chunk-3]", acme.SourceChunks)
	}
	beta := findBySlug(gotE, "entity/beta")
	if beta == nil {
		t.Fatalf("entity/beta missing from result")
	}
	if len(beta.SourceChunks) != 0 {
		t.Errorf("entity/beta should have no citations, got %v", beta.SourceChunks)
	}
	rag := findBySlug(gotC, "concept/rag")
	if rag == nil {
		t.Fatalf("concept/rag missing")
	}
	if !equalStrings(rag.SourceChunks, []string{"chunk-2"}) {
		t.Errorf("concept/rag source_chunks = %v, want [chunk-2]", rag.SourceChunks)
	}
	if uncited != 1 {
		t.Errorf("uncited = %d, want 1", uncited)
	}
}

// TestMergeCitationsIntoItems_AddsNewSlugsAndUnionsChunksAcrossBatches checks
// that genuinely new slugs (ones Pass 0 missed) are appended to the right
// type slice, and that a slug surfacing in two batches ends up with the union
// of its source chunks.
func TestMergeCitationsIntoItems_AddsNewSlugsAndUnionsChunksAcrossBatches(t *testing.T) {
	entities := []extractedItem{
		{Name: "Known", Slug: "entity/known"},
	}
	concepts := []extractedItem{}

	newSlugs := []newSlugFromCitation{
		{
			Type:         "entity",
			Name:         "Fresh Entity",
			Slug:         "entity/fresh",
			Description:  "desc",
			Details:      "details",
			SourceChunks: []string{"c001", "c002"},
		},
		{
			// Same slug as above, appears in another batch — must union.
			Type:         "entity",
			Name:         "Fresh Entity",
			Slug:         "entity/fresh",
			SourceChunks: []string{"c002", "c003"},
		},
		{
			Type:         "concept",
			Name:         "New Concept",
			Slug:         "concept/new-concept",
			SourceChunks: []string{"c010"},
		},
		{
			// Duplicate of an existing candidate — should NOT produce a
			// duplicate entry (Known already exists in `entities`).
			Type:         "entity",
			Name:         "Known",
			Slug:         "entity/known",
			SourceChunks: []string{"c020"},
		},
	}

	gotE, gotC, _ := mergeCitationsIntoItems(entities, concepts, nil, newSlugs)

	if len(gotE) != 2 {
		t.Fatalf("expected 2 entities, got %d (%+v)", len(gotE), gotE)
	}
	if len(gotC) != 1 {
		t.Fatalf("expected 1 concept, got %d (%+v)", len(gotC), gotC)
	}
	fresh := findBySlug(gotE, "entity/fresh")
	if fresh == nil {
		t.Fatalf("entity/fresh missing")
	}
	sort.Strings(fresh.SourceChunks)
	if !equalStrings(fresh.SourceChunks, []string{"c001", "c002", "c003"}) {
		t.Errorf("entity/fresh source_chunks = %v, want union [c001 c002 c003]", fresh.SourceChunks)
	}
	newC := findBySlug(gotC, "concept/new-concept")
	if newC == nil || !equalStrings(newC.SourceChunks, []string{"c010"}) {
		t.Errorf("concept/new-concept missing or wrong chunks: %+v", newC)
	}
}

// TestSplitChunksIntoCitationBatches_RespectsBudgetAndOrder verifies that the
// batcher never puts too many runes in one batch, preserves document order,
// and that an oversized chunk gets its own batch.
func TestSplitChunksIntoCitationBatches_RespectsBudgetAndOrder(t *testing.T) {
	// Each small chunk is 5k runes → 3 of them should fit in one batch
	// (15k > 12k limit would spill to a second batch).
	mk := func(idx int, runes int, id string) *types.Chunk {
		return &types.Chunk{
			ID:         id,
			ChunkIndex: idx,
			Content:    repeatRune('a', runes),
			ChunkType:  types.ChunkTypeText,
		}
	}
	chunks := []*types.Chunk{
		mk(0, 5000, "id-0"),
		mk(1, 5000, "id-1"),
		mk(2, 5000, "id-2"), // this should start a new batch (15k > 12k)
		// An oversized chunk gets a dedicated batch.
		mk(3, 20000, "id-big"),
		mk(4, 1000, "id-small"),
	}
	batches := splitChunksIntoCitationBatches(chunks)
	if len(batches) < 3 {
		t.Fatalf("expected at least 3 batches, got %d", len(batches))
	}
	// All input IDs should show up in some batch, exactly once, in order.
	seen := []string{}
	for _, b := range batches {
		for _, c := range b.chunks {
			seen = append(seen, c.ID)
		}
	}
	wantOrder := []string{"id-0", "id-1", "id-2", "id-big", "id-small"}
	if !equalStrings(seen, wantOrder) {
		t.Errorf("batch order = %v, want %v", seen, wantOrder)
	}

	// Verify the typed handle table is populated per batch.
	for bi, b := range batches {
		if b.handles.Len() != len(b.chunks) {
			t.Errorf("batch %d handle count %d != chunk count %d", bi, b.handles.Len(), len(b.chunks))
		}
	}
}

// --- helpers ---

func findBySlug(items []extractedItem, slug string) *extractedItem {
	for i := range items {
		if items[i].Slug == slug {
			return &items[i]
		}
	}
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func repeatRune(r rune, n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}

// TestFormatPreviousSlugsDeterministic verifies that the previous-slugs block
// rendered into extraction prompts is byte-stable across iterations: map
// iteration order is randomized on every range, so an unsorted implementation
// would produce different prompt bytes between runs and break prompt-prefix
// caching.
func TestFormatPreviousSlugsDeterministic(t *testing.T) {
	slugs := map[string]bool{
		"concept/机器学习":     true,
		"entity/zhang-san": true,
		"entity/alibaba":   true,
		"summary/abc":      true, // filtered out
		"page/other":       true, // filtered out
	}
	want := "- concept/机器学习\n- entity/alibaba\n- entity/zhang-san\n"
	first := formatPreviousSlugs(slugs)
	if first != want {
		t.Fatalf("got %q, want %q", first, want)
	}
	for i := 0; i < 100; i++ {
		if got := formatPreviousSlugs(slugs); got != first {
			t.Fatalf("formatPreviousSlugs not deterministic on iteration %d:\ngot:  %q\nwant: %q", i, got, first)
		}
	}
}

func TestFormatPreviousSlugsEmpty(t *testing.T) {
	const none = "(none — this is a new document)"
	if got := formatPreviousSlugs(nil); got != none {
		t.Errorf("nil map: got %q", got)
	}
	if got := formatPreviousSlugs(map[string]bool{}); got != none {
		t.Errorf("empty map: got %q", got)
	}
	if got := formatPreviousSlugs(map[string]bool{"summary/abc": true}); got != none {
		t.Errorf("only filtered-out slugs: got %q", got)
	}
}

func TestWikiPageRegenEligible(t *testing.T) {
	page := &types.WikiPage{}
	if !wikiPageRegenEligible(page, 2, 0) {
		t.Error("legacy (empty edit source) page with additions should be eligible")
	}
	page.LastEditSource = types.WikiEditSourcePipeline
	if !wikiPageRegenEligible(page, 1, 0) {
		t.Error("pipeline-authored page with additions should be eligible")
	}
	page.LastEditSource = types.WikiEditSourceUser
	if wikiPageRegenEligible(page, 1, 0) {
		t.Error("user-edited page must not be regenerated")
	}
	page.LastEditSource = types.WikiEditSourceAgent
	if wikiPageRegenEligible(page, 1, 0) {
		t.Error("agent-edited page must not be regenerated")
	}
	page.LastEditSource = ""
	if wikiPageRegenEligible(page, 1, 1) {
		t.Error("retract updates stay on the legacy merge path")
	}
	if wikiPageRegenEligible(page, 0, 0) {
		t.Error("no additions means nothing to regenerate from")
	}
}

func TestRenderRegenSourceBlockDeterministic(t *testing.T) {
	chunks := []*types.Chunk{
		{ID: "c3", KnowledgeID: "kb1", ChunkIndex: 2, Content: "third"},
		{ID: "c1", KnowledgeID: "kb1", ChunkIndex: 0, Content: "first"},
		{ID: "c2", KnowledgeID: "kb1", ChunkIndex: 1, Content: "second"},
		{ID: "d1", KnowledgeID: "kb2", ChunkIndex: 0, Content: "other doc"},
		{ID: "c4", KnowledgeID: "kb1", ChunkIndex: 5, Content: "   "}, // blank, dropped
	}
	titles := map[string]string{"kb1": "Doc One", "kb2": "Doc Two"}
	want := "<document>\n<title>Doc One</title>\n<content>\n[chunk 0]\nfirst\n</content>\n</document>\n\n" +
		"<document>\n<title>Doc One</title>\n<content>\n[chunk 1]\nsecond\n</content>\n</document>\n\n" +
		"<document>\n<title>Doc One</title>\n<content>\n[chunk 2]\nthird\n</content>\n</document>\n\n" +
		"<document>\n<title>Doc Two</title>\n<content>\n[chunk 0]\nother doc\n</content>\n</document>\n\n"
	first := renderRegenSourceBlock(chunks, titles, nil)
	if first != want {
		t.Fatalf("got %q, want %q", first, want)
	}
	// Repeated rendering must be byte-identical so regeneration prompts stay
	// stable across runs.
	for i := 0; i < 50; i++ {
		if got := renderRegenSourceBlock(chunks, titles, nil); got != first {
			t.Fatalf("renderRegenSourceBlock not deterministic on iteration %d", i)
		}
	}
}
