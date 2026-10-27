"""Σ_LLVM, surveyed: every instruction and intrinsic of the installed LLVM, classified
by whether the language can state it today, and if not, by the wall it meets.

    python -I experiments/llvmsurvey/survey.py [LLVM_DIR]

LLVM_DIR defaults to %LOCALAPPDATA%/Programs/LLVM. Reads include/llvm/IR/Instruction.def
and the records llvm-tblgen dumps from include/llvm/IR/Intrinsics.td. Two classifications:

  1. By signature: the sorts an intrinsic's types need (mechanical, every target).
  2. By meaning: each instruction and target-independent intrinsic, by hand-written rules
     below, first match wins; an unmatched name fails the run.

Result: gauntlet/results/llvmsurvey-2026-10-27.md.
"""
import collections, json, os, re, subprocess, sys, tempfile

LLVM = sys.argv[1] if len(sys.argv) > 1 else os.path.join(
    os.environ.get("LOCALAPPDATA", ""), "Programs", "LLVM")
INC = os.path.join(LLVM, "include")

# The verdicts. S is "stateable now"; every other letter is a wall or a decision already made.
VERDICT = {
    "S":  "stateable now: a host name over Int (the word), F64, Bool, tuples, variants or buffers; UB its `where`",
    "SN": "stateable on ↑0 (Nat) only; on a negative operand it is a bit-vector operation (B)",
    "B":  "WALL bit-vectors: an operation of BV_N or ℤ/2ⁿ, not the image of a function on ℤ",
    "W":  "WALL widths: an integer of N ≠ 64 bits, a representation ρ does not yet choose",
    "F":  "WALL float formats: half, bfloat, float, fp128, x86_fp80, ppc_fp128",
    "N":  "WALL relations: LLVM specifies a set of results, not one (unspecified rounding or fusion)",
    "X":  "WALL float environment: a dynamic rounding mode or exception flags as state",
    "P":  "WALL pointers as values: addresses, provenance, address spaces",
    "V":  "WALL vectors: <N x T> and <vscale x N x T> as a type",
    "R":  "WALL records: struct aggregates (6d's wall)",
    "A":  "concurrency: atomics, fences, thread-locals (ADR 0081's lowering, not a program's primitive)",
    "C":  "concurrency: coroutines (a candidate lowering of ADR 0081's processes)",
    "E":  "decided, ADR 0040: unwinding exceptions (a failure is a value; the language is nounwind)",
    "U":  "decided, never: unstructured control (indirect branches, callbr)",
    "K":  "the frame and stack: introspection, trampolines, varargs definition, register access",
    "M":  "another runtime's: garbage collection, deoptimization, Objective-C ARC, pointer authentication",
    "G":  "another target's or the backend's own: GPU convergence, SPIR-V structure, BPF CO-RE, hardware loops",
    "I":  "the printer's or the build's vocabulary: assumptions, lifetimes, debug info, profiling, sanitizers, CFI",
    "T":  "structure: the printer's own (return, branch, switch, phi, call)",
}

# ---- 1. instructions ------------------------------------------------------------------------
INSTR = {
    # terminators
    "Ret": "T", "UncondBr": "T", "CondBr": "T", "Switch": "T", "Unreachable": "T",
    "IndirectBr": "U", "CallBr": "U",
    "Invoke": "E", "Resume": "E", "CleanupRet": "E", "CatchRet": "E", "CatchSwitch": "E",
    # arithmetic: S when its poison flags hold (nsw, nuw, exact), else ℤ/2ⁿ
    "Add": "S", "Sub": "S", "Mul": "S", "SDiv": "S", "SRem": "S",
    "UDiv": "SN", "URem": "SN",
    "Shl": "S", "AShr": "S", "LShr": "SN",
    "And": "S", "Or": "S", "Xor": "S",   # total on ℤ (2-adic), closed on the word
    "FNeg": "S", "FAdd": "S", "FSub": "S", "FMul": "S", "FDiv": "S", "FRem": "S",
    # memory: the printing of tables (alloca a buffer, load an index, gep a segment)
    "Alloca": "S", "Load": "S", "Store": "S", "GetElementPtr": "S",
    "Fence": "A", "AtomicCmpXchg": "A", "AtomicRMW": "A",
    # casts
    "Trunc": "W", "ZExt": "W", "SExt": "W",
    "FPToUI": "SN", "FPToSI": "S", "UIToFP": "SN", "SIToFP": "S",
    "FPTrunc": "F", "FPExt": "F",
    "PtrToInt": "P", "PtrToAddr": "P", "IntToPtr": "P", "AddrSpaceCast": "P",
    "BitCast": "B",                      # f64 ↔ i64 is Float64bits, a bit pattern
    "CleanupPad": "E", "CatchPad": "E", "LandingPad": "E",
    "ICmp": "S", "FCmp": "S", "PHI": "T", "Call": "T", "Select": "S",
    "VAArg": "K",
    "ExtractElement": "V", "InsertElement": "V", "ShuffleVector": "V",
    "ExtractValue": "R", "InsertValue": "R",
    "Freeze": "S",                       # the identity on every value the language produces
}

# ---- 2. target-independent intrinsics, by meaning --------------------------------------------
RULES = [
    # integers on the word
    (r"abs|smax|smin|scmp", "S"),
    (r"umax|umin|ucmp", "SN"),
    (r"ctpop|ctlz", "SN"), (r"cttz", "S"),            # cttz is ν₂, the 2-adic valuation
    (r"bswap|bitreverse|fshl|fshr|clmul|pdep|pext", "B"),
    (r"s(add|sub|mul)\.with\.overflow", "S"),          # the partial op, totalized as a variant
    (r"u(add|sub|mul)\.with\.overflow", "SN"),
    (r"s(add|sub|shl)\.sat", "S"),                     # clamp ∘ op
    (r"u(add|sub|shl)\.sat", "SN"),                    # usub.sat is ℕ's monus
    (r"[su](mul|div)\.fix(\.sat)?", "N"),              # "The rounding direction is unspecified."
    # floats on F64
    (r"fmuladd", "N"),                                 # "Fusion is not guaranteed"
    (r"powi", "N"),                                    # "order of evaluation of multiplications is not defined"
    (r"fptrunc\.round|convert\.(to|from)\.arbitrary\.fp", "F"),
    (r"sqrt|fabs|copysign|floor|ceil|trunc|rint|nearbyint|round|roundeven|fma|"
     r"minnum|maxnum|minimum|maximum|minimumnum|maximumnum|"
     r"a?(sin|cos|tan)h?|atan2|exp(2|10)?|log(2|10)?|pow|ldexp|frexp|modf|sincos(pi)?|"
     r"l?lrint|l?lround|is\.fpclass|fpto[su]i\.sat|canonicalize", "S"),
    (r"experimental\.constrained\..*", "X"),
    (r"[gs]et\.rounding|(get|set|reset)\.fp(env|mode)", "X"),
    (r"arithmetic\.fence", "I"),
    # memory over buffers
    (r"mem(cpy|move|set)(\.inline)?|experimental\.memset\.pattern", "S"),
    (r"mem(cpy|move|set)\.element\.unordered\.atomic", "A"),
    (r"prefetch|readcyclecounter|readsteadycounter", "S"),
    (r"trap|debugtrap|ubsantrap|looptrap|cond\.loop", "S"),
    (r"objectsize|load\.relative|ptrmask", "P"),
    (r"threadlocal\.address|thread\.pointer", "A"),
    # vectors and matrices
    (r"(experimental\.)?vector\..*|(experimental\.)?vp\..*|masked\..*|matrix\..*|stepvector|vscale|"
     r"get\.active\.lane\.mask|experimental\.(get\.vector\.length|cttz\.elts)|loop\.dependence\..*", "V"),
    # control, exceptions, coroutines
    (r"callbr\.landingpad", "U"),
    (r"eh\..*|seh\..*", "E"),
    (r"coro\..*", "C"),
    # the frame
    (r"stack(address|guard|protector)|frameaddress|returnaddress|addressofreturnaddress|"
     r"sponentry|local(address|escape|recover)|(read|write)(_volatile)?_register|clear_cache|"
     r"(init|adjust)\.trampoline|get\.dynamic\.area\.offset|call\.preallocated\..*|va_(start|end|copy)|"
     r"swift\.async\.context\.addr", "K"),
    # other runtimes
    (r"experimental\.(gc\..*|stackmap|patchpoint(\.void)?|deoptimize|guard|widenable\.condition)|"
     r"gc(root|read|write)|objc\..*|ptrauth\..*", "M"),
    (r"experimental\.convergence\..*|structured\..*|preserve\..*|"
     r"(set|start|test\.set|test\.start)\.loop\.iterations|loop\.decrement(\.reg)?", "G"),
    # the printer's and the build's
    (r"stack(save|restore)|assume|expect(\.with\.probability)?|lifetime\..*|invariant\..*|(launder|strip)\.invariant\.group|"
     r"experimental\.noalias\.scope\.decl|is\.constant|sideeffect|donothing|fake\.use|ssa\.copy|"
     r"((var|ptr|codeview)\.)?annotation|pseudoprobe|dbg\..*|instrprof\..*|xray\..*|h?w?asan\..*|"
     r"allow\..*|(public\.)?type\.(test|checked\.load(\.relative)?)|icall\.branch\.funnel|"
     r"alloc\.token\.id|protected\.field\.ptr|reloc\.none|pcmarker", "I"),
]

# ---- signatures -------------------------------------------------------------------------------
TIER = ["word", "narrow", "subfloat", "wide", "widefloat", "ptr", "vector", "special", "internal"]
TIER_NAME = {
    "word": "Int, F64, Bool only (or overloaded and instantiable at them)",
    "narrow": "needs i8/i16/i32", "subfloat": "needs f16/bf16/f32", "wide": "needs i128",
    "widefloat": "needs f128/ppc_fp128", "ptr": "needs a pointer", "vector": "needs a vector",
    "special": "needs a target's special type (x86_amx, x86_mmx, svcount, wasm references)",
    "internal": "needs a token or metadata",
}

def vt_class(vt):
    if vt in ("i64", "i1", "f64", "isVoid", "iAny", "fAny", "Any"): return "word"
    if vt in ("i8", "i16", "i32"): return "narrow"
    if vt in ("f32", "f16", "bf16"): return "subfloat"
    if vt == "i128": return "wide"
    if vt in ("f128", "ppcf128", "f80"): return "widefloat"
    if vt in ("iPTR", "pAny"): return "ptr"
    if re.match(r"(nx)?v\d+", vt) or vt == "vAny": return "vector"
    if vt in ("token", "MetadataVT"): return "internal"
    return "special"

def signature_tier(d, r):
    types = r["RetTypes"] + r["ParamTypes"]
    over = [d[t["def"]] for t in types if "LLVMAnyType" in d[t["def"]]["!superclasses"]]
    # An immediate argument is a constant, erased by staging: it needs no runtime sort.
    # ImmArg's ArgNo is LLVM's attribute index: 0 the return, parameter i at i+1.
    imm = {len(r["RetTypes"]) + d[p["def"]]["ArgNo"] - 1 for p in r["IntrProperties"]
           if "ImmArg" in d[p["def"]]["!superclasses"]}
    worst = 0
    for i, t in enumerate(types):
        if i in imm:
            continue
        tr = d[t["def"]]
        vt = tr["VT"]["def"]
        if vt == "OtherVT":                                # dependent on an overloaded type
            i = tr.get("OverloadIndex", 0)
            vt = over[i]["VT"]["def"] if i < len(over) else "Any"
        worst = max(worst, TIER.index(vt_class(vt)))
    return TIER[worst]

def main():
    out = tempfile.mkdtemp()
    js = os.path.join(out, "intr.json")
    subprocess.run([os.path.join(LLVM, "bin", "llvm-tblgen"), "-I", INC, "--dump-json",
                    os.path.join(INC, "llvm", "IR", "Intrinsics.td"), "-o", js], check=True)
    d = json.load(open(js))
    ver = subprocess.run([os.path.join(LLVM, "bin", "clang"), "--version"],
                         capture_output=True, text=True).stdout.splitlines()[0]
    print("#", ver)

    idef = open(os.path.join(INC, "llvm", "IR", "Instruction.def")).read()
    names = re.findall(r"^HANDLE_\w+_INST\s*\(\s*\d+\s*,\s*(\w+)", idef, re.M)
    names = [n for n in names if not n.startswith("UserOp")]   # "Internal to passes only"
    missing = [n for n in names if n not in INSTR]
    assert not missing, ("unclassified instructions", missing)
    print("\n## instructions:", len(names))
    c = collections.Counter(INSTR[n] for n in names)
    for v, k in c.most_common():
        print(f"{v:3}{k:4}  " + " ".join(n for n in names if INSTR[n] == v))

    intr = d["!instanceof"]["Intrinsic"]
    indep = sorted((k for k in intr if d[k]["TargetPrefix"] == ""), key=lambda k: llname(d, k))
    print("\n## target-independent intrinsics:", len(indep))
    by = collections.defaultdict(list)
    for k in indep:
        n = llname(d, k)[len("llvm."):]
        v = next((v for pat, v in RULES if re.fullmatch(pat, n)), None)
        assert v, ("unclassified intrinsic", n)
        by[v].append((n, signature_tier(d, d[k])))
    for v in sorted(by, key=lambda v: -len(by[v])):
        print(f"{v:3}{len(by[v]):4}  " + " ".join(n for n, _ in by[v]))
    print("\n### stateable by meaning, against what the signature needs")
    for v in ("S", "SN"):
        t = collections.Counter(tier for _, tier in by[v])
        print(v, dict(t), " needing more than the word:",
              " ".join(f"{n}[{tier}]" for n, tier in by[v] if tier != "word"))

    print("\n## every intrinsic by target, by what its signature needs")
    per = collections.defaultdict(collections.Counter)
    for k in intr:
        per[d[k]["TargetPrefix"] or "(independent)"][signature_tier(d, d[k])] += 1
    print("target".ljust(14) + "".join(t[:8].rjust(9) for t in TIER) + "    total")
    tot = collections.Counter()
    for tgt in sorted(per, key=lambda t: -sum(per[t].values())):
        tot.update(per[tgt])
        print(tgt.ljust(14) + "".join(str(per[tgt][t]).rjust(9) for t in TIER)
              + str(sum(per[tgt].values())).rjust(9))
    print("all".ljust(14) + "".join(str(tot[t]).rjust(9) for t in TIER) + str(sum(tot.values())).rjust(9))

    print("\n## x86 intrinsics whose signature needs only Int, F64, Bool")
    xs = sorted(llname(d, k) for k in intr if d[k]["TargetPrefix"] == "x86"
                and signature_tier(d, d[k]) == "word")
    print(len(xs), " ".join(xs))

def llname(d, k):
    return d[k]["LLVMName"] or "llvm." + k[len("int_"):].replace("_", ".")

if __name__ == "__main__":
    main()
