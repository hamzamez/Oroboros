#!/bin/sh
# Run the conformance suite on all three targets and require identical output.
set -e
cd "$(dirname "$0")"
go run conform.go > out.go.txt
node conform.mjs   > out.js.txt
javac -encoding UTF-8 -d . Conform.java && java -cp . Conform > out.java.txt
diff out.go.txt out.js.txt   && echo "go == js"
diff out.go.txt out.java.txt && echo "go == java"
echo "conformance: all three targets agree"
