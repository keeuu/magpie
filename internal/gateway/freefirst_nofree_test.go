package gateway

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// pricedInCatalog gives provider id a model m the catalog prices, so its cost
// is known: not free, but not unknown. liveModels cannot say this — a
// vendor's own list carries whether a model is free, never what it costs.
func pricedInCatalog(t *testing.T, id, m string) {
	t.Helper()
	path := catalog.CachePath()
	doc := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &doc)
	}
	entry, _ := doc[id].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
		doc[id] = entry
	}
	ms, _ := entry["models"].(map[string]any)
	if ms == nil {
		ms = map[string]any{}
		entry["models"] = ms
	}
	ms[m] = map[string]any{"id": m, "name": m, "cost": map[string]any{"input": 3.0, "output": 15.0}}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()

	// the provider must name that catalog: ListPrice reads the catalog's,
	// never the vendor's own list
	found, err := provider.Find(id)
	if err != nil {
		t.Fatal(err)
	}
	found.Catalog = id
	if err := provider.Save(*found); err != nil {
		t.Fatal(err)
	}
}

// A pool routed free first whose members' costs nothing can tell — a vendor
// added with no list fetched yet, and no catalog pricing its model — spends
// the listed order, which is In order. That is the right order to spend: what
// was wrong was that it looked exactly like a pool where every model is
// genuinely paid, so a user who set free first to stop spending had no way to
// know that it had stopped working. The trace says it now, and a pool whose
// models are all known paid stays quiet: there the routing did its job.
func TestFreeFirstSaysWhenNoMemberCostIsKnown(t *testing.T) {
	fresh(t)
	vendor := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"ok","choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	})
	serveOn(t, "p1", "sk-1", []string{"m"}, vendor)
	serveOn(t, "p2", "sk-2", []string{"m"}, vendor)
	// no list and no price for either: nothing says free, nothing says what it costs
	if err := provider.SaveGroup(provider.Group{
		Name: "G", Members: []string{"p1/m", "p2/m"}, Routing: provider.FreeFirst,
	}); err != nil {
		t.Fatal(err)
	}
	s := New()
	code, body := postAs(t, s, "", `{"model":"group/g","messages":[{"role":"user","content":"x"}]}`)
	if code != 200 {
		t.Fatalf("code %d: %s", code, body)
	}
	last := s.trace.routes[len(s.trace.routes)-1]
	if last.Group == nil {
		t.Fatal("the trace has no group")
	}
	if !last.Group.FreeUnknown {
		t.Fatal("free first ordered nothing and the trace doesn't say so: a pool routed free whose members' costs are unknown reads as one where everything is paid")
	}
	for _, w := range last.Order {
		if w.Free {
			t.Fatalf("a row says free where nothing is known: %+v", w)
		}
	}
}

// The same group with both lists fetched, both models clearly paid: the
// routing found nothing free because nothing is, which it does not report as
// unknown. A user who set free first loses nothing there, so saying "can't
// tell" would be a false alarm.
func TestFreeFirstIsQuietWhenEveryMemberIsKnownPaid(t *testing.T) {
	fresh(t)
	vendor := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"ok","choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	})
	serveOn(t, "p1", "sk-1", []string{"m"}, vendor)
	serveOn(t, "p2", "sk-2", []string{"m"}, vendor)
	// each vendor prices the model in the catalog, so its cost is known:
	// not free, but not unknown either
	pricedInCatalog(t, "p1", "m")
	pricedInCatalog(t, "p2", "m")
	if err := provider.SaveGroup(provider.Group{
		Name: "G", Members: []string{"p1/m", "p2/m"}, Routing: provider.FreeFirst,
	}); err != nil {
		t.Fatal(err)
	}
	s := New()
	code, body := postAs(t, s, "", `{"model":"group/g","messages":[{"role":"user","content":"x"}]}`)
	if code != 200 {
		t.Fatalf("code %d: %s", code, body)
	}
	last := s.trace.routes[len(s.trace.routes)-1]
	if last.Group == nil {
		t.Fatal("the trace has no group")
	}
	if last.Group.FreeUnknown {
		t.Fatal("both costs were known and paid; the routing worked, so nothing is unknown")
	}
}

// A pool with a free member in it never reads as unknown, whatever the other
// members are: the routing had something to put first and did.
func TestFreeFirstIsQuietWhenAMemberIsFree(t *testing.T) {
	fresh(t)
	vendor := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"ok","choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	})
	serveOn(t, "freev", "sk-f", []string{"m"}, vendor)
	serveOn(t, "paidv", "sk-p", []string{"m"}, vendor)
	liveModels(t, "freev", `{"models":[{"id":"m","free":true}]}`)
	pricedInCatalog(t, "paidv", "m")
	if err := provider.SaveGroup(provider.Group{
		Name: "G", Members: []string{"paidv/m", "freev/m"}, Routing: provider.FreeFirst,
	}); err != nil {
		t.Fatal(err)
	}
	s := New()
	code, body := postAs(t, s, "", `{"model":"group/g","messages":[{"role":"user","content":"x"}]}`)
	if code != 200 {
		t.Fatalf("code %d: %s", code, body)
	}
	last := s.trace.routes[len(s.trace.routes)-1]
	if last.Group == nil {
		t.Fatal("the trace has no group")
	}
	if last.Group.FreeUnknown {
		t.Fatal("a free member was known, so the routing ordered something")
	}
	if len(last.Order) < 2 || !last.Order[0].Free {
		t.Fatalf("free first didn't go to the free member: %+v", last.Order)
	}
}
