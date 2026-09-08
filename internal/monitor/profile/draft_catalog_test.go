package profile

import (
	"path/filepath"
	"testing"

	"github.com/paulohspred/Gateway/internal/monitor"
)

func TestDraftControllerCatalogLoadsAllReferenceFamilies(t *testing.T) {
	path := filepath.Join("..", "..", "..", "controllers", "DRAFT_PROFILES.json")
	bundles, err := LoadDraftCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 9 {
		t.Fatalf("expected 9 draft controller families, got %d", len(bundles))
	}
	for id, bundle := range bundles {
		if bundle.Manifest.Status != StatusDraft {
			t.Fatalf("%s: expected draft status, got %s", id, bundle.Manifest.Status)
		}
		if bundle.Manifest.Capabilities.RemoteControl {
			t.Fatalf("%s: remote control must remain disabled", id)
		}
		if err := bundle.Validate(); err != nil {
			t.Fatalf("%s: invalid generated bundle: %v", id, err)
		}
	}
}

func TestDraftCatalogRejectsUnknownMetric(t *testing.T) {
	catalog := DraftCatalog{
		Schema: DraftCatalogSchemaVersion,
		Profiles: []DraftController{{
			ID:           "test.bad",
			Manufacturer: "Test",
			Model:        "Bad",
			DisplayName:  "Bad Profile",
			Metrics:      []monitor.MetricKey{"vendor.private"},
		}},
	}
	if _, err := catalog.Bundles(); err == nil {
		t.Fatal("expected unknown metric to fail")
	}
}
