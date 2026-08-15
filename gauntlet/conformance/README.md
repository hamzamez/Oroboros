# Conformance

The suite [modules.md §8](../../docs/spec/modules.md) requires and did not have.

It tests **our lowerings**, never the host. A failure means the JS or Java implementation of a
name disagrees with its specification — not that the target is disqualified.

`cases.json` is the shared input. Each runner emits one line per case: the field count, then the
fields. The three outputs must be byte-identical.

    go run conform.go          > out.go.txt
    node conform.mjs           > out.js.txt
    javac -encoding UTF-8 -d . Conform.java && java -cp . Conform > out.java.txt
    diff out.go.txt out.js.txt && diff out.go.txt out.java.txt

`split-words` failed this on four of ten cases before 2026-08-15 and nothing detected it, because
the covering check proves a name is *provided* and can never prove it is *right*.
