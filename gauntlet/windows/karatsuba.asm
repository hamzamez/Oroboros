; KARATSUBA WITHOUT RECURSION, IN PLACE, ON x86-64. Hand-written MASM.
;
; The port of gauntlet/go/karatsuba2.go to the one host with no bignum of its
; own, where "as fast as possible" is the whole brief because we control
; everything. Three things this host has that the others do not:
;
;   mul   64x64 -> 128 in one instruction, high half in rdx.
;   adc   the carry lives in a FLAG, so a limb-vector add needs no carry
;         variable — which is why every loop below bumps its index with `lea`
;         and its counter with `dec`, the two instructions that leave CF alone.
;   sbb   the same for the two subtractions in the Karatsuba combine.
;
; AND ONE STRUCTURAL WIN THE OTHER PORTS MISS. The descriptor table — every
; node's (aOff, bOff, len) — is a function of (n, D) ALONE. It does not depend
; on the operands, so it is computed ONCE in setup and the timed path never
; touches it. The Go, Java and JavaScript versions rebuild it on every multiply
; because it was cheap enough to ignore there; here it is free to hoist.
;
;   kara_setup(n, D, ws)   once
;   kara_mul(ws, a, b)     the three passes, timed
;
; Layout inside ws, all qword arrays, carved linearly:
;   lenOf[D+1] prodOf[D+1] baseIdx[D+2] aOff[N] bOff[N] ln[N] pOff[N] arena prod
;
; Build:  build-karatsuba.bat
; Prints ns/round and a checksum that must match every other host.

option casemap:none

extern VirtualAlloc: proc
extern GetStdHandle: proc
extern WriteFile: proc
extern ExitProcess: proc
extern QueryPerformanceCounter: proc
extern QueryPerformanceFrequency: proc

.data
    lbl_s    db "schoolbook   ", 0
    lbl_k    db "karatsuba D=", 0
    lbl_ns   db "  ", 0
    lbl_sum  db " ns/round  sum=", 0
    nl       db 13, 10
    align 8
    qpc_freq dq 0

.code

; ---------------------------------------------------------------- kernels

; k_zero(rcx=ptr, rdx=n)
k_zero PROC
    test rdx, rdx
    jz   kz_d
    xor  rax, rax
    xor  r10, r10
kz_1:
    mov  [rcx + r10*8], rax
    lea  r10, [r10+1]
    dec  rdx
    jnz  kz_1
kz_d:
    ret
k_zero ENDP

; k_copy(rcx=dst, rdx=src, r8=n)
k_copy PROC
    test r8, r8
    jz   kc_d
    xor  r10, r10
kc_1:
    mov  rax, [rdx + r10*8]
    mov  [rcx + r10*8], rax
    lea  r10, [r10+1]
    dec  r8
    jnz  kc_1
kc_d:
    ret
k_copy ENDP

; k_addat(rcx=dst, rdx=dstLen, r8=src, r9=srcLen)
; dst[0..] += src, carrying to the end of dst. THE CARRY IS THE FLAG: `lea` and
; `dec` are chosen because neither touches CF.
k_addat PROC
    test r9, r9
    jz   ka_d
    mov  r11, r9
    xor  r10, r10
    clc
ka_1:
    mov  rax, [r8 + r10*8]
    adc  [rcx + r10*8], rax
    lea  r10, [r10+1]
    dec  r11
    jnz  ka_1
    jnc  ka_d
ka_2:
    cmp  r10, rdx
    jge  ka_d
    add  qword ptr [rcx + r10*8], 1
    lea  r10, [r10+1]
    jc   ka_2
ka_d:
    ret
k_addat ENDP

; k_subat(rcx=dst, rdx=dstLen, r8=src, r9=srcLen)
k_subat PROC
    test r9, r9
    jz   ks_d
    mov  r11, r9
    xor  r10, r10
    clc
kb_1:
    mov  rax, [r8 + r10*8]
    sbb  [rcx + r10*8], rax
    lea  r10, [r10+1]
    dec  r11
    jnz  kb_1
    jnc  ks_d
kb_2:
    cmp  r10, rdx
    jge  ks_d
    sub  qword ptr [rcx + r10*8], 1
    lea  r10, [r10+1]
    jc   kb_2
ks_d:
    ret
k_subat ENDP

; k_mac(rcx=out, rdx=b, r8=n, r9=m) -> rax = carry
;   out[0..n) += m * b[0..n)
;
; THE INNER LOOP IS THE WHOLE GAP. Go's math/big has no Toom-Cook: at 1024 words
; it runs the same Karatsuba we do, and it is 1.91x faster purely because
; `addMulVVW` is hand-written MULX/ADOX/ADCX. The naive form serialises on
; mul's fixed rdx:rax and on one carry chain:
;
;       mul  [b+j]        ; 3-4 cycle latency, fixed registers
;       add  rax, carry
;       adc  rdx, 0
;       add  [out+j], rax
;       adc  rdx, 0
;
; Three instructions fix it. MULX writes two CHOSEN registers and touches no
; flags, so several can be in flight. ADCX and ADOX carry through CF and OF
; respectively, which are INDEPENDENT — so two accumulation chains proceed at
; once instead of one.
;
; The catch: `dec` and `cmp` write OF, so ordinary loop control destroys the
; ADOX chain. The answer is to keep both chains inside an UNROLLED BLOCK and
; fold them into the running carry at the block boundary, where the flags are
; dead and the loop counter is free to use them.
k_mac PROC
    push rbx
    push rsi
    push rdi
    push r12
    push r13
    push r14
    push r15
    sub  rsp, 20h
    mov  [rsp], r8                  ; n, kept off the register file: the
                                    ; unrolled block uses every other register
    mov  rsi, rdx                   ; b
    mov  rdx, r9                    ; m — mulx's implicit multiplier
    xor  rbx, rbx                   ; running carry
    mov  rdi, r8
    shr  rdi, 2                     ; blocks of four
    xor  r9, r9                     ; j
    test rdi, rdi
    jz   km_tail
km_blk:
    xor  rax, rax                   ; CF = OF = 0, and the zero we fold with
    mulx r11, r10, qword ptr [rsi + r9*8]
    adox r10, qword ptr [rcx + r9*8]
    adcx r10, rbx
    mov  qword ptr [rcx + r9*8], r10

    mulx r13, r12, qword ptr [rsi + r9*8 + 8]
    adox r12, qword ptr [rcx + r9*8 + 8]
    adcx r12, r11
    mov  qword ptr [rcx + r9*8 + 8], r12

    mulx r15, r14, qword ptr [rsi + r9*8 + 16]
    adox r14, qword ptr [rcx + r9*8 + 16]
    adcx r14, r13
    mov  qword ptr [rcx + r9*8 + 16], r14

    mulx rbx, r10, qword ptr [rsi + r9*8 + 24]
    adox r10, qword ptr [rcx + r9*8 + 24]
    adcx r10, r15
    mov  qword ptr [rcx + r9*8 + 24], r10

    ; Fold both chains into the carry. rbx <= 2^64-2, so +2 cannot overflow.
    adcx rbx, rax
    adox rbx, rax

    lea  r9, [r9+4]
    dec  rdi
    jnz  km_blk
km_tail:
    ; The remainder, plain: base-case sizes are not multiples of four.
    mov  r8, [rsp]
km_t1:
    cmp  r9, r8
    jge  km_done
    mulx r11, r10, qword ptr [rsi + r9*8]
    add  r10, rbx
    adc  r11, 0
    add  qword ptr [rcx + r9*8], r10
    adc  r11, 0
    mov  rbx, r11
    lea  r9, [r9+1]
    jmp  km_t1
km_done:
    mov  rax, rbx
    add  rsp, 20h
    pop  r15
    pop  r14
    pop  r13
    pop  r12
    pop  rdi
    pop  rsi
    pop  rbx
    ret
k_mac ENDP

; k_school(rcx=a, rdx=b, r8=n, r9=out) — out is 2n limbs and is zeroed here.
k_school PROC
    push rbx
    push rsi
    push rdi
    push r12
    push r13
    push r14
    sub  rsp, 30h
    mov  rsi, rcx                   ; a
    mov  rdi, rdx                   ; b
    mov  r13, r8                    ; n
    mov  rbx, r9                    ; out
    ; NO ZERO HERE. The slot is prodOf[D] limbs and 2n is SMALLER for the lo and
    ; hi children, whose ln is lenOf[D]-1 — so zeroing 2n would leave stale limbs
    ; at the top of the slot, which the combine then reads. The caller zeroes the
    ; whole slot instead. This was latent: the benchmark multiplies the same
    ; operands every round, so a stale limb held exactly the value it should.
    xor  r14, r14                   ; i
sc_outer:
    cmp  r14, r13
    jge  sc_done
    lea  rcx, [rbx + r14*8]         ; &out[i]
    mov  rdx, rdi                   ; b
    mov  r8, r13                    ; n
    mov  r9, [rsi + r14*8]          ; a[i]
    call k_mac
    ; out[i+n] = carry. The slot is untouched by this row and by every earlier
    ; one, so it is an assignment rather than an accumulate.
    mov  rcx, r14
    add  rcx, r13
    mov  [rbx + rcx*8], rax
    lea  r14, [r14+1]
    jmp  sc_outer
sc_done:
    add  rsp, 30h
    pop  r14
    pop  r13
    pop  r12
    pop  rdi
    pop  rsi
    pop  rbx
    ret
k_school ENDP

; ------------------------------------------------------------- workspace
;
; ws header, 16 qwords: n D nodes lenOf prodOf baseIdx aOff bOff ln pOff
;                       arena prod arenaLen prodLen - -

WS_N      equ 0
WS_D      equ 8
WS_NODES  equ 16
WS_LENOF  equ 24
WS_PRODOF equ 32
WS_BASE   equ 40
WS_AOFF   equ 48
WS_BOFF   equ 56
WS_LN     equ 64
WS_POFF   equ 72
WS_ARENA  equ 80
WS_PROD   equ 88
WS_HDR    equ 128

; kara_setup(rcx=n, rdx=D, r8=ws)
; Computes every size and EVERY DESCRIPTOR. Nothing here depends on the
; operands, which is exactly why it can be hoisted out of the timed path.
kara_setup PROC
    push rbx
    push rsi
    push rdi
    push r12
    push r13
    push r14
    push r15
    sub  rsp, 0C0h

    mov  rbx, r8                    ; ws
    mov  [rbx + WS_N], rcx
    mov  [rbx + WS_D], rdx
    mov  r14, rcx                   ; n
    mov  r15, rdx                   ; D

    lea  rsi, [rbx + WS_HDR]        ; bump pointer

    mov  [rbx + WS_LENOF], rsi      ; lenOf[D+1]
    lea  rsi, [rsi + r15*8 + 8]
    mov  [rbx + WS_PRODOF], rsi     ; prodOf[D+1]
    lea  rsi, [rsi + r15*8 + 8]
    mov  [rbx + WS_BASE], rsi       ; baseIdx[D+2]
    lea  rsi, [rsi + r15*8 + 16]

    ; lenOf[0]=n ; lenOf[L+1] = (lenOf[L] - lenOf[L]/2) + 1
    mov  rdi, [rbx + WS_LENOF]
    mov  [rdi], r14
    xor  r8, r8
sl_1:
    cmp  r8, r15
    jge  sl_2
    mov  rax, [rdi + r8*8]
    mov  rcx, rax
    shr  rcx, 1                     ; h
    sub  rax, rcx
    inc  rax
    mov  [rdi + r8*8 + 8], rax
    inc  r8
    jmp  sl_1
sl_2:
    ; baseIdx and node count
    mov  rdi, [rbx + WS_BASE]
    xor  r8, r8                     ; L
    xor  r9, r9                     ; acc
    mov  r10, 1                     ; 3^L
sb_1:
    cmp  r8, r15
    jg   sb_2
    mov  [rdi + r8*8], r9
    add  r9, r10
    lea  r10, [r10 + r10*2]
    inc  r8
    jmp  sb_1
sb_2:
    mov  [rdi + r8*8], r9
    mov  [rbx + WS_NODES], r9
    mov  r13, r9                    ; nodes

    mov  [rbx + WS_AOFF], rsi
    lea  rsi, [rsi + r13*8]
    mov  [rbx + WS_BOFF], rsi
    lea  rsi, [rsi + r13*8]
    mov  [rbx + WS_LN], rsi
    lea  rsi, [rsi + r13*8]
    mov  [rbx + WS_POFF], rsi
    lea  rsi, [rsi + r13*8]

    ; prodOf[D] = 2*lenOf[D]; prodOf[L] = 2*(lenOf[L]/2) + prodOf[L+1]
    mov  rdi, [rbx + WS_PRODOF]
    mov  rcx, [rbx + WS_LENOF]
    mov  rax, [rcx + r15*8]
    add  rax, rax
    mov  [rdi + r15*8], rax
    mov  r8, r15
sp_1:
    test r8, r8
    jz   sp_2
    dec  r8
    mov  rax, [rcx + r8*8]
    shr  rax, 1
    add  rax, rax
    add  rax, [rdi + r8*8 + 8]
    mov  [rdi + r8*8], rax
    jmp  sp_1
sp_2:
    ; THE PROD LAYOUT IS TILED NOW, so pOff is assigned in the descriptor pass
    ; below rather than by a running total here. Every slot is exactly 2*ln
    ; limbs, and that is what makes child 0 and child 1 tile the parent's
    ; output exactly: [0, 2h) and [2h, 2l), nothing left over and no copies.
    mov  [rbx + WS_PROD], rsi
    ; Size it: the root's 2n, plus one fresh slot per internal node for its
    ; SUM child, each 2*lenOf[L+1] limbs. The other two children need none.
    mov  r11, r14
    add  r11, r14                   ; 2n
    xor  r8, r8
    mov  r10, 1
    mov  rcx, [rbx + WS_LENOF]
sz_1:
    cmp  r8, r15
    jge  sz_2
    mov  rax, [rcx + r8*8 + 8]
    add  rax, rax
    imul rax, r10
    add  r11, rax
    lea  r10, [r10 + r10*2]
    inc  r8
    jmp  sz_1
sz_2:
    lea  rsi, [rsi + r11*8]
    mov  [rbx + WS_ARENA], rsi

    ; DESCRIPTORS. aOff[0]=0, bOff[0]=n, ln[0]=n, then three children per node.
    mov  r12, [rbx + WS_AOFF]
    mov  rcx, [rbx + WS_BOFF]
    mov  [rsp+20h], rcx
    mov  rcx, [rbx + WS_LN]
    mov  [rsp+28h], rcx
    mov  qword ptr [r12], 0
    mov  rcx, [rsp+20h]
    mov  [rcx], r14
    mov  rcx, [rsp+28h]
    mov  [rcx], r14
    mov  rcx, [rbx + WS_POFF]
    mov  qword ptr [rcx], 0         ; the root's product is at 0
    mov  rcx, r14
    add  rcx, r14
    mov  [rsp+60h], rcx             ; prodFree = 2n
    mov  r11, r14
    add  r11, r14                   ; arena free = 2n
    xor  r8, r8                     ; L
    mov  r10, 1                     ; 3^L
sd_1:
    cmp  r8, r15
    jge  sd_done
    mov  rdx, [rbx + WS_BASE]
    mov  r9, [rdx + r8*8]           ; base
    mov  rdi, [rdx + r8*8 + 8]      ; cbase
    mov  rcx, [rbx + WS_LENOF]
    mov  rsi, [rcx + r8*8 + 8]      ; cl
    xor  rax, rax                   ; k
sd_2:
    cmp  rax, r10
    jge  sd_3
    ; id = base+k
    mov  rcx, r9
    add  rcx, rax
    mov  [rsp+30h], rcx             ; id
    ; c0 = cbase + 3k
    mov  rdx, rax
    lea  rdx, [rdx + rdx*2]
    add  rdx, rdi
    mov  [rsp+38h], rdx             ; c0

    mov  rcx, [rsp+30h]
    mov  rdx, [r12 + rcx*8]         ; ao
    mov  [rsp+40h], rdx
    mov  rdx, [rsp+20h]
    mov  rdx, [rdx + rcx*8]         ; bo
    mov  [rsp+48h], rdx
    mov  rdx, [rsp+28h]
    mov  rdx, [rdx + rcx*8]         ; l
    mov  [rsp+50h], rdx
    shr  rdx, 1                     ; h
    mov  [rsp+58h], rdx

    mov  rcx, [rsp+38h]             ; c0
    ; child 0 = (ao, bo, h)
    mov  rdx, [rsp+40h]
    mov  [r12 + rcx*8], rdx
    mov  rdx, [rsp+20h]
    mov  r14, [rsp+48h]
    mov  [rdx + rcx*8], r14
    mov  rdx, [rsp+28h]
    mov  r14, [rsp+58h]
    mov  [rdx + rcx*8], r14
    ; child 1 = (ao+h, bo+h, l-h)
    mov  rdx, [rsp+40h]
    add  rdx, [rsp+58h]
    mov  [r12 + rcx*8 + 8], rdx
    mov  rdx, [rsp+48h]
    add  rdx, [rsp+58h]
    mov  r14, [rsp+20h]
    mov  [r14 + rcx*8 + 8], rdx
    mov  rdx, [rsp+50h]
    sub  rdx, [rsp+58h]
    mov  r14, [rsp+28h]
    mov  [r14 + rcx*8 + 8], rdx
    ; child 2 = (free, free+cl, cl)
    mov  [r12 + rcx*8 + 16], r11
    mov  rdx, r11
    add  rdx, rsi
    mov  r14, [rsp+20h]
    mov  [r14 + rcx*8 + 16], rdx
    mov  r14, [rsp+28h]
    mov  [r14 + rcx*8 + 16], rsi
    add  r11, rsi
    add  r11, rsi                   ; arena free += 2cl

    ; PRODUCT SLOTS. Child 0 lands on the parent's output at 0 and child 1 at
    ; 2h, which tiles [0, 2l) exactly because a slot is 2*ln. Only the sum
    ; child needs storage of its own.
    ; rax is the loop counter here, so r13 is the scratch.
    mov  r14, [rbx + WS_POFF]
    mov  rdx, [rsp+30h]             ; id
    mov  rdx, [r14 + rdx*8]         ; parent pOff
    mov  [r14 + rcx*8], rdx         ; child 0 lands on it
    mov  r13, [rsp+58h]
    add  r13, r13                   ; 2h
    add  r13, rdx
    mov  [r14 + rcx*8 + 8], r13     ; child 1 at 2h
    mov  rdx, [rsp+60h]
    mov  [r14 + rcx*8 + 16], rdx    ; the sum child, its own slot
    mov  r13, rsi
    add  r13, r13                   ; 2*cl
    add  rdx, r13
    mov  [rsp+60h], rdx             ; prodFree += 2cl

    mov  r14, [rbx + WS_N]
    inc  rax
    jmp  sd_2
sd_3:
    lea  r10, [r10 + r10*2]
    inc  r8
    jmp  sd_1
sd_done:
    add  rsp, 0C0h
    pop  r15
    pop  r14
    pop  r13
    pop  r12
    pop  rdi
    pop  rsi
    pop  rbx
    ret
kara_setup ENDP

; kara_mul(rcx=ws, rdx=a, r8=b) — the timed path. Descriptors already exist.
kara_mul PROC
    push rbx
    push rsi
    push rdi
    push r12
    push r13
    push r14
    push r15
    sub  rsp, 0C0h

    mov  rbx, rcx
    mov  rsi, [rbx + WS_ARENA]
    mov  rdi, [rbx + WS_PROD]
    mov  r12, [rbx + WS_AOFF]
    mov  r13, [rbx + WS_BOFF]
    mov  r14, [rbx + WS_LN]
    mov  r15, [rbx + WS_POFF]
    mov  [rsp+88h], rdx             ; a
    mov  [rsp+90h], r8              ; b

    mov  rcx, rsi
    mov  rdx, [rsp+88h]
    mov  r8, [rbx + WS_N]
    call k_copy
    mov  rax, [rbx + WS_N]
    lea  rcx, [rsi + rax*8]
    mov  rdx, [rsp+90h]
    mov  r8, rax
    call k_copy

    ; ---- DOWNWARD: only the sum child is written.
    xor  rax, rax
    mov  [rsp+20h], rax             ; L
    mov  qword ptr [rsp+30h], 1     ; 3^L
md_1:
    mov  rax, [rsp+20h]
    cmp  rax, [rbx + WS_D]
    jge  md_done
    mov  rdx, [rbx + WS_BASE]
    mov  rcx, [rdx + rax*8]
    mov  [rsp+38h], rcx             ; base
    mov  rcx, [rdx + rax*8 + 8]
    mov  [rsp+40h], rcx             ; cbase
    xor  rax, rax
    mov  [rsp+28h], rax             ; k
md_2:
    mov  rax, [rsp+28h]
    cmp  rax, [rsp+30h]
    jge  md_3
    mov  rcx, [rsp+38h]
    add  rcx, rax                   ; id
    mov  rdx, rax
    lea  rdx, [rdx + rdx*2]
    add  rdx, [rsp+40h]             ; c0
    mov  [rsp+48h], rdx

    mov  r8, [r12 + rcx*8]          ; ao
    mov  r9, [r13 + rcx*8]          ; bo
    mov  r10, [r14 + rcx*8]         ; l
    mov  r11, r10
    shr  r11, 1                     ; h
    mov  [rsp+50h], r8
    mov  [rsp+58h], r9
    mov  [rsp+60h], r10
    mov  [rsp+68h], r11

    mov  rdx, [rsp+48h]
    mov  rax, [r14 + rdx*8 + 16]    ; cl
    mov  [rsp+70h], rax
    mov  rax, [r12 + rdx*8 + 16]    ; as
    mov  [rsp+78h], rax
    mov  rax, [r13 + rdx*8 + 16]    ; bs
    mov  [rsp+80h], rax

    ; a-sum
    mov  rax, [rsp+78h]
    lea  rcx, [rsi + rax*8]
    mov  rdx, [rsp+70h]
    call k_zero
    mov  rax, [rsp+78h]
    lea  rcx, [rsi + rax*8]
    mov  rax, [rsp+50h]
    add  rax, [rsp+68h]
    lea  rdx, [rsi + rax*8]
    mov  r8, [rsp+60h]
    sub  r8, [rsp+68h]
    call k_copy
    mov  rax, [rsp+78h]
    lea  rcx, [rsi + rax*8]
    mov  rdx, [rsp+70h]
    mov  rax, [rsp+50h]
    lea  r8, [rsi + rax*8]
    mov  r9, [rsp+68h]
    call k_addat
    ; b-sum
    mov  rax, [rsp+80h]
    lea  rcx, [rsi + rax*8]
    mov  rdx, [rsp+70h]
    call k_zero
    mov  rax, [rsp+80h]
    lea  rcx, [rsi + rax*8]
    mov  rax, [rsp+58h]
    add  rax, [rsp+68h]
    lea  rdx, [rsi + rax*8]
    mov  r8, [rsp+60h]
    sub  r8, [rsp+68h]
    call k_copy
    mov  rax, [rsp+80h]
    lea  rcx, [rsi + rax*8]
    mov  rdx, [rsp+70h]
    mov  rax, [rsp+58h]
    lea  r8, [rsi + rax*8]
    mov  r9, [rsp+68h]
    call k_addat

    mov  rax, [rsp+28h]
    inc  rax
    mov  [rsp+28h], rax
    jmp  md_2
md_3:
    mov  rax, [rsp+30h]
    lea  rax, [rax + rax*2]
    mov  [rsp+30h], rax
    mov  rax, [rsp+20h]
    inc  rax
    mov  [rsp+20h], rax
    jmp  md_1
md_done:

    ; ---- BASE CASE
    mov  rax, [rbx + WS_D]
    mov  rdx, [rbx + WS_BASE]
    mov  rcx, [rdx + rax*8]
    mov  [rsp+38h], rcx             ; base
    mov  rcx, [rdx + rax*8 + 8]
    sub  rcx, [rsp+38h]
    mov  [rsp+30h], rcx             ; count = 3^D
    xor  rax, rax
    mov  [rsp+28h], rax
mb_1:
    mov  rax, [rsp+28h]
    cmp  rax, [rsp+30h]
    jge  mb_2
    mov  rcx, [rsp+38h]
    add  rcx, rax                   ; id
    mov  rax, [r12 + rcx*8]
    lea  r10, [rsi + rax*8]
    mov  rax, [r13 + rcx*8]
    lea  r11, [rsi + rax*8]
    mov  r8, [r14 + rcx*8]
    mov  rax, [r15 + rcx*8]
    lea  r9, [rdi + rax*8]
    mov  [rsp+40h], r10
    mov  [rsp+48h], r11
    mov  [rsp+50h], r8
    mov  [rsp+58h], r9
    ; zero the slot, which is now exactly 2*ln limbs
    mov  rcx, r9
    mov  rdx, [rsp+50h]
    add  rdx, rdx
    call k_zero
    mov  rcx, [rsp+40h]
    mov  rdx, [rsp+48h]
    mov  r8, [rsp+50h]
    mov  r9, [rsp+58h]
    call k_school
    mov  rax, [rsp+28h]
    inc  rax
    mov  [rsp+28h], rax
    jmp  mb_1
mb_2:

    ; ---- UPWARD
    mov  rax, [rbx + WS_D]
    dec  rax
    mov  [rsp+20h], rax             ; L
mu_1:
    mov  rax, [rsp+20h]
    cmp  rax, 0
    jl   mu_done
    mov  rdx, [rbx + WS_BASE]
    mov  rcx, [rdx + rax*8]
    mov  [rsp+38h], rcx             ; base
    mov  rcx, [rdx + rax*8 + 8]
    mov  [rsp+40h], rcx             ; cbase
    sub  rcx, [rsp+38h]
    mov  [rsp+30h], rcx             ; 3^L
    mov  rdx, [rbx + WS_PRODOF]
    mov  rcx, [rdx + rax*8]
    mov  [rsp+78h], rcx             ; sz
    mov  rcx, [rdx + rax*8 + 8]
    mov  [rsp+70h], rcx             ; csz
    xor  rax, rax
    mov  [rsp+28h], rax
mu_2:
    mov  rax, [rsp+28h]
    cmp  rax, [rsp+30h]
    jge  mu_3
    mov  rcx, [rsp+38h]
    add  rcx, rax                   ; id
    ; THE COMBINE, IN THREE PASSES. z0 and z1 are already in the destination —
    ; child 0's slot IS out[0..2h) and child 1's IS out[2h..2l), because a slot
    ; is exactly 2*ln and the two tile the parent. Nothing is copied and nothing
    ; is zeroed: the children wrote every limb.
    ;
    ;     z2 -= z0 ;  z2 -= z1 ;  out[h..] += z2
    ;
    ; The subtractions go into z2's OWN buffer, which is the only ordering that
    ; works: doing them in place would read out[0..2h) while writing out[h..),
    ; and those overlap. z2 >= z0 + z1 by construction, so no borrow escapes.
    mov  r10, [r14 + rcx*8]         ; l
    mov  r11, r10
    shr  r11, 1                     ; h
    mov  [rsp+68h], r11
    mov  r9, r11
    add  r9, r9                     ; lo2 = 2h
    mov  [rsp+48h], r9
    sub  r10, r11
    add  r10, r10                   ; hi2 = 2(l-h)
    mov  [rsp+60h], r10
    mov  r9, [rsp+48h]
    add  r9, r10                    ; 2l = out's length
    mov  [rsp+78h], r9
    mov  rax, [r15 + rcx*8]
    lea  r11, [rdi + rax*8]
    mov  [rsp+50h], r11             ; out

    mov  rax, [rsp+28h]
    lea  rax, [rax + rax*2]
    add  rax, [rsp+40h]             ; c0 index
    mov  [rsp+88h], rax
    mov  r9, [r15 + rax*8]
    lea  r9, [rdi + r9*8]
    mov  [rsp+58h], r9              ; z0 = out
    mov  r9, [r15 + rax*8 + 8]
    lea  r9, [rdi + r9*8]
    mov  [rsp+80h], r9              ; z1
    mov  r9, [r15 + rax*8 + 16]
    lea  r9, [rdi + r9*8]
    mov  [rsp+90h], r9              ; z2
    mov  r9, [r14 + rax*8 + 16]
    add  r9, r9                     ; z2 length = 2*ln[c2]
    mov  [rsp+70h], r9

    mov  rcx, [rsp+90h]
    mov  rdx, [rsp+70h]
    mov  r8, [rsp+58h]
    mov  r9, [rsp+48h]
    call k_subat                    ; z2 -= z0

    mov  rcx, [rsp+90h]
    mov  rdx, [rsp+70h]
    mov  r8, [rsp+80h]
    mov  r9, [rsp+60h]
    call k_subat                    ; z2 -= z1

    mov  rax, [rsp+68h]
    mov  rcx, [rsp+50h]
    lea  rcx, [rcx + rax*8]
    mov  rdx, [rsp+78h]
    sub  rdx, rax
    mov  r8, [rsp+90h]
    mov  r9, [rsp+70h]
    call k_addat                    ; out[h..] += z2

    mov  rax, [rsp+28h]
    inc  rax
    mov  [rsp+28h], rax
    jmp  mu_2
mu_3:
    mov  rax, [rsp+20h]
    dec  rax
    mov  [rsp+20h], rax
    jmp  mu_1
mu_done:
    add  rsp, 0C0h
    pop  r15
    pop  r14
    pop  r13
    pop  r12
    pop  rdi
    pop  rsi
    pop  rbx
    ret
kara_mul ENDP

; ------------------------------------------------------------------ output

putstr PROC                          ; rcx = ptr, rdx = len
    push rbx
    push rsi
    push rdi
    sub  rsp, 40h
    mov  rsi, rcx
    mov  rdi, rdx
    mov  rcx, -11
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
    sub  rsp, 28h
    mov  rsi, rcx
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

NLIMBS equ 1024
ROUNDS equ 200

main PROC
    push rbx
    push rsi
    push rdi
    push r12
    push r13
    push r14
    push r15
    sub  rsp, 0A0h

    lea  rcx, qpc_freq
    call QueryPerformanceFrequency

    mov  rcx, 0
    mov  rdx, 4000000h               ; 64 MiB
    mov  r8, 3000h
    mov  r9, 4
    call VirtualAlloc
    mov  rbx, rax                    ; ws

    ; operands after the workspace, at a generous fixed offset
    lea  rsi, [rbx + 3000000h]       ; a
    lea  rdi, [rsi + NLIMBS*8]       ; b

    ; THE SAME GENERATOR AS gauntlet/go/bigarith.go's LimbsOf, so the checksum
    ; can be compared against the other hosts rather than only with itself.
    mov  r14, 12345
    mov  rcx, rsi
    call genlimbs
    mov  r14, 67890
    mov  rcx, rdi
    call genlimbs

    xor  r15, r15                    ; D index
mn_1:
    cmp  r15, 7
    jge  mn_done
    mov  r13, r15
    add  r13, r13
    cmp  r15, 0
    jne  mn_h2
    xor  r13, r13
    jmp  mn_have
mn_h2:
    mov  r13, r15
    add  r13, 3                     ; D = 4,5,6,7,8,9 for indices 1..6
    dec  r13
mn_have:
    mov  rcx, NLIMBS
    mov  rdx, r13
    mov  r8, rbx
    call kara_setup

    lea  rcx, [rsp+20h]
    call QueryPerformanceCounter
    mov  r12, ROUNDS
mn_2:
    mov  rcx, rbx
    mov  rdx, rsi
    mov  r8, rdi
    call kara_mul
    dec  r12
    jnz  mn_2
    lea  rcx, [rsp+28h]
    call QueryPerformanceCounter

    ; label
    test r13, r13
    jnz  mn_lk
    lea  rcx, lbl_s
    call putz
    jmp  mn_num
mn_lk:
    lea  rcx, lbl_k
    call putz
    mov  rcx, r13
    call putint
    lea  rcx, lbl_ns
    call putz
mn_num:
    mov  rax, [rsp+28h]
    sub  rax, [rsp+20h]
    mov  rdx, 1000000000
    mul  rdx
    div  qword ptr [qpc_freq]
    xor  rdx, rdx
    mov  rcx, ROUNDS
    div  rcx
    mov  rcx, rax
    call putint
    lea  rcx, lbl_sum
    call putz
    ; checksum: the TOP limb of the 2n-limb product
    mov  rax, [rbx + WS_PROD]
    mov  rcx, [rax + (2*NLIMBS-1)*8]
    call putint
    lea  rcx, nl
    mov  rdx, 2
    call putstr

    inc  r15
    jmp  mn_1
mn_done:
    xor  rcx, rcx
    call ExitProcess
main ENDP

; genlimbs(rcx = dst), r14 = seed. Matches Go's LimbsOf exactly.
genlimbs PROC
    push rbx
    push rsi
    mov  rsi, rcx
    mov  rax, r14
    mov  rbx, 8000000000000000h
    or   rax, rbx                    ; x = seed | 1<<63
    mov  r10, NLIMBS
    xor  r11, r11
gl_1:
    mov  r9, 6364136223846793005
    mul  r9                          ; rdx:rax, low half is what we want
    mov  r9, 1442695040888963407
    add  rax, r9
    mov  r8, rax
    or   r8, rbx
    mov  [rsi + r11*8], r8
    lea  r11, [r11+1]
    dec  r10
    jnz  gl_1
    pop  rsi
    pop  rbx
    ret
genlimbs ENDP

end
