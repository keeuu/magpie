package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// liveFree writes the list a vendor's /models answered, where
// catalog.Live reads it: what Available goes by, and so FreeModel.
func liveFree(t *testing.T, id, json string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(catalog.LivePath(id)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog.LivePath(id), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
}

// FreeModel is what Free first goes by: a model its vendor lists at no cost
// is free, one the vendor prices is not, and a model the list doesn't name
// is not either — nothing says it costs nothing, so Free first puts it with
// the paid ones rather than inventing a saving.
func TestFreeModelGoesByTheVendorsOwnList(t *testing.T) {
	prefsHome(t)
	p := Provider{ID: "fv", Name: "FV", Key: "sk-fv", Chat: "http://127.0.0.1:9/v1"}
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	liveFree(t, "fv", `{"models":[
		{"id":"free-one","free":true},
		{"id":"paid-one","cost":{"input":3,"output":15}},
		{"id":"zero-priced"}
	]}`)
	got, err := Find("fv")
	if err != nil {
		t.Fatal(err)
	}
	if !got.FreeModel("free-one") {
		t.Error("a model its vendor lists free isn't free")
	}
	if got.FreeModel("paid-one") {
		t.Error("a model its vendor prices as paid is free")
	}
	if got.FreeModel("nobody-lists-this") {
		t.Error("a model the list doesn't name is free")
	}
}

// The vendor's own word is what decides it, however many models it so
// marks: a pool where some are free and some are not puts only the marked
// ones first, and one where none is marked behaves as In order.
func TestFreeModelIsOnlyWhereTheVendorSaysSo(t *testing.T) {
	prefsHome(t)
	p := Provider{ID: "mv", Name: "MV", Key: "sk-mv", Chat: "http://127.0.0.1:9/v1"}
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	liveFree(t, "mv", `{"models":[{"id":"marked","free":true},{"id":"plain"},{"id":"also-marked","free":true}]}`)
	got, err := Find("mv")
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"marked", "also-marked"} {
		if !got.FreeModel(model) {
			t.Errorf("%s isn't free", model)
		}
	}
	if got.FreeModel("plain") {
		t.Error("plain is free")
	}
}

// FreeModelKnown is FreeModel and whether p could tell at all. A provider
// whose list nothing has fetched and whose model no catalog prices answers
// false, false — unknown, not paid — and that is what keeps a pool routed
// free first from reading as one where everything costs money. The vendor's
// own list, once fetched, and a priced catalog both settle it.
func TestFreeModelKnownTellsUnknownFromPaid(t *testing.T) {
	prefsHome(t)
	p := Provider{ID: "unk", Name: "Unk", Key: "sk-u", Chat: "http://127.0.0.1:9/v1", Models: []string{"m"}}
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Find("unk")
	if err != nil {
		t.Fatal(err)
	}
	// nothing fetched, and no catalog pricing it
	if _, ok := got.Listed(); ok {
		t.Fatal("this test needs a provider no list was fetched for")
	}
	if _, priced := got.ListPrice("m"); priced {
		t.Fatal("this test needs a model no catalog prices")
	}
	free, known := got.FreeModelKnown("m")
	if free || known {
		t.Fatalf("unknown reads as free=%v known=%v; it must be false, false", free, known)
	}
	// FreeModel keeps answering false for it: unknown is not a saving
	if got.FreeModel("m") {
		t.Fatal("FreeModel says an unknown model is free")
	}

	// the vendor says what it costs: known, and paid
	liveFree(t, "unk", `{"models":[{"id":"m","price":{"input":3,"output":15}}]}`)
	fresh, err := Find("unk")
	if err != nil {
		t.Fatal(err)
	}
	if free, known := fresh.FreeModelKnown("m"); free || known {
		t.Fatalf("a priced model reads as free=%v known=%v; the price is not what FreeModelKnown reads", free, known)
	}

	// the vendor says it is free: known, and free
	liveFree(t, "unk", `{"models":[{"id":"m","free":true}]}`)
	fresh, err = Find("unk")
	if err != nil {
		t.Fatal(err)
	}
	if free, known := fresh.FreeModelKnown("m"); !free || !known {
		t.Fatalf("a model its vendor marks free reads as free=%v known=%v", free, known)
	}
}

// The catalog's price settles it too, for a model no vendor list carries:
// known, and paid unless every count is zero.
func TestFreeModelKnownReadsACatalogPrice(t *testing.T) {
	prefsHome(t)
	p := Provider{ID: "cat", Name: "Cat", Catalog: "cat", Key: "sk-c", Chat: "http://127.0.0.1:9/v1", Models: []string{"priced-m", "zero-m"}}
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	writeCatalog(t, `{"cat":{"models":{
		"priced-m":{"id":"priced-m","name":"Priced","cost":{"input":3,"output":15}},
		"zero-m":{"id":"zero-m","name":"Zero","cost":{}}
	}}}`)
	got, err := Find("cat")
	if err != nil {
		t.Fatal(err)
	}
	if free, known := got.FreeModelKnown("priced-m"); free || !known {
		t.Fatalf("a cataloged price reads as free=%v known=%v; it is known and paid", free, known)
	}
	if free, known := got.FreeModelKnown("zero-m"); !free || !known {
		t.Fatalf("a catalog price of zero on every count reads as free=%v known=%v", free, known)
	}
}

// writeCatalog replaces the model catalog the tests price against.
func writeCatalog(t *testing.T, json string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog.CachePath(), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)
}
