package core

import "testing"

// A TEMPLATE'S HOLES ARE ITS PARAMETERS IN THE ORDER IT NAMES THEM. One that
// names each parameter once, in order, keeps the `%s` holes fill takes in
// order; one that names them out of order, or one twice, is indexed, `%[k]s`.
// Before, `(${b} >= 64 ? 0L : ${a} >>> ${b})` was filled with a and b swapped
// (the JVM's `u64>>`, found by fnv64-fold).
func TestATemplateNamesItsParametersByIndexWhereOrderDiffers(t *testing.T) {
	params := []string{"a", "b"}
	for tpl, want := range map[string]string{
		"(${a} + ${b})":                     "(%s + %s)",
		"f(${a})":                           "f(%s)",
		"(${b} - ${a})":                     "(%[2]s - %[1]s)",
		"(${a} * ${a})":                     "(%[1]s * %[1]s)",
		"(${b} >= 64 ? 0L : ${a} >>> ${b})": "(%[2]s >= 64 ? 0L : %[1]s >>> %[2]s)",
		"100% ${a}":                         "100%% %s",
	} {
		if got := positionalHoles(tpl, params, nil, "", false); got != want {
			t.Errorf("%s: holes %s, want %s", tpl, got, want)
		}
	}
}
