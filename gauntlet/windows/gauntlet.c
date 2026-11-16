// THE GAUNTLET ON WINDOWS: hand-written C against the LLVM IR the windows target
// prints, both compiled by the same clang at the target's own -O2
// (gauntlet/windows/README.md, gauntlet-2026-11-16).
//
// The references are the Go ones (gauntlet/go) written as a person writes C:
// a pointer and a length, `int` indices, stack arrays where a size is fixed,
// malloc where Go allocates, and `restrict` where the stencil's buffers are
// known apart, which C can say and Go, JavaScript and Java cannot. The generated
// functions take the language's table, a pointer to an i64 length followed by
// the elements (docs/spec/llvm.md §4).
//
// One case per process, as on the other hosts:
//
//	bench DIR CASE    checks every case against Go's answers, then times CASE:
//	                  9 batches of at least 20 ms, the median ns/op, and the
//	                  process's peak working set
//	bench DIR check   checks only
//
// Every function timed is called through a volatile pointer and is not inlined
// into the timing loop, the generated ones being in other objects anyway, so a
// call costs the same on both sides and no work can be hoisted out of the loop.
#define _CRT_SECURE_NO_WARNINGS
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <windows.h>
#include <psapi.h>

#define NVEC 65536
#define NSMALL 1024
#define NSTENCIL 100000
#define NOINLINE __declspec(noinline)

// ------------------------------------------------------------------ tables
typedef struct { int64_t n; double d[]; } f64tab;
typedef struct { int64_t n; unsigned char b[]; } bytetab;

static f64tab *f64new(int64_t n) {
    f64tab *t = malloc(sizeof(f64tab) + (size_t)n * sizeof(double));
    t->n = n;
    return t;
}

// A generated table is released by Σ_OS's rule (targets/windows/os.oro): from
// 64 KiB VirtualFree, below it HeapFree on the process heap.
static void gen_release(void *p, int64_t bytes) {
    if (bytes >= 65536) VirtualFree(p, 0, MEM_RELEASE);
    else HeapFree(GetProcessHeap(), 0, p);
}

// ------------------------------------------------------------------ generated
double native_dot(const f64tab *a, const f64tab *b);
double native_centroid(const f64tab *xs, const f64tab *ys);
double native_sum_of(const f64tab *a);
f64tab *native_smooth(const f64tab *a);
f64tab *native_smooth_into(f64tab *dst, const f64tab *a);
int64_t native_find_first(const f64tab *a, double k);
int64_t tokenize_tokens(const bytetab *src);
int64_t tree_measure(const bytetab *src);

// ------------------------------------------------------------------ G1 dot
NOINLINE double dot_hand(const double *x, const double *y, int n) {
    double acc = 0;
    for (int i = 0; i < n; i++) acc += x[i] * y[i];
    return acc;
}

// ------------------------------------------------------------------ G2 centroid
NOINLINE double centroid_hand(const double *xs, const double *ys, int n) {
    double ax = 0, ay = 0;
    for (int i = 0; i < n; i++) {
        ax += xs[i];
        ay += ys[i];
    }
    return ax + ay;
}

// ------------------------------------------------------------------ G3 sum
NOINLINE double sum_hand(const double *xs, int n) {
    double acc = 0;
    for (int i = 0; i < n; i++) acc += xs[i];
    return acc;
}

// ------------------------------------------------------------------ G6 stencil
// The generated kernel writes j in [0, n-2) from a[j..j+2]; these write the same
// n - 2 values, so a fresh result has n - 2 elements, as SmoothAlloc's does.
NOINLINE double *smooth_alloc_hand(const double *src, int n) {
    double *dst = malloc((size_t)(n - 2) * sizeof(double));
    for (int i = 0; i < n - 2; i++) dst[i] = (src[i] + src[i + 1] + src[i + 2]) / 3;
    return dst;
}
NOINLINE void smooth_into_hand(double *dst, const double *src, int n) {
    for (int i = 0; i < n - 2; i++) dst[i] = (src[i] + src[i + 1] + src[i + 2]) / 3;
}
NOINLINE void smooth_into_restrict(double *restrict dst, const double *restrict src, int n) {
    for (int i = 0; i < n - 2; i++) dst[i] = (src[i] + src[i + 1] + src[i + 2]) / 3;
}

// ------------------------------------------------------------------ search
NOINLINE int64_t find_first_hand(const double *a, int n, double k) {
    for (int i = 0; i < n; i++)
        if (a[i] > k) return i;
    return -1;
}

// ------------------------------------------------------------------ G7 tokenizer
static int numeric(int c) { return (c >= 48 && c <= 57) || c == 45 || c == 43 || c == 46 || c == 101 || c == 69; }
static int alpha(int c) { return c >= 97 && c <= 122; }
static int scan_string(const unsigned char *src, int n, int i) {
    for (int j = i + 1;;) {
        if (j >= n) return j;
        if (src[j] == '\\') { j += 2; continue; }
        if (src[j] == '"') return j + 1;
        j++;
    }
}

#define TOKCAP 32
NOINLINE int64_t tokens_hand(const unsigned char *src, int n) {
    unsigned char stk[TOKCAP];
    int i = 0, nt = 0, sp = 0, mx = 0, ok = 1;
    for (;;) {
        if (i >= n) return (int64_t)nt * 1000 + mx * 10 + (sp != 0 ? 0 : ok);
        if (sp >= TOKCAP) return (int64_t)nt * 1000;
        int c = src[i];
        switch (c) {
        case ' ': case '\t': case '\n': case '\r':
            i++;
            break;
        case '{': case '[':
            stk[sp] = c == '{' ? '}' : ']';
            i++; nt++; sp++;
            if (sp > mx) mx = sp;
            break;
        case '}': case ']':
            i++; nt++;
            if (sp < 1) ok = 0;
            else if (stk[--sp] != c) ok = 0;
            break;
        case ':': case ',':
            i++; nt++;
            break;
        case '"':
            i = scan_string(src, n, i); nt++;
            break;
        default:
            if (numeric(c)) {
                int j = i;
                while (j < n && numeric(src[j])) j++;
                i = j; nt++;
            } else if (alpha(c)) {
                int j = i;
                while (j < n && alpha(src[j])) j++;
                i = j; nt++;
            } else {
                i++; ok = 0;
            }
        }
    }
}

// ------------------------------------------------------------------ JSON tree
// TreeFlat (gauntlet/go/jsontree.go) over bytes: a flat node table of 4 slots
// (tag, length, first child, next sibling) and a stack of 2 (node, last child).
// Two forms: the tables on the stack, what a person writes in C for a fixed
// bound, and from calloc, which is what the generated code and Go's do.
#define NMAX 512
#define DMAX 32

static void link_node(int *nodes, const int *stk, int sp, int k) {
    if (sp < 1) return;
    int lc = stk[2 * (sp - 1) + 1];
    if (lc == 0) nodes[4 * stk[2 * (sp - 1)] + 2] = k;
    else nodes[4 * lc + 3] = k;
}

static int64_t walk(const int *nodes, int *wl) {
    wl[0] = 1; wl[1] = 1;
    int sp = 1, seen = 0, acc = 0, steps = 0;
    while (sp >= 1 && steps < 2 * NMAX) {
        int nd = wl[2 * (sp - 1)], d = wl[2 * (sp - 1) + 1];
        int sb = nodes[4 * nd + 3], kd = nodes[4 * nd + 2];
        sp--;
        if (sb != 0) { wl[2 * sp] = sb; wl[2 * sp + 1] = d; sp++; }
        if (kd != 0) { wl[2 * sp] = kd; wl[2 * sp + 1] = d + 1; sp++; }
        seen++;
        acc += nodes[4 * nd] * d;
        steps++;
    }
    return (int64_t)seen * 1000 + acc;
}

static int64_t tree(const unsigned char *src, int n, int *nodes, int *stk, int *wl) {
    int i = 0, nn = 1, sp = 0;
    while (i < n && nn < NMAX && sp < DMAX) {
        int c = src[i];
        if (c == 32 || c == 9 || c == 10 || c == 13 || c == 58 || c == 44) {
            i++;
        } else if (c == 123 || c == 91) {
            nodes[4 * nn] = c == 91 ? 4 : 5;
            nodes[4 * nn + 1] = 0;
            link_node(nodes, stk, sp, nn);
            if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
            stk[2 * sp] = nn;
            stk[2 * sp + 1] = 0;
            i++; sp++; nn++;
        } else if (c == 125 || c == 93) {
            i++;
            if (sp >= 1) sp--;
        } else if (c == 34 || numeric(c) || alpha(c)) {
            int tg = 2, ni;
            if (c == 34) {
                ni = scan_string(src, n, i);
            } else if (numeric(c)) {
                tg = 1;
                ni = i;
                while (ni < n && numeric(src[ni])) ni++;
            } else {
                tg = 3;
                ni = i;
                while (ni < n && alpha(src[ni])) ni++;
            }
            nodes[4 * nn] = tg;
            nodes[4 * nn + 1] = ni - i;
            link_node(nodes, stk, sp, nn);
            if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
            i = ni; nn++;
        } else {
            i++;
        }
    }
    return walk(nodes, wl);
}

NOINLINE int64_t tree_hand_stack(const unsigned char *src, int n) {
    int nodes[4 * NMAX] = {0}, stk[2 * DMAX] = {0}, wl[2 * NMAX];
    return tree(src, n, nodes, stk, wl);
}
NOINLINE int64_t tree_hand_calloc(const unsigned char *src, int n) {
    int *nodes = calloc(4 * NMAX, sizeof(int)), *stk = calloc(2 * DMAX, sizeof(int));
    int *wl = calloc(2 * NMAX, sizeof(int));
    int64_t r = tree(src, n, nodes, stk, wl);
    free(nodes); free(stk); free(wl);
    return r;
}

// TokDoc (gauntlet/go/jsontok.go): the same document, built the same way.
static bytetab *tokdoc(int records) {
    static const char *rec = "{\"id\":1234,\"name\":\"a b\\\"c\",\"tags\":[\"x\",\"y\",\"z\"],"
                             "\"score\":-12.5e3,\"ok\":true,\"prev\":null,"
                             "\"meta\":{\"depth\":2,\"flag\":false}}";
    size_t rl = strlen(rec), cap = 16 + (size_t)records * (rl + 1);
    bytetab *t = malloc(sizeof(bytetab) + cap);
    size_t n = 0;
    memcpy(t->b, "{\"items\":[", 10); n = 10;
    for (int r = 0; r < records; r++) {
        if (r > 0) t->b[n++] = ',';
        memcpy(t->b + n, rec, rl); n += rl;
    }
    t->b[n++] = ']'; t->b[n++] = '}';
    t->n = (int64_t)n;
    return t;
}

// ------------------------------------------------------------------ inputs
static f64tab *small_a, *small_b, *vec_a, *vec_b, *stencil_src, *stencil_dst, *search_arr;
static bytetab *tok_doc, *tree_doc;
static volatile double sink_d;
static volatile int64_t sink_i;

static void load(const char *dir) {
    char path[1024];
    snprintf(path, sizeof path, "%s/vec.bin", dir);
    FILE *f = fopen(path, "rb");
    if (!f) { fprintf(stderr, "cannot open %s: go run ./wininputs DIR first\n", path); exit(2); }
    vec_a = f64new(NVEC); vec_b = f64new(NVEC);
    if (fread(vec_a->d, sizeof(double), NVEC, f) != NVEC || fread(vec_b->d, sizeof(double), NVEC, f) != NVEC) {
        fprintf(stderr, "%s is short\n", path); exit(2);
    }
    fclose(f);
    small_a = f64new(NSMALL); small_b = f64new(NSMALL);
    memcpy(small_a->d, vec_a->d, NSMALL * sizeof(double));
    memcpy(small_b->d, vec_b->d, NSMALL * sizeof(double));
    stencil_src = f64new(NSTENCIL); stencil_dst = f64new(NSTENCIL); search_arr = f64new(NSTENCIL);
    for (int i = 0; i < NSTENCIL; i++) {
        stencil_src->d[i] = (double)(i % 97);
        search_arr->d[i] = (double)i;
        stencil_dst->d[i] = 0;
    }
    tok_doc = tokdoc(64);
    tree_doc = tokdoc(20);
}

// ------------------------------------------------------------------ cases
static int64_t smooth_bytes(void) { return 8 + 8 * (int64_t)(NSTENCIL - 2); }

static void c_dot1024_hand(void) { sink_d = dot_hand(small_a->d, small_b->d, NSMALL); }
static void c_dot1024_gen(void) { sink_d = native_dot(small_a, small_b); }
static void c_dot65536_hand(void) { sink_d = dot_hand(vec_a->d, vec_b->d, NVEC); }
static void c_dot65536_gen(void) { sink_d = native_dot(vec_a, vec_b); }
static void c_centroid_hand(void) { sink_d = centroid_hand(vec_a->d, vec_b->d, NVEC); }
static void c_centroid_gen(void) { sink_d = native_centroid(vec_a, vec_b); }
static void c_sum_hand(void) { sink_d = sum_hand(vec_a->d, NVEC); }
static void c_sum_gen(void) { sink_d = native_sum_of(vec_a); }
static void c_alloc_hand(void) {
    double *r = smooth_alloc_hand(stencil_src->d, NSTENCIL);
    sink_d = r[0];
    free(r);
}
static void c_alloc_gen(void) {
    f64tab *r = native_smooth(stencil_src);
    sink_d = r->d[0];
    gen_release(r, smooth_bytes());
}
static void c_into_hand(void) { smooth_into_hand(stencil_dst->d, stencil_src->d, NSTENCIL); }
static void c_into_restrict(void) { smooth_into_restrict(stencil_dst->d, stencil_src->d, NSTENCIL); }
static void c_into_gen(void) { native_smooth_into(stencil_dst, stencil_src); }
static void c_early_hand(void) { sink_i = find_first_hand(search_arr->d, NSTENCIL, 5); }
static void c_early_gen(void) { sink_i = native_find_first(search_arr, 5); }
static void c_late_hand(void) { sink_i = find_first_hand(search_arr->d, NSTENCIL, 99998); }
static void c_late_gen(void) { sink_i = native_find_first(search_arr, 99998); }
static void c_tok_hand(void) { sink_i = tokens_hand(tok_doc->b, (int)tok_doc->n); }
static void c_tok_gen(void) { sink_i = tokenize_tokens(tok_doc); }
static void c_tree_stack(void) { sink_i = tree_hand_stack(tree_doc->b, (int)tree_doc->n); }
static void c_tree_calloc(void) { sink_i = tree_hand_calloc(tree_doc->b, (int)tree_doc->n); }
static void c_tree_gen(void) { sink_i = tree_measure(tree_doc); }

static const struct { const char *name; void (*run)(void); } cases[] = {
    {"g1-dot1024-hand", c_dot1024_hand}, {"g1-dot1024-gen", c_dot1024_gen},
    {"g1-dot65536-hand", c_dot65536_hand}, {"g1-dot65536-gen", c_dot65536_gen},
    {"g2-centroid-hand", c_centroid_hand}, {"g2-centroid-gen", c_centroid_gen},
    {"g3-sum-hand", c_sum_hand}, {"g3-sum-gen", c_sum_gen},
    {"g6-alloc-hand", c_alloc_hand}, {"g6-alloc-gen", c_alloc_gen},
    {"g6-into-hand", c_into_hand}, {"g6-into-restrict", c_into_restrict}, {"g6-into-gen", c_into_gen},
    {"s-early-hand", c_early_hand}, {"s-early-gen", c_early_gen},
    {"s-late-hand", c_late_hand}, {"s-late-gen", c_late_gen},
    {"g7-tok-hand", c_tok_hand}, {"g7-tok-gen", c_tok_gen},
    {"tree-stack-hand", c_tree_stack}, {"tree-calloc-hand", c_tree_calloc}, {"tree-gen", c_tree_gen},
};
#define NCASES (int)(sizeof cases / sizeof cases[0])

// ------------------------------------------------------------------ agreement
static char *answer(const char *dir, const char *name) {
    static char buf[4096];
    static int loaded = 0;
    if (!loaded) {
        char path[1024];
        snprintf(path, sizeof path, "%s/answers.txt", dir);
        FILE *f = fopen(path, "rb");
        if (!f) { fprintf(stderr, "cannot open %s\n", path); exit(2); }
        buf[fread(buf, 1, sizeof buf - 1, f)] = 0;
        fclose(f);
        loaded = 1;
    }
    size_t l = strlen(name);
    for (char *p = buf; *p; p = strchr(p, '\n') + 1) {
        if (strncmp(p, name, l) == 0 && p[l] == ' ') return p + l + 1;
        if (!strchr(p, '\n')) break;
    }
    fprintf(stderr, "no answer %s\n", name);
    exit(2);
}
static int fails = 0;
static void want_f64(const char *dir, const char *name, const char *who, double got) {
    uint64_t bits, want = strtoull(answer(dir, name), NULL, 16);
    memcpy(&bits, &got, 8);
    if (bits != want) { printf("DISAGREE %s %s: %016llx, Go %016llx\n", name, who, bits, want); fails++; }
}
static void want_i64(const char *dir, const char *name, const char *who, int64_t got, int field) {
    char *p = answer(dir, name);
    for (int k = 0; k < field; k++) p = strchr(p, ' ') + 1;
    long long want = strtoll(p, NULL, 10);
    if (got != want) { printf("DISAGREE %s %s: %lld, Go %lld\n", name, who, (long long)got, want); fails++; }
}
static double stencil_sum(const double *d) {
    double s = 0;
    for (int i = 0; i < NSTENCIL - 2; i++) s += d[i];
    return s;
}

static void check(const char *dir) {
    want_f64(dir, "dot1024", "hand", dot_hand(small_a->d, small_b->d, NSMALL));
    want_f64(dir, "dot1024", "gen", native_dot(small_a, small_b));
    want_f64(dir, "dot65536", "hand", dot_hand(vec_a->d, vec_b->d, NVEC));
    want_f64(dir, "dot65536", "gen", native_dot(vec_a, vec_b));
    want_f64(dir, "centroid", "hand", centroid_hand(vec_a->d, vec_b->d, NVEC));
    want_f64(dir, "centroid", "gen", native_centroid(vec_a, vec_b));
    want_f64(dir, "sum", "hand", sum_hand(vec_a->d, NVEC));
    want_f64(dir, "sum", "gen", native_sum_of(vec_a));
    double *h = smooth_alloc_hand(stencil_src->d, NSTENCIL);
    want_f64(dir, "stencil", "alloc hand", stencil_sum(h));
    free(h);
    f64tab *g = native_smooth(stencil_src);
    if (g->n != NSTENCIL - 2) { printf("DISAGREE stencil gen: length %lld\n", (long long)g->n); fails++; }
    want_f64(dir, "stencil", "alloc gen", stencil_sum(g->d));
    gen_release(g, smooth_bytes());
    smooth_into_hand(stencil_dst->d, stencil_src->d, NSTENCIL);
    want_f64(dir, "stencil", "into hand", stencil_sum(stencil_dst->d));
    memset(stencil_dst->d, 0, NSTENCIL * sizeof(double));
    smooth_into_restrict(stencil_dst->d, stencil_src->d, NSTENCIL);
    want_f64(dir, "stencil", "into restrict", stencil_sum(stencil_dst->d));
    memset(stencil_dst->d, 0, NSTENCIL * sizeof(double));
    native_smooth_into(stencil_dst, stencil_src);
    want_f64(dir, "stencil", "into gen", stencil_sum(stencil_dst->d));
    want_i64(dir, "search5", "hand", find_first_hand(search_arr->d, NSTENCIL, 5), 0);
    want_i64(dir, "search5", "gen", native_find_first(search_arr, 5), 0);
    want_i64(dir, "search99998", "hand", find_first_hand(search_arr->d, NSTENCIL, 99998), 0);
    want_i64(dir, "search99998", "gen", native_find_first(search_arr, 99998), 0);
    want_i64(dir, "tokens", "document length", tok_doc->n, 0);
    want_i64(dir, "tokens", "hand", tokens_hand(tok_doc->b, (int)tok_doc->n), 1);
    want_i64(dir, "tokens", "gen", tokenize_tokens(tok_doc), 1);
    want_i64(dir, "tree", "document length", tree_doc->n, 0);
    want_i64(dir, "tree", "stack hand", tree_hand_stack(tree_doc->b, (int)tree_doc->n), 1);
    want_i64(dir, "tree", "calloc hand", tree_hand_calloc(tree_doc->b, (int)tree_doc->n), 1);
    want_i64(dir, "tree", "gen", tree_measure(tree_doc), 1);
    // The malformed and edge documents the Go agreement tests use, generated
    // against hand-written: the tokenizer at four sizes and seven bad inputs.
    static const char *odd[] = {"]", "{\"a\":1", "[}", "{]", "\x01", "[[[[1]]]]", "", "[1,2]", "{\"a\":[1,2],\"b\":true}"};
    for (int k = 0; k < (int)(sizeof odd / sizeof odd[0]); k++) {
        size_t l = strlen(odd[k]);
        bytetab *t = malloc(sizeof(bytetab) + l + 1);
        t->n = (int64_t)l;
        memcpy(t->b, odd[k], l);
        int64_t a = tokens_hand(t->b, (int)l), b = tokenize_tokens(t);
        if (a != b) { printf("DISAGREE tokens %s: hand %lld gen %lld\n", odd[k], (long long)a, (long long)b); fails++; }
        a = tree_hand_stack(t->b, (int)l), b = tree_measure(t);
        if (a != b) { printf("DISAGREE tree %s: hand %lld gen %lld\n", odd[k], (long long)a, (long long)b); fails++; }
        free(t);
    }
    static const int recs_of[] = {0, 1, 3, 17};
    for (int k = 0; k < 4; k++) {
        int recs = recs_of[k];
        bytetab *t = tokdoc(recs);
        int64_t a = tokens_hand(t->b, (int)t->n), b = tokenize_tokens(t);
        if (a != b) { printf("DISAGREE tokens %d records: hand %lld gen %lld\n", recs, (long long)a, (long long)b); fails++; }
        free(t);
    }
}

// ------------------------------------------------------------------ timing
static double now_ns(void) {
    static LARGE_INTEGER freq;
    LARGE_INTEGER t;
    if (!freq.QuadPart) QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&t);
    return (double)t.QuadPart * 1e9 / (double)freq.QuadPart;
}
static int cmp(const void *a, const void *b) {
    double x = *(const double *)a, y = *(const double *)b;
    return x < y ? -1 : x > y;
}

int main(int argc, char **argv) {
    if (argc != 3) { fprintf(stderr, "usage: bench DIR CASE|check\n"); return 2; }
    load(argv[1]);
    check(argv[1]);
    if (fails) { printf("%d disagreement(s)\n", fails); return 1; }
    if (strcmp(argv[2], "check") == 0) { printf("all %d cases agree with Go\n", NCASES); return 0; }
    void (*volatile run)(void) = NULL;
    for (int k = 0; k < NCASES; k++)
        if (strcmp(cases[k].name, argv[2]) == 0) run = cases[k].run;
    if (!run) { fprintf(stderr, "no case %s\n", argv[2]); return 2; }
    long long iters = 1;
    for (;;) { // calibrate: a batch of at least 20 ms
        double t0 = now_ns();
        for (long long i = 0; i < iters; i++) run();
        if (now_ns() - t0 >= 20e6) break;
        iters *= 2;
    }
    double s[9];
    for (int b = 0; b < 9; b++) {
        double t0 = now_ns();
        for (long long i = 0; i < iters; i++) run();
        s[b] = (now_ns() - t0) / (double)iters;
    }
    qsort(s, 9, sizeof s[0], cmp);
    // The peak working set, the allocation criterion's measure here: a table
    // the program does not release grows it with every call.
    PROCESS_MEMORY_COUNTERS pm = {sizeof pm};
    K32GetProcessMemoryInfo(GetCurrentProcess(), &pm, sizeof pm);
    printf("%s %.2f ns/op %lld calls peak %zu KB\n", argv[2], s[4], iters * 10, pm.PeakWorkingSetSize / 1024);
    return 0;
}
