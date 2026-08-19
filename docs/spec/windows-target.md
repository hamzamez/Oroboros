# The windows target: a host with no expressions

**Status, 2026-08-19. Built and measured.** `targets/windows/` emits x86-64 assembly, MASM
assembles it, `link` produces a `.exe`. Three programs build and run:
`examples/native/hello-win.oro`, `sieve-win.oro` (prints 2262, the same answer as the Go, JS and
Java sieves) and `sieve-win-bench.oro`. The benchmark is **at parity with hand-written assembly**
— [windows-2026-08-19](../../gauntlet/results/windows-2026-08-19.md).

The fourth target exists to ask a question the first three could not. Go, JavaScript and Java are
all **expression languages**: a target file writes `expr "%s + %s"`, the hole is filled with the
operand's emitted expression, and the host's own parser rebuilds the tree. Assembly has no tree.
`add` takes two operands, writes the first, and cannot be nested in anything.

So: how much of the target-file format was a fact about *targets*, and how much was a fact about
*hosts that happen to have expressions*?

**The answer is three additions and no subtractions.** `%r`, `%u`, and `(jump …)`. Everything else
survived unchanged — `expr`, `stmt`, `(import …)`, `pure`, `index`, `where`, and the three
structural kinds. **The structural set is still `let`, `if`, `loop`**, on a host where none of
those exists.

---

## 1. What the target looks like

```lisp
(use x64)
(use windows/kernel32)
(use win/fmt)

(def cross (fn (c i n)
  (loop ((j (x64.imul i i)))
    (x64.setl j n)  (seq (x64.movb c j 1) (again (x64.add j i)))
    else            c)))
```

The names are the host's, per the rule that produced `go.+` and `java.>>>`. On this host the
host's name for adding is `add` and for "is less than" is `setl`, so that is what they are called.
That reads strangely and it is not a mistake — it is the parasite thesis carried to a host whose
vocabulary is an instruction set.

`targets/windows/` is four files: `windows.oro` (types, storage, structural, build), `x64.oro`
(the instruction set), `kernel32.oro` (the Win32 API), `msvcrt.oro` (the C runtime).

## 2. The three additions

### `%r` — where to put the result

There is no expression to *be* the result, so a template is told where to put one.

```lisp
(prim add (int int) int expr "mov %r, %1\nadd %r, %2" pure)
```

Two instructions, because x86's `add` is destructive. The emitter allocates `%r` **before** it
frees the operands, which is what makes this safe with no alias analysis at all: `%r` cannot be an
operand that is still live.

### `%u` — a unique number, so a template may have control flow

```lisp
(prim strlen (string) int expr
  "mov rax, %1\nLstr%u:\ncmp byte ptr [rax], 0\nje Lstrend%u\ninc rax\njmp Lstr%u\nLstrend%u:\nsub rax, %1\nmov %r, rax" pure)
```

That is a loop, inside a template, in a data file. It answers the obvious question about a format
made of templates: no, they are not limited to straight-line code, and a target may put a whole
routine behind one name where the host has no call for it.

`%1…%9` name operands positionally (an instruction sequence rarely uses them in order), and
`%b1`/`%br` and `%e1`/`%er` spell an operand's register at 8 and 32 bits, because x86 gives one
register three names.

### `(jump …)` — a predicate that branches instead of computing

This is the one that changes the numbers.

```lisp
(prim setl (int int) bool expr "mov %r, %1\ncmp %r, %2\nsetl %br\nmovzx %er, %br" pure (jump "l"))
```

The `expr` form is what a comparison costs **as a value**. `(jump "l")` is what it costs **as a
guard**: the backend emits `cmp` and the negated conditional jump, and nothing is materialised.
Without it every loop guard is two compares — one to make a 0/1 boolean and one to test that
boolean against zero.

Go, JavaScript and Java all fold a comparison into a branch inside their own compilers. **This is
the first host that has to be told**, and `(jump …)` is the whole of telling it.

It carries an optional second string, which is the flag-setting instruction when the default
(`cmp %1, %2`, or `comisd %1, %2` for floats) is not it:

```lisp
(prim test-byte ((p ptr) (i int)) bool
  expr "xor %er, %er\ncmp byte ptr [%1+%2], 0\nsetne %br" pure
  (jump "ne" "cmp byte ptr [%1+%2], 0"))
```

`(setne (movzx c i) 0)` is a load, a compare and a branch. x86 does it in one instruction, and
this is how a target says so. It is worth an instruction and a register in the sieve's counting
loop, which is the difference between the emitted inner loop and the hand-written one.

> **Corrected 2026-08-19 by [ADR 0017](../decisions/0017-booleans-are-in-the-language.md).** There
> were two pseudo-codes here, `"and"` and `"or"`, and they were a mistake: they made ONE name mean
> the strict instruction `x64.andb` in value position and a short-circuiting branch in a guard.
> That is observable — an `idiv` by zero behind the guard raises #DE — and it is not something a
> target file should be able to claim. Short-circuiting is now the language's `if`, the connectives
> are reader sugar, and the windows backend recognises the shape. `x64.andb` remains, meaning only
> the instruction.

## 3. `(data …)` — storage the target owns

New, and Win32 forced it. The platform's output primitive is

```
WriteFile(handle, buffer, count, &written, NULL)
```

The fourth argument is a pointer to a cell it writes into. The language has no pointers to locals,
no addresses, and no multiple returns; there was **nowhere to put one**. So the target declares
the cell and hides it inside the template:

```lisp
(data "__written qword 0")
```

```lisp
(prim WriteFile (ptr any int) ptr stmt
  "mov rcx, %1\nmov rdx, %2\nmov r8, %3\nlea r9, __written\nmov qword ptr [rsp+32], 0\ncall WriteFile"
  (import "WriteFile"))
```

The language never learns the argument exists. That works, and it would **not** work under
threads, which is stated rather than fixed.

`(data …)` lines are emitted only when the label appears in the code, exactly as imports are.
That was not the first design: a 4096-byte scratch buffer made `hello-win.exe` 6656 bytes against
a hand-written 2560, for a buffer it never touches. With the check, and a 256-byte buffer, the
hello artifact is **2560 bytes — the same as hand-written, to the byte**.

## 4. `fmt` in assembly is a program, not a declaration

This is the finding worth the whole experiment.

`targets/go/fmt.oro` declares `fmt.Println` and Go does everything. Java has
`System.out.println`, JavaScript has `console.log`. **Win32 has nothing.** It has `WriteFile` over
bytes, and nothing beneath it knows what a number is. Turning 2262 into the four bytes `"2262"` is
a program, and on this target it is written in Oroboros:

```lisp
(def print-int (fn (n)
  (if (x64.sete n 0)
      (print-str "0")
      (let (x64.buf) (fn (b)
        (seq (x64.movb b 24 10)
             (let (loop ((m n) (i 24))
                     (x64.setg m 0)
                     (seq (x64.movb b (x64.sub i 1) (x64.add 48 (x64.irem m 10)))
                          (again (x64.idiv m 10) (x64.sub i 1)))
                     else i)
                  (fn (start)
                    (write-bytes (x64.lea b start)
                                 (x64.add (x64.sub 24 start) 1))))))))))
```

`lib/win/fmt.oro`. Thirty lines where Go needs one declaration — and it still reduces away into
the caller, because the reducer does not care which of the two it is.

**That is the price of a bare host, stated exactly.** Requirement 1 says the language is small and
requirement 2 says there are many targets; this is the first target where the two are visibly in
tension, and the resolution is that the *language* did not grow. The program did.

The other side of the same coin is `windows/msvcrt.oro`. `printf` is **variadic**, and
`targets/go/fmt.oro` records that Go's variadic `Printf` with four operands "cannot be called at
all". Here it can, at every arity declared, because at this level a variadic call is not a
different kind of call — it is a call with n arguments and the caller cleans up. **The wall on Go
was Go's type system, not the concept.** What remains is that an undeclared arity is still
uncallable, which is our own limitation wearing the same clothes.

## 5. What the emitted code looks like

The sieve's crossing loop, emitted:

```asm
Ltop9:
        cmp r12, 20000
        jge Lnext10
        mov byte ptr [rbx+r12], 1
        add r12, rsi
        jmp Ltop9
```

That is instruction-for-instruction what a person writes. Getting there took four things beyond
the naive lowering, and each is worth naming because each was a real gap:

1. **Values live in callee-saved registers** (rbx, rsi, rdi, r12-r15). kernel32 preserves all of
   them, so a Win32 call costs nothing and nothing is spilled around one.
2. **In-place assignment on the back edge.** `x = x OP e` emitted the general way is three
   instructions where x86 does it in one. The condition for folding a template into its own first
   operand is textual and checkable — the template must begin `mov %r, %1` and never mention `%1`
   again — so it is a proof about the template rather than a special case for `add`.
3. **Ordered back-edge assignments.** `needTemps` asks the question Go, JS and Java can act on:
   is a temporary needed at all? x86 can do better, because for `(again (x64.add i 1) (x64.add acc i))`
   the answer is *no, if you assign `acc` first*. That is parallel-copy sequentialisation, and only
   a genuine cycle — a swap — costs one copy.
4. **Two jump peepholes.** A clause chain branches *around* a clause body, and a clause body that
   only leaves the loop is one `jmp`. `jmp L … L:` disappears; `jcc A / jmp B / A:` becomes
   `jncc B`. Structured control flow generated structurally always produces this shape and a
   person never does.

## 6. What broke, and what could not be said

**`cmd/build` could not build any program containing a `loop`.** `core.Residual` treated `again`
as a free name. ADR 0015 was built and benchmarked entirely through `gen`, which emits a function;
this check lives in `build`, which makes a binary. Every loop program was refused by the one
command that produces an artifact, and no test noticed **because no test built one**. Found here,
fixed in `core.Residual`, and it was a bug on all four targets.

**One `idiv` produces two results** — quotient in rax, remainder in rdx — and the language has no
product, so `x64.idiv` and `x64.irem` are declared separately and the divide runs twice. This is
the **fifth independent demand for a product type**, and the first that comes from the hardware
rather than from a library.

**`mov` is both directions.** x86 spells load and store with one mnemonic and tells them apart by
which operand is memory. The language has no assignable place, so the direction has to be in the
name: `x64.mov` loads and `x64.mov-store` stores. That is the one place the host's own naming
could not be kept.

**Two of our type names are one machine type.** `string` *is* `ptr` here — the same qword — so
declaring `WriteFile`'s buffer as `ptr` made `(WriteFile h "hi" 2)` a type error the host does not
have. It is declared `any`. This is the JavaScript `js.+` problem from the other side: there, one
host type had to serve two of ours; here, two of ours name one host type.

**A template cannot see that its argument is a literal.** The emitter knows a string literal's
length at compile time and writes it into the data section as `LS2_len`, and then throws it away:
`x64.strlen` receives a register and runs a loop. A hand-written program writes `msglen equ $ -
msg` and spends zero instructions. The fix is not in the format — it is that a string here is a
bare pointer, which is the product type again.

**No common-subexpression elimination.** The sieve computes `i*i` in the guard and again as the
inner loop's start, because they are different subterms and nothing in the pipeline unifies them.
On the other three hosts the host compiler did this for us and we never noticed. **This is the
sharpest thing the bare host shows: the optimisations you were parasitizing are gone, and you
only find out which ones they were by removing the host.** It costs two instructions per outer
iteration here, which is measurable in principle and lost in the noise in fact.

**A target file cannot ask for a bigger frame.** Every procedure reserves 48 bytes at the bottom —
the 32-byte Win64 home space plus two stack arguments — so a template can write `[rsp+32]` without
knowing anything about the frame it is expanded into. `WriteFile` needs one stack argument and
fits. `CreateFileA` takes seven arguments, needs three, and is **not declared**, because there is
no way to say "this primitive needs a deeper frame".

**`Build` is split on whitespace and run without a shell**, and every Windows toolchain lives
under a path with a space in it. `go`, `node` and `javac` are bare words on PATH, so no target had
ever needed more than one command or a quoted path. Discovery moves into a `build.bat` the target
writes — which works and is a workaround, not a fix.

**`cmd/build` copied the artifact only before the build.** `go build -o` takes a destination;
`ml64` and `link` do not, and neither does any toolchain driven through a script. It now copies
after as well.

## 7. What the emitter got right that the hand-written reference got wrong

Stack alignment. `gauntlet/windows/sieve.asm` had it wrong in two of three procedures on the first
attempt; the program printed the correct answer and then crashed inside kernel32, with nothing in
the traceback pointing at either. The emitter computes the padding from the number of pushes and
cannot get it wrong.

That is worth recording beside the places where generated code loses. Parity is the bar, and on
this host part of reaching it is not making the mistakes the bar's author made.

## 8. Where it stands

| | |
|---|---|
| structural primitives | **3** — `let`, `if`, `loop`, the same as the other three natives |
| format additions | `%r`, `%u`, `(jump …)`, `(data …)` |
| format subtractions | none |
| sieve benchmark | **0.97× hand-written** (median of 9), inside the 15% noise floor |
| `hello` artifact | **2560 bytes**, identical to hand-written |
| sieve artifact | 3072 bytes against hand-written 2560 |
| floats | `f64` in xmm6-xmm13, literals emitted as IEEE-754 bits (ADR 0009) |
| not built | `f64` output (`printf1f` is declared and untested), CreateFileA, threads |
