; HAND-WRITTEN x86-64 BIGNUM. The bar for R3 on the one host with no bignum.
;
; Go, JavaScript and Java each ship one, so on those hosts the question was
; "ours or theirs" (bigarith-2026-08-28: ours, with a per-target threshold).
; windows ships none — `targets/windows/` brings VirtualAlloc and nothing else —
; so ours is the only option and the question is a different one:
;
;   WHAT DOES THE TARGET HAVE TO DECLARE FOR US TO REACH THIS?
;
; Two things x86-64 has that no other host in the set does:
;
;   mul   a 64x64 -> 128 multiply in ONE instruction, high half in rdx. Go has
;         bits.Mul64, the JVM has a SIGNED multiplyHigh needing a correction on
;         JDK 17, and JavaScript has nothing at all.
;   adc   add-with-carry. The carry lives in a FLAG, so a limb-vector add needs
;         no carry variable and no compare — which is why the loop below bumps
;         its index with `lea` and its counter with `dec`, the two instructions
;         that leave CF alone.
;
; `mul` is declarable: one more `(prim …)` in targets/windows/x64.oro, the same
; shape as every other. `adc` is NOT — the carry flag does not survive between
; two emitted statements, and nothing in the IR can express "keep this chain
; intact". So the two forms below are exactly the question:
;
;   *_adc    what a person writes
;   *_expl   what we could emit today: the carry as an ordinary value, with a
;            compare to detect it, which is what the Go and Java references in
;            this result already write by hand
;
; If the gap is small, `adc` is not worth inventing IR for and `mul` is one
; declaration. If it is large, the carry flag is a real hole in the model.
;
; Build:
;   build-bigarith.bat
;
; Prints, per form: nanoseconds per round, then a checksum. The checksum is the
; low limb of 2000! and of fib(1000) and must match every other host.

option casemap:none

extern VirtualAlloc: proc
extern GetStdHandle: proc
extern WriteFile: proc
extern ExitProcess: proc
extern QueryPerformanceCounter: proc
extern QueryPerformanceFrequency: proc

.data
    lbl_fa   db "fact adc   ", 0
    lbl_fe   db "fact expl  ", 0
    lbl_ba   db "fib  adc   ", 0
    lbl_be   db "fib  expl  ", 0
    lbl_bp   db "fib  prim  ", 0
    lbl_ns   db " ns/round  sum=", 0
    nl       db 13, 10
    align 8
    qpc_freq dq 0

.code

; ---------------------------------------------------------------- factorial
;
; acc is a little-endian base-2^64 magnitude. Multiply in place by k = 2..n.
; rcx = n, rdx = buf, r8 = cap (limbs, for the zero fill). Returns rax = used.

fact_adc PROC
    push rbx
    push rsi
    push rdi
    mov  rsi, rcx                   ; n
    mov  rdi, rdx                   ; buf
    xor  rax, rax                   ; zero the buffer
    mov  rcx, r8
    mov  r9, rdi
zf1:
    mov  qword ptr [r9], 0
    lea  r9, [r9+8]
    dec  rcx
    jnz  zf1
    mov  qword ptr [rdi], 1
    mov  r10, 1                     ; used
    mov  rbx, 2                     ; k
fa_outer:
    cmp  rbx, rsi
    jg   fa_done
    xor  r8, r8                     ; carry
    xor  r9, r9                     ; i
fa_inner:
    cmp  r9, r10
    jge  fa_carry
    mov  rax, [rdi + r9*8]
    mul  rbx                        ; rdx:rax = acc[i] * k
    add  rax, r8                    ; THE ADC PAIR: low half plus the carry in,
    adc  rdx, 0                     ; and the carry out folded into the high.
    mov  [rdi + r9*8], rax
    mov  r8, rdx
    lea  r9, [r9+1]
    jmp  fa_inner
fa_carry:
    test r8, r8
    jz   fa_next
    mov  [rdi + r10*8], r8
    lea  r10, [r10+1]
fa_next:
    lea  rbx, [rbx+1]
    jmp  fa_outer
fa_done:
    mov  rax, r10
    pop  rdi
    pop  rsi
    pop  rbx
    ret
fact_adc ENDP

fact_expl PROC
    push rbx
    push rsi
    push rdi
    mov  rsi, rcx
    mov  rdi, rdx
    xor  rax, rax
    mov  rcx, r8
    mov  r9, rdi
zf2:
    mov  qword ptr [r9], 0
    lea  r9, [r9+8]
    dec  rcx
    jnz  zf2
    mov  qword ptr [rdi], 1
    mov  r10, 1
    mov  rbx, 2
fe_outer:
    cmp  rbx, rsi
    jg   fe_done
    xor  r8, r8
    xor  r9, r9
fe_inner:
    cmp  r9, r10
    jge  fe_carry
    mov  rax, [rdi + r9*8]
    mul  rbx
    ; THE EXPLICIT FORM. No carry flag is carried between operations: the sum is
    ; computed, then COMPARED against one operand to detect wraparound, exactly
    ; as bits.Add64 and Long.compareUnsigned are written by hand elsewhere in
    ; this result. Three extra instructions and a dependency on the compare.
    mov  r11, rax
    add  rax, r8
    cmp  rax, r11
    setb cl
    movzx rcx, cl
    add  rdx, rcx
    mov  [rdi + r9*8], rax
    mov  r8, rdx
    lea  r9, [r9+1]
    jmp  fe_inner
fe_carry:
    test r8, r8
    jz   fe_next
    mov  [rdi + r10*8], r8
    lea  r10, [r10+1]
fe_next:
    lea  rbx, [rbx+1]
    jmp  fe_outer
fe_done:
    mov  rax, r10
    pop  rdi
    pop  rsi
    pop  rbx
    ret
fact_expl ENDP

; ---------------------------------------------------------------- fibonacci
;
; rcx = n, rdx = three consecutive buffers of `r8` limbs each. Returns rax =
; the low limb of fib(n), which is all the checksum needs.

fib_adc PROC
    push rbx
    push rsi
    push rdi
    push r12
    push r13
    push r14
    mov  r12, rcx                   ; n
    mov  r13, r8                    ; cap
    mov  rsi, rdx                   ; a
    lea  rdi, [rdx + r13*8]         ; b
    lea  r14, [rdi + r13*8]         ; t
    mov  rcx, r13
    add  rcx, r13
    add  rcx, r13                   ; three buffers, zeroed as one run
    mov  rax, rsi
zf3:
    mov  qword ptr [rax], 0
    lea  rax, [rax+8]
    dec  rcx
    jnz  zf3
    mov  qword ptr [rdi], 1         ; b = 1
ba_outer:
    test r12, r12
    jz   ba_done
    ; THE CARRY CHAIN. `lea` bumps the index and `dec` the counter, and neither
    ; touches CF — which is the whole reason this loop needs no carry variable.
    xor  r9, r9
    mov  rcx, r13
    clc
ba_inner:
    mov  rax, [rsi + r9*8]
    adc  rax, [rdi + r9*8]
    mov  [r14 + r9*8], rax
    lea  r9, [r9+1]
    dec  rcx
    jnz  ba_inner
    mov  rax, rsi                   ; rotate a,b,t
    mov  rsi, rdi
    mov  rdi, r14
    mov  r14, rax
    dec  r12
    jmp  ba_outer
ba_done:
    mov  rax, [rsi]
    pop  r14
    pop  r13
    pop  r12
    pop  rdi
    pop  rsi
    pop  rbx
    ret
fib_adc ENDP

fib_expl PROC
    push rbx
    push rsi
    push rdi
    push r12
    push r13
    push r14
    mov  r12, rcx
    mov  r13, r8
    mov  rsi, rdx
    lea  rdi, [rdx + r13*8]
    lea  r14, [rdi + r13*8]
    mov  rcx, r13
    add  rcx, r13
    add  rcx, r13                   ; three buffers, zeroed as one run
    mov  rax, rsi
zf4:
    mov  qword ptr [rax], 0
    lea  rax, [rax+8]
    dec  rcx
    jnz  zf4
    mov  qword ptr [rdi], 1
be_outer:
    test r12, r12
    jz   be_done
    xor  r9, r9
    xor  r8, r8                     ; carry as a VALUE
    mov  rcx, r13
be_inner:
    mov  rax, [rsi + r9*8]
    mov  r11, rax
    add  rax, [rdi + r9*8]
    cmp  rax, r11
    setb bl
    movzx rbx, bl
    mov  r11, rax
    add  rax, r8
    cmp  rax, r11
    setb r10b
    movzx r10, r10b
    add  rbx, r10
    mov  [r14 + r9*8], rax
    mov  r8, rbx
    lea  r9, [r9+1]
    dec  rcx
    jnz  be_inner
    mov  rax, rsi
    mov  rsi, rdi
    mov  rdi, r14
    mov  r14, rax
    dec  r12
    jmp  be_outer
be_done:
    mov  rax, [rsi]
    pop  r14
    pop  r13
    pop  r12
    pop  rdi
    pop  rsi
    pop  rbx
    ret
fib_expl ENDP

; fib_prim is the MIDDLE form, and it is the one that decides the design.
;
; The carry is materialised as an ordinary VALUE — so nothing has to survive
; between emitted statements — but it is produced with `adc r,0` rather than
; with a compare. That is what a DECLARED primitive could emit: values.md's
; multiple return gives us `(values sum carry)`, and x86 already passes two
; results in rax/rdx, so a `(prim add-carry (int int int) (int int) …)` is
; expressible with machinery that is built and measured.
;
; If this lands near fib_adc, the carry flag is not a hole in the model and one
; declaration closes it. If it lands near fib_expl, the implicit chain is doing
; the work and we cannot reach it.
fib_prim PROC
    push rbx
    push rsi
    push rdi
    push r12
    push r13
    push r14
    mov  r12, rcx
    mov  r13, r8
    mov  rsi, rdx
    lea  rdi, [rdx + r13*8]
    lea  r14, [rdi + r13*8]
    mov  rcx, r13
    add  rcx, r13
    add  rcx, r13
    mov  rax, rsi
zf5:
    mov  qword ptr [rax], 0
    lea  rax, [rax+8]
    dec  rcx
    jnz  zf5
    mov  qword ptr [rdi], 1
bp_outer:
    test r12, r12
    jz   bp_done
    xor  r9, r9
    xor  r8, r8
    mov  rcx, r13
bp_inner:
    mov  rax, [rsi + r9*8]
    xor  r11, r11
    add  rax, [rdi + r9*8]
    adc  r11, 0
    add  rax, r8
    adc  r11, 0
    mov  [r14 + r9*8], rax
    mov  r8, r11
    lea  r9, [r9+1]
    dec  rcx
    jnz  bp_inner
    mov  rax, rsi
    mov  rsi, rdi
    mov  rdi, r14
    mov  r14, rax
    dec  r12
    jmp  bp_outer
bp_done:
    mov  rax, [rsi]
    pop  r14
    pop  r13
    pop  r12
    pop  rdi
    pop  rsi
    pop  rbx
    ret
fib_prim ENDP

; ------------------------------------------------------------------ output

putstr PROC                          ; rcx = ptr, rdx = len
    push rbx
    push rsi
    push rdi
    sub  rsp, 40h
    mov  rsi, rcx
    mov  rdi, rdx
    mov  rcx, -11                    ; STD_OUTPUT_HANDLE
    call GetStdHandle
    mov  rcx, rax
    mov  rdx, rsi
    mov  r8, rdi
    lea  r9, [rsp+38h]
    mov  qword ptr [rsp+20h], 0
    call WriteFile
    add  rsp, 40h
    pop  rdi
    pop  rsi
    pop  rbx
    ret
putstr ENDP

putz PROC                            ; rcx = zero-terminated
    push rsi
    push rdi
    sub  rsp, 28h                    ; shadow space AND 16-byte alignment: two
    mov  rsi, rcx                    ; pushes leave rsp at 8 mod 16 for the call
    xor  rdx, rdx
pz1:
    cmp  byte ptr [rsi + rdx], 0
    je   pz2
    inc  rdx
    jmp  pz1
pz2:
    mov  rcx, rsi
    call putstr
    add  rsp, 28h
    pop  rdi
    pop  rsi
    ret
putz ENDP

putint PROC                          ; rcx = value
    push rbx
    push rsi
    push rdi
    sub  rsp, 40h
    mov  rax, rcx
    lea  rdi, [rsp+30h]
    mov  rbx, 10
    xor  rsi, rsi
pi1:
    xor  rdx, rdx
    div  rbx
    add  dl, '0'
    dec  rdi
    mov  [rdi], dl
    inc  rsi
    test rax, rax
    jnz  pi1
    mov  rcx, rdi
    mov  rdx, rsi
    call putstr
    add  rsp, 40h
    pop  rdi
    pop  rsi
    pop  rbx
    ret
putint ENDP

; -------------------------------------------------------------------- main
;
; Rounds are chosen so each form runs for a few hundred milliseconds, and the
; elapsed time is divided by the round count. QueryPerformanceCounter is used
; rather than timing the process, so process startup — 9 ms on this host per
; staticdata-2026-08-20 — never enters the number.

FACTN   equ 2000
FACTCAP equ 1024
FIBN    equ 1000
FIBCAP  equ 13
ROUNDS  equ 200

main PROC
    push rbx
    push rsi
    push rdi
    push r12
    push r13
    sub  rsp, 70h                    ; multiple of 16: five pushes already
                                     ; left rsp aligned

    lea  rcx, qpc_freq
    call QueryPerformanceFrequency

    mov  rcx, 0
    mov  rdx, 100000h                ; 1 MiB, enough for three fib buffers too
    mov  r8, 3000h                   ; MEM_COMMIT | MEM_RESERVE
    mov  r9, 4                       ; PAGE_READWRITE
    call VirtualAlloc
    mov  rsi, rax                    ; buf

    lea  rcx, lbl_fa
    mov  rdx, 0
    call one_fact
    lea  rcx, lbl_fe
    mov  rdx, 1
    call one_fact
    lea  rcx, lbl_ba
    mov  rdx, 0
    call one_fib
    lea  rcx, lbl_be
    mov  rdx, 1
    call one_fib
    lea  rcx, lbl_bp
    mov  rdx, 2
    call one_fib

    xor  rcx, rcx
    call ExitProcess
main ENDP

; one_fact: rcx = label, rdx = 0 for adc / 1 for explicit.
; Reads rsi = buf and the frequency slot from main's frame, both of which are
; live for the whole program — so they are read from registers and a global
; rather than by walking a frame, which is what the first version got wrong.
one_fact PROC
    push rbx
    push r12
    sub  rsp, 48h
    mov  rbx, rcx
    mov  r13, rdx

    lea  rcx, [rsp+20h]
    call QueryPerformanceCounter
    mov  r12, ROUNDS
of1:
    mov  rcx, FACTN
    mov  rdx, rsi
    mov  r8, FACTCAP
    test r13, r13
    jnz  of2
    call fact_adc
    jmp  of3
of2:
    call fact_expl
of3:
    mov  r14, rax                    ; used, kept before QPC clobbers rax
    dec  r12
    jnz  of1
    lea  rcx, [rsp+28h]
    call QueryPerformanceCounter

    mov  rcx, rbx
    mov  rdx, [rsp+20h]
    mov  r8, [rsp+28h]
    ; THE TOP LIMB, not the low one. 2000! is divisible by 2^1994, so limb 0 is
    ; zero on every correct implementation AND on most wrong ones — a checksum
    ; that cannot fail is not a checksum.
    lea  r14, [r14-1]
    mov  r9, [rsi + r14*8]
    call report
    add  rsp, 48h
    pop  r12
    pop  rbx
    ret
one_fact ENDP

one_fib PROC
    push rbx
    push r12
    sub  rsp, 48h
    mov  rbx, rcx
    mov  r13, rdx

    lea  rcx, [rsp+20h]
    call QueryPerformanceCounter
    mov  r12, ROUNDS
ob1:
    mov  rcx, FIBN
    mov  rdx, rsi
    mov  r8, FIBCAP
    cmp  r13, 0
    jne  ob2
    call fib_adc
    jmp  ob3
ob2:
    cmp  r13, 1
    jne  ob4
    call fib_expl
    jmp  ob3
ob4:
    call fib_prim
ob3:
    mov  r14, rax                    ; the low limb, kept before QPC clobbers rax
    dec  r12
    jnz  ob1
    lea  rcx, [rsp+28h]
    call QueryPerformanceCounter

    mov  rcx, rbx
    mov  rdx, [rsp+20h]
    mov  r8, [rsp+28h]
    mov  r9, r14
    call report
    add  rsp, 48h
    pop  r12
    pop  rbx
    ret
one_fib ENDP

; report: rcx = label, rdx = t0, r8 = t1, r9 = checksum. The frequency is read
; from the global set by main. Everything arrives as an ARGUMENT, which is the
; fix for the first version reading offsets into a frame it did not own.
report PROC
    push rbx
    push rsi
    push rdi
    sub  rsp, 30h
    mov  rbx, rcx
    mov  rsi, r9
    mov  rax, r8
    sub  rax, rdx                    ; ticks
    mov  rdx, 1000000000
    mul  rdx                         ; ticks * 1e9  (rdx:rax)
    div  qword ptr [qpc_freq]
    xor  rdx, rdx
    mov  rcx, ROUNDS
    div  rcx
    mov  rdi, rax                    ; ns per round

    mov  rcx, rbx
    call putz
    mov  rcx, rdi
    call putint
    lea  rcx, lbl_ns
    call putz
    mov  rcx, rsi
    call putint
    lea  rcx, nl
    mov  rdx, 2
    call putstr
    add  rsp, 30h
    pop  rdi
    pop  rsi
    pop  rbx
    ret
report ENDP

end
