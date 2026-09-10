// jsdump.mjs — the JavaScript runtime's surface as a manifest, for
// gauntlet/stdlib/js.go.
//
//   node gauntlet/stdlib/jsdump.mjs > /tmp/js-api.txt
//
// THE RUNTIME IS THE MANIFEST, for the third time — Go ships a text file, the
// JDK is reflectable, and V8 is the most reflectable of the three. What it
// cannot give is the thing the other two give for free: A TYPE. `globalThis`
// knows every name it has and nothing about what any of them accepts or
// returns, and there is no second source on this machine to ask.
//
// SO THE ARITY IS NOT RELIABLE EITHER, and that is worth more than it looks.
// `Function.length` is the count of parameters before the first default or rest
// — but a NATIVE function's parameters are not introspectable at all, so
// `Math.max.length` is 2 and `Math.max(1,2,3,4)` is 4. The host does not merely
// decline to tell us the types; it misreports the shape.
//
// What is recorded, per member, is therefore the most a program could act on:
//
//   path       what a template must write — `Math.max`, `Array.prototype.map`
//   kind       function | class | getter | value | namespace
//   arity      `Function.length`, which is a LOWER BOUND and is marked as one
//   symbol     a Symbol-keyed member, which has no name to declare
//
// Everything is sorted, and the walk is depth-limited and cycle-guarded, so two
// runs give the same file (backend-2026-09-06).

import { builtinModules } from "node:module";

const out = [];
const seen = new WeakSet();

function kindOf(v, desc) {
  if (desc && (desc.get || desc.set)) return "getter";
  const t = typeof v;
  if (t === "function") {
    // A CLASS MUST BE CALLED WITH `new` AND A FUNCTION MUST NOT, and the two
    // are a real distinction here even though JavaScript spells them the same:
    // a template that gets it wrong throws at RUN time rather than failing to
    // compile, which is the whole hazard of a host with no static check.
    //
    // A non-writable `prototype` is what BOTH a `class` declaration and a
    // native constructor (Array, Map, Date) produce, and an ordinary function's
    // is writable. The first version also called anything with
    // `prototype.constructor === v` a class — which is EVERY plain function, so
    // `fs.readFileSync` came out a class. Caught by reading the manifest rather
    // than by anything failing.
    const d = Object.getOwnPropertyDescriptor(v, "prototype");
    if (d && d.writable === false) return "class";
    return "function";
  }
  if (v !== null && t === "object") return "namespace";
  return "value";
}

function walk(root, path, depth) {
  if (depth > 3 || root === null || root === undefined) return;
  if (typeof root === "object" || typeof root === "function") {
    if (seen.has(root)) return;
    seen.add(root);
  }
  let names;
  try {
    names = Object.getOwnPropertyNames(root).sort();
  } catch {
    return;
  }
  let syms = 0;
  try {
    syms = Object.getOwnPropertySymbols(root).length;
  } catch {}
  if (syms > 0) out.push(`sym ${path} ${syms}`);

  for (const n of names) {
    if (n === "constructor" || n === "caller" || n === "arguments" || n === "__proto__") continue;
    // `length`, `name` and `prototype` are every function's OWN properties and
    // are not surface. And a `_`-prefixed member is private by convention —
    // `node:module._pathCache` is keyed by ABSOLUTE PATH, so leaving it in put
    // this machine's working directory into the manifest. A manifest that
    // embeds where it was run is not a description of the host.
    if (n === "length" || n === "name" || n === "prototype") continue;
    if (n.startsWith("_")) continue;
    // RUNTIME STATE IS NOT SURFACE. `process.env` is this machine's
    // environment, `process.config` is how this Node was built, and
    // `process.moduleLoadList` is what this run happens to have loaded — none
    // of them is a description of JavaScript, and all three put machine-
    // specific text into the manifest. Named explicitly rather than filtered by
    // a heuristic, because three names are checkable and a heuristic is not.
    if (path === "node:process" && (n === "env" || n === "config" || n === "moduleLoadList")) {
      out.push(`mem ${p} value 0`);
      continue;
    }
    let desc;
    try {
      desc = Object.getOwnPropertyDescriptor(root, n);
    } catch {
      continue;
    }
    if (!desc) continue;
    const p = path + "." + n;
    // THE NAME CHECK COMES BEFORE THE KIND, because a GETTER can have an
    // unnameable key too: `RegExp.$&` is a legacy accessor, and the first
    // version marked only value-shaped members, so it reached the reader as
    // `"$&" is not a valid identifier`. The refusal was the loud direction.
    const ident = /^[A-Za-z_$][A-Za-z0-9_$]*$/.test(n) ? "" : " unnameable";
    // A GETTER IS NOT READ. Reading one runs host code — `process.memoryUsage`
    // is harmless and some are not — so a getter is recorded by its descriptor
    // and never invoked. A survey that evaluated the host to describe it would
    // be measuring a side effect.
    if (desc.get || desc.set) {
      out.push(`mem ${p} getter 0${ident}`);
      continue;
    }
    const v = desc.value;
    // AN ARRAY'S ELEMENTS ARE DATA, NOT SURFACE. `http.METHODS` is a list of
    // strings and walking it produced 35 members named `0`, `1`, `2` — every
    // one of them "unnameable", which inflated that count fivefold and told us
    // nothing about the host.
    if (Array.isArray(v)) {
      out.push(`mem ${p} value 0`);
      continue;
    }
    const k = kindOf(v, desc);
    const arity = typeof v === "function" ? v.length : 0;
    out.push(`mem ${p} ${k} ${arity}${ident}`);
    if (k === "class") {
      // A CLASS'S INSTANCE METHODS LIVE ON ITS PROTOTYPE, which is where a
      // receiver-as-argument-0 declaration has to read them from. Its statics
      // are its own properties.
      try {
        walk(v.prototype, p + ".prototype", depth + 1);
      } catch {}
      try {
        walk(v, p, depth + 1);
      } catch {}
    } else if (k === "namespace" && ident === "") {
      try {
        walk(v, p, depth + 1);
      } catch {}
    }
  }
}

out.push(`root globalThis`);
walk(globalThis, "globalThis", 0);

for (const m of builtinModules.slice().sort()) {
  if (m.startsWith("_") || m === "sys") continue;
  let mod;
  try {
    mod = await import("node:" + m);
  } catch {
    continue;
  }
  out.push(`root node:${m}`);
  try {
    walk(mod.default ?? mod, "node:" + m, 0);
  } catch {}
}

process.stdout.write(out.join("\n") + "\n");
