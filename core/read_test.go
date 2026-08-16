package core

import "testing"

// core-0 §1.1 — NFC. Without it, `é` as U+00E9 and as e+U+0301 are two
// distinct identifiers that display identically.
func TestNonNFCIsRejected(t *testing.T) {
	if _, err := Read("(def caf\u0065\u0301 1)"); err == nil {
		t.Fatal("a decomposed é must be rejected")
	}
	if _, err := Read("(def caf\u00e9 1)"); err != nil {
		t.Errorf("a precomposed é is NFC and must pass: %v", err)
	}
	// A combining mark with no precomposed form is NFC-stable, and rejecting
	// it would break every script that needs one.
	if _, err := Read("(def q\u0307x 1)"); err != nil {
		t.Errorf("q + combining dot above has no precomposed form: %v", err)
	}
}

// §1.1 — case is preserved and significant for IDENTITY. What the rule forbids
// is case carrying MEANING, as in Shen's capitals-are-variables or Go's
// capitals-are-exported.
func TestCaseIsSignificantForIdentity(t *testing.T) {
	forms, err := Read("(f Xr xr XR)")
	if err != nil {
		t.Fatal(err)
	}
	kids := forms[0].Term.Kids
	seen := map[string]bool{}
	for _, k := range kids[1:] {
		seen[k.Name] = true
	}
	if len(seen) != 3 {
		t.Errorf("Xr, xr and XR must be three distinct names, got %d: %v", len(seen), seen)
	}
}

// A repeated binder in ONE abstraction is ill-formed: β substituted parameter
// by parameter, so ((fn (x x) x) 1 2) reduced to 2 and the first argument
// vanished with no way to name it.
func TestDuplicateParameterIsRejected(t *testing.T) {
	if _, err := Read("(fn (x x) x)"); err == nil {
		t.Fatal("(fn (x x) x) must be rejected")
	}
	if _, err := Read("(fn (a b) a)"); err != nil {
		t.Errorf("distinct parameters must pass: %v", err)
	}
	// Nested shadowing is two abstractions, and stays legal.
	if _, err := Read("(fn (x) (fn (x) x))"); err != nil {
		t.Errorf("nested shadowing must stay legal: %v", err)
	}
}
