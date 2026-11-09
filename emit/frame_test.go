package emit

import "testing"

// A FALLING COUNTER IS BOUNDED BY ITS START (natnum-2026-11-10): Houdini's frame
// templates run both ways, z ≤ v and v ≤ z, so a scan from n − 1 down keeps
// l ≤ n − 1, which t[l] needs. THE PLANTED FAULT drops v ≤ z, and the index
// must be refused; a counter that rises past its start must be refused either
// way, so the template is checked, never assumed.
func TestAFallingCounterIsBoundedByItsStart(t *testing.T) {
	const falling = "export fn f(t ∈ Array(Int)) (Int) {\n" +
		"    n = len(t)\n" +
		"    loop l = n - 1, s = 0 { l < 0 -> s; else -> again(l - 1, s + t[l] % 2) }\n}\n"
	const rising = "export fn f(t ∈ Array(Int)) (Int) {\n" +
		"    n = len(t)\n" +
		"    loop l = n - 1, s = 0 { l < 0 -> s; l > n + 5 -> s; else -> again(l + 1, s + t[l] % 2) }\n}\n"
	if err := refineGo(t, falling); err != nil {
		t.Errorf("a falling counter's index is refused: %v", err)
	}
	if err := refineGo(t, rising); err == nil {
		t.Errorf("a counter rising past its start is proven inside the table")
	}
	PlantNoUpperFrame = true
	defer func() { PlantNoUpperFrame = false }()
	if err := refineGo(t, falling); err == nil {
		t.Errorf("without v ≤ z the falling counter is still proven: the witness cannot fail")
	}
}
