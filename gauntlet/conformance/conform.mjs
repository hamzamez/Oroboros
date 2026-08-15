// Conformance runner for the JavaScript target. See README.md.
import { readFileSync } from "fs";
const cases = JSON.parse(readFileSync("cases.json", "utf8"));
for (const s of cases) {
  const f = (s.match(/\S+/g) ?? []);       // the lowering in targets/js.oro
  console.log(`${f.length} ${JSON.stringify(f)}`);
}
