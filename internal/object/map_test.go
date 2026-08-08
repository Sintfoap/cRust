package object

import (
	"strings"
	"testing"
)

func TestMapSetGet(t *testing.T) {
	m := NewMap()

	ok := m.Set(&String{Value: "a"}, NewInteger(1))
	if !ok {
		t.Fatal("Set with a String key should succeed")
	}

	value, ok := m.Get(&String{Value: "a"})
	if !ok {
		t.Fatal("Get should find a key equal by value, even as a different Go object")
	}
	if value.(*Integer).Value != 1 {
		t.Errorf("Get returned %v, want 1", value)
	}

	_, ok = m.Get(&String{Value: "missing"})
	if ok {
		t.Error("Get should report not-found for an absent key")
	}
}

func TestMapIntegerKeysDistinctFromStringKeys(t *testing.T) {
	m := NewMap()
	m.Set(NewInteger(1), &String{Value: "int one"})
	m.Set(&String{Value: "1"}, &String{Value: "string one"})

	if len(m.Pairs) != 2 {
		t.Fatalf("Integer(1) and String(\"1\") collided into %d pair(s), want 2", len(m.Pairs))
	}

	iv, _ := m.Get(NewInteger(1))
	sv, _ := m.Get(&String{Value: "1"})
	if iv.(*String).Value != "int one" || sv.(*String).Value != "string one" {
		t.Error("Integer and String keys that print the same must not collide")
	}
}

func TestMapIsAReferenceType(t *testing.T) {
	original := NewMap()
	alias := original
	alias.Set(&String{Value: "k"}, NewInteger(1))

	if _, ok := original.Get(&String{Value: "k"}); !ok {
		t.Error("mutating through alias didn't affect original")
	}
}

func TestMapSetNonHashableKeyFails(t *testing.T) {
	m := NewMap()
	ok := m.Set(NewList(nil), NewInteger(1))
	if ok {
		t.Error("Set with a non-Hashable key (List) should fail, not silently store")
	}
	if len(m.Pairs) != 0 {
		t.Error("a failed Set must not leave a partial entry behind")
	}
}

func TestMapGetNonHashableKey(t *testing.T) {
	m := NewMap()
	m.Set(&String{Value: "a"}, NewInteger(1))

	_, ok := m.Get(NewList(nil))
	if ok {
		t.Error("Get with a non-Hashable key (List) should report not-found, not panic or match anything")
	}
}

func TestMapInspect(t *testing.T) {
	m := NewMap()
	m.Set(&String{Value: "a"}, NewInteger(1))

	got := m.Inspect()
	want := `{a: 1}`
	if got != want {
		t.Errorf("Inspect() = %q, want %q", got, want)
	}
}

func TestMapInspectMultipleEntries(t *testing.T) {
	// Go map iteration order is randomized, so this can't assert an
	// exact string — just that both entries and the separator show up.
	m := NewMap()
	m.Set(&String{Value: "a"}, NewInteger(1))
	m.Set(&String{Value: "b"}, NewInteger(2))

	got := m.Inspect()
	for _, want := range []string{"a: 1", "b: 2", ", "} {
		if !strings.Contains(got, want) {
			t.Errorf("Inspect() = %q, missing %q", got, want)
		}
	}
}

func TestEmptyMapInspect(t *testing.T) {
	if got := NewMap().Inspect(); got != "{}" {
		t.Errorf("Inspect() = %q, want %q", got, "{}")
	}
}

func TestMapDelete(t *testing.T) {
	m := NewMap()
	m.Set(&String{Value: "a"}, NewInteger(1))

	value, removed := m.Delete(&String{Value: "a"})
	if !removed {
		t.Fatal("Delete should report removed for a present key")
	}
	if value.(*Integer).Value != 1 {
		t.Errorf("Delete returned %v, want the removed value 1", value)
	}
	if len(m.Pairs) != 0 {
		t.Errorf("Delete left %d pair(s) behind, want 0", len(m.Pairs))
	}
	if _, ok := m.Get(&String{Value: "a"}); ok {
		t.Error("deleted key should no longer be found by Get")
	}
}

func TestMapDeleteMissingKey(t *testing.T) {
	m := NewMap()
	_, removed := m.Delete(&String{Value: "missing"})
	if removed {
		t.Error("Delete should report not-removed for an absent key")
	}
}

func TestMapDeleteNonHashableKey(t *testing.T) {
	m := NewMap()
	_, removed := m.Delete(NewList(nil))
	if removed {
		t.Error("Delete with a non-Hashable key (List) should report not-removed, not panic")
	}
}

func TestMapOverwrite(t *testing.T) {
	m := NewMap()
	m.Set(&String{Value: "a"}, NewInteger(1))
	m.Set(&String{Value: "a"}, NewInteger(2))

	if len(m.Pairs) != 1 {
		t.Fatalf("re-Set on the same key should overwrite, not add: %d pairs", len(m.Pairs))
	}
	v, _ := m.Get(&String{Value: "a"})
	if v.(*Integer).Value != 2 {
		t.Errorf("Get after overwrite = %v, want 2", v)
	}
}
