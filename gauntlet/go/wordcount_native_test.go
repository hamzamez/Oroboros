package gauntlet

import "testing"

// Gauntlet program 4 on the NATIVE Go target (examples/native/wordcount-go.oro).
//
// g4's pass condition was never a number: the Go output must use Go's own
// map[string]int and Go's own splitting. On the portable layer that took three
// names — dict-empty, dict-inc, split-words — whose whole job was to hide which
// host construct was chosen. Here there is nothing to hide, and what the
// migration tested instead was whether the target could SAY it: the native Go
// target had no string surface at all until this program asked for one.

func TestNativeTallyAgrees(t *testing.T) {
	got, want := NativeTally(text), WordCountIncr(text)
	if len(got) != len(want) {
		t.Fatalf("native has %d keys, hand-written %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%q: native %d, hand-written %d", k, got[k], v)
		}
	}
}

func BenchmarkG4WordCountNative(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkM = NativeTally(text)
	}
}

// The fused form. Go's `m[k]++` is one mapassign returning a value pointer;
// `m[k] = m[k] + 1` hashes the key twice. Carried as the second form because
// the rule for adding to the gauntlet is to carry the one expected to lose —
// and on Java the fused `merge` DOES lose, by 2.6x.
func TestNativeTallyIncAgrees(t *testing.T) {
	got, want := NativeTallyInc(text), WordCountIncr(text)
	if len(got) != len(want) {
		t.Fatalf("native has %d keys, hand-written %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%q: native %d, hand-written %d", k, got[k], v)
		}
	}
}

func BenchmarkG4WordCountNativeInc(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkM = NativeTallyInc(text)
	}
}
