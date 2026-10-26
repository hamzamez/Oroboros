/* The driver every path links against (backend-2026-10-26): it calls the
   generated gen_bench, times the call in-process with the performance counter,
   and prints the result and the milliseconds. Only gen_bench differs between
   the builds, so only the generated code is compared. */
#include <windows.h>
#include <stdio.h>

extern long long gen_bench(void);

int main(void) {
    LARGE_INTEGER f, a, b;
    QueryPerformanceFrequency(&f);
    QueryPerformanceCounter(&a);
    long long r = gen_bench();
    QueryPerformanceCounter(&b);
    printf("%lld %.2f\n", r, (double)(b.QuadPart - a.QuadPart) * 1000.0 / (double)f.QuadPart);
    return 0;
}
