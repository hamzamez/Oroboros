; HAND-WRITTEN x86-64. This is the bar.
;
; The same program as examples/native/sieve-win-bench.oro: 100 rounds of a
; 200000-element sieve of Eratosthenes, summed and printed in decimal.
;
; Written the way a person writes it, NOT in the shape the emitter happens to
; produce — CLAUDE.md is explicit that the comparison is against the best
; hand-written form. Two things here are things the emitted version does not do:
;
;   1. i*i is computed ONCE per outer iteration and kept in rdi, serving both
;      the loop guard and the inner loop's starting value. The emitted version
;      computes it twice, because the two appear in different subterms and
;      nothing in the pipeline does common-subexpression elimination.
;
;   2. The decimal conversion divides once per digit. So does the emitted one —
;      but the emitted one divides TWICE per digit, once for the remainder and
;      once for the quotient, because x86's single idiv produces both and the
;      language has no product type to receive them with.
;
; STACK ALIGNMENT was wrong here twice on the first attempt — in putint and in
; main — and the program printed the right answer and then crashed inside
; kernel32 with nothing in the traceback pointing at either. The emitter
; computes this padding from the number of pushes and cannot get it wrong. That
; is a real point in favour of generated code and it is worth recording
; alongside the places where generated code loses.
;
; Build:
;   ml64 -nologo -c -Fosieve.obj sieve.asm
;   link -nologo -subsystem:console -entry:main sieve.obj kernel32.lib -out:sieve.exe

option casemap:none

extern VirtualAlloc: proc
extern GetStdHandle: proc
extern WriteFile: proc
extern ExitProcess: proc

N       equ 200000
ROUNDS  equ 100

.data
written qword 0
buf     db 32 dup(0)

.code

; rax = number of primes below N
countprimes proc
        push    rbx
        push    rsi
        push    rdi
        push    r12
        sub     rsp, 40

        xor     ecx, ecx
        mov     rdx, N
        mov     r8, 3000h               ; MEM_COMMIT | MEM_RESERVE
        mov     r9, 4                   ; PAGE_READWRITE
        call    VirtualAlloc
        mov     rbx, rax                ; c

        mov     rsi, 2                  ; i
outer:
        mov     rdi, rsi
        imul    rdi, rsi                ; i*i, ONCE
        cmp     rdi, N
        jge     counting
        cmp     byte ptr [rbx+rsi], 0
        jne     nexti
        mov     r12, rdi                ; j = i*i, reused
inner:
        cmp     r12, N
        jge     nexti
        mov     byte ptr [rbx+r12], 1
        add     r12, rsi
        jmp     inner
nexti:
        add     rsi, 1
        jmp     outer

counting:
        xor     esi, esi                ; acc
        mov     r12, 2                  ; k
count:
        cmp     r12, N
        jge     done
        cmp     byte ptr [rbx+r12], 0
        jne     skip
        add     rsi, 1
skip:
        add     r12, 1
        jmp     count
done:
        mov     rax, rsi
        add     rsp, 40
        pop     r12
        pop     rdi
        pop     rsi
        pop     rbx
        ret
countprimes endp

; print rcx as decimal followed by a newline
putint proc
        push    rbx
        push    rsi
        push    rdi
        sub     rsp, 48                 ; 3 pushes + 48 keeps rsp 16-aligned
        mov     rbx, rcx
        lea     rsi, buf
        mov     byte ptr [rsi+24], 10
        mov     rdi, 24
        test    rbx, rbx
        jnz     digits
        dec     rdi
        mov     byte ptr [rsi+rdi], '0'
        jmp     emit
digits:
        cmp     rbx, 0
        jle     emit
        mov     rax, rbx
        cqo
        mov     rcx, 10
        idiv    rcx                     ; ONE divide: rax quotient, rdx remainder
        add     rdx, 48
        dec     rdi
        mov     byte ptr [rsi+rdi], dl
        mov     rbx, rax
        jmp     digits
emit:
        mov     rcx, -11
        call    GetStdHandle
        mov     rcx, rax
        lea     rdx, buf
        add     rdx, rdi
        mov     r8, 25
        sub     r8, rdi
        lea     r9, written
        mov     qword ptr [rsp+32], 0
        call    WriteFile
        add     rsp, 48
        pop     rdi
        pop     rsi
        pop     rbx
        ret
putint endp

main proc
        push    rbx
        push    rsi
        sub     rsp, 40                 ; 2 pushes + 40 keeps rsp 16-aligned
        xor     ebx, ebx                ; total
        mov     rsi, ROUNDS
rounds:
        call    countprimes
        add     rbx, rax
        dec     rsi
        jnz     rounds
        mov     rcx, rbx
        call    putint
        xor     ecx, ecx
        call    ExitProcess
main endp

end
