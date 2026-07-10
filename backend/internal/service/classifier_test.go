package service

import "testing"

func TestClassifierRegisteredDestinationIsInternal(t *testing.T) {
	classifier, err := NewClassifier([]string{"10.0.0.0/8"}, ScopeExternalPrivate)
	if err != nil {
		t.Fatal(err)
	}

	if got := classifier.Scope("tenant-a", "tenant-a", true, "10.20.20.199"); got != ScopeInternalSameTenant {
		t.Fatalf("same tenant scope = %s, want %s", got, ScopeInternalSameTenant)
	}
	if got := classifier.Scope("tenant-a", "tenant-b", true, "10.20.20.199"); got != ScopeInternalCrossTenant {
		t.Fatalf("cross tenant scope = %s, want %s", got, ScopeInternalCrossTenant)
	}
}

func TestClassifierUnregisteredPrivateDestinationDefaultsToExternalPrivate(t *testing.T) {
	classifier, err := NewClassifier([]string{"10.0.0.0/8"}, ScopeExternalPrivate)
	if err != nil {
		t.Fatal(err)
	}

	if got := classifier.Scope("tenant-a", "", false, "10.30.30.10"); got != ScopeExternalPrivate {
		t.Fatalf("unregistered private scope = %s, want %s", got, ScopeExternalPrivate)
	}
}

func TestClassifierCanKeepUnknownInternalDiscoveryMode(t *testing.T) {
	classifier, err := NewClassifier([]string{"10.0.0.0/8"}, ScopeUnknownInternal)
	if err != nil {
		t.Fatal(err)
	}

	if got := classifier.Scope("tenant-a", "", false, "10.30.30.10"); got != ScopeUnknownInternal {
		t.Fatalf("discovery scope = %s, want %s", got, ScopeUnknownInternal)
	}
}

func TestClassifierUnregisteredPublicDestinationIsExternalPublic(t *testing.T) {
	classifier, err := NewClassifier([]string{"10.0.0.0/8"}, ScopeExternalPrivate)
	if err != nil {
		t.Fatal(err)
	}

	if got := classifier.Scope("tenant-a", "", false, "1.1.1.1"); got != ScopeExternalPublic {
		t.Fatalf("public scope = %s, want %s", got, ScopeExternalPublic)
	}
}

func TestClassifierRejectsUnsupportedUnregisteredScope(t *testing.T) {
	if _, err := NewClassifier([]string{"10.0.0.0/8"}, "internal_same_tenant"); err == nil {
		t.Fatal("expected unsupported scope error")
	}
}
