#include <stdio.h>
#include <stdint.h>
#ifndef T
#define T uint16_t
#endif
static T a[65536], b[65536];
__declspec(noinline) static int64_t dot(const T *x, const T *y, int64_t n) {
    int64_t acc = 0;
    for (int64_t i = 0; i < n; i++) acc += (int64_t)x[i] * (int64_t)y[i];
    return acc;
}
int main(void) {
    for (int64_t k = 0; k < 65536; k++) { a[k] = (T)((k * 7919) % 65536); b[k] = (T)((k * 104729) % 65536); }
    int64_t s = 0;
    for (int64_t r = 0; r < 2000; r++) { __asm__ volatile("" ::: "memory"); } for (int64_t r = 0; r < 2000; r++) { __asm__ volatile("" ::: "memory"); s = (s + dot(a, b, 65536) + r) % 1000000007; }
    printf("%lld\n", (long long)s);
    return 0;
}
