package cache

import (
	"os"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// enforceIsolation is set by the candidate evaluator (SOMA_ENFORCE_ISOLATION=1).
// On the frozen baseline the tenant/config omission in Key is a known defect (F6),
// so the baseline run characterises it (logs it) instead of failing; every
// candidate that claims to fix it runs with enforcement on and must pass.
func enforceIsolation() bool { return os.Getenv("SOMA_ENFORCE_ISOLATION") == "1" }

var keyAt = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func TestKeyStableForIdenticalInputs(t *testing.T) {
	a := Key("tenant-a", "cfg-a", "c1", "SKU-1", 3, "web", keyAt)
	b := Key("tenant-a", "cfg-a", "c1", "SKU-1", 3, "web", keyAt.Add(3*time.Hour))
	if a != b {
		t.Fatalf("same inputs on the same UTC day must give the same key: %s vs %s", a, b)
	}
}

func TestKeyDistinguishesRequestFields(t *testing.T) {
	base := Key("tenant-a", "cfg-a", "c1", "SKU-1", 3, "web", keyAt)
	variants := map[string]string{
		"customer": Key("tenant-a", "cfg-a", "c2", "SKU-1", 3, "web", keyAt),
		"sku":      Key("tenant-a", "cfg-a", "c1", "SKU-2", 3, "web", keyAt),
		"qty":      Key("tenant-a", "cfg-a", "c1", "SKU-1", 4, "web", keyAt),
		"channel":  Key("tenant-a", "cfg-a", "c1", "SKU-1", 3, "store", keyAt),
		"date":     Key("tenant-a", "cfg-a", "c1", "SKU-1", 3, "web", keyAt.AddDate(0, 0, 1)),
	}
	for name, k := range variants {
		if k == base {
			t.Errorf("key does not change when %s changes", name)
		}
	}
}

func TestKeyTenantIsolation(t *testing.T) {
	a := Key("tenant-a", "cfg-a", "c1", "SKU-1", 3, "web", keyAt)
	b := Key("tenant-b", "cfg-a", "c1", "SKU-1", 3, "web", keyAt)
	if a == b {
		if enforceIsolation() {
			t.Fatal("cross-tenant key collision: tenantID is not part of the cache key")
		}
		t.Log("BASELINE DEFECT F6 (characterised): identical keys for different tenants")
	}
}

func TestKeyConfigVersionIsolation(t *testing.T) {
	a := Key("tenant-a", "cfg-1", "c1", "SKU-1", 3, "web", keyAt)
	b := Key("tenant-a", "cfg-2", "c1", "SKU-1", 3, "web", keyAt)
	if a == b {
		if enforceIsolation() {
			t.Fatal("cross-config key collision: configVersion is not part of the cache key")
		}
		t.Log("BASELINE DEFECT F6 (characterised): identical keys for different config versions")
	}
}

func TestCacheBoundedByMaxSize(t *testing.T) {
	const max = 50
	c := New(max, time.Minute)
	for i := 0; i < 500; i++ {
		c.Put(Key("t", "cfg", "c", "SKU", i+1, "web", keyAt), domain.PricingDecision{Quantity: i + 1})
		if c.Size() > max {
			t.Fatalf("cache exceeded maxSize: size=%d max=%d at insert %d", c.Size(), max, i)
		}
	}
	if c.Size() != max {
		t.Fatalf("expected cache to be full at %d, got %d", max, c.Size())
	}
}

func TestCacheGetPutAndDisable(t *testing.T) {
	c := New(10, time.Minute)
	k := Key("t", "cfg", "c", "SKU", 1, "web", keyAt)
	if _, ok := c.Get(k); ok {
		t.Fatal("unexpected hit on empty cache")
	}
	c.Put(k, domain.PricingDecision{AmountJPY: 42})
	if d, ok := c.Get(k); !ok || d.AmountJPY != 42 {
		t.Fatalf("expected hit with 42, got %v %v", d.AmountJPY, ok)
	}
	c.Disable()
	if _, ok := c.Get(k); ok {
		t.Fatal("disabled cache must miss")
	}
}
