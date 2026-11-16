// freq: the twenty most frequent words, case-folded (examples/io/freq.oro):
// `sort | uniq -c | sort -rn | head -20`. A word is a maximal run of ASCII
// letters; equal words fold A-Z to a-z; each distinct word is spelled as its
// first occurrence, and ties in count go alphabetically by the folded word,
// which is what the program's two stable merge sorts give.
#include "files.h"

static const unsigned char *src;
typedef struct { int beg, len; } word;
typedef struct { int first, count; } dist;
static word *ws;

static int fold(int c) { return c >= 'A' && c <= 'Z' ? c + 32 : c; }
static int letter(int c) { return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'); }

static int wcmp(const word *a, const word *b) {
    int n = a->len < b->len ? a->len : b->len;
    for (int k = 0; k < n; k++) {
        int x = fold(src[a->beg + k]), y = fold(src[b->beg + k]);
        if (x != y) return x < y ? -1 : 1;
    }
    return a->len < b->len ? -1 : a->len > b->len;
}

// by folded word, then position: the stable order
static int by_word(const void *a, const void *b) {
    int i = *(const int *)a, j = *(const int *)b;
    int c = wcmp(&ws[i], &ws[j]);
    return c ? c : (i > j) - (i < j);
}

static dist *ds;
// by count, descending, then the distinct words' own (alphabetical) order
static int by_count(const void *a, const void *b) {
    int i = *(const int *)a, j = *(const int *)b;
    if (ds[i].count != ds[j].count) return ds[i].count > ds[j].count ? -1 : 1;
    return (i > j) - (i < j);
}

int main(int argc, char **argv) {
    binary_out();
    if (argc < 2) { puts("usage: freq FILE"); return 0; }
    size_t n;
    src = read_all(argv[1], &n);
    if (!src) { puts("freq: cannot read that file"); return 0; }
    if (n >= 1u << 16) { puts("freq: file is larger than this tool accepts"); return 0; }
    ws = malloc((n / 2 + 1) * sizeof *ws);
    int nw = 0;
    for (size_t i = 0; i < n && nw < 32768;) {
        if (!letter(src[i])) { i++; continue; }
        size_t j = i + 1;
        while (j < n && letter(src[j])) j++;
        ws[nw].beg = (int)i;
        ws[nw].len = (int)(j - i);
        nw++;
        i = j;
    }
    int *ord = malloc((nw + 1) * sizeof *ord);
    for (int k = 0; k < nw; k++) ord[k] = k;
    qsort(ord, nw, sizeof *ord, by_word);
    ds = malloc((nw + 1) * sizeof *ds);
    int nd = 0;
    for (int k = 0; k < nw; k++) {
        if (k == 0 || wcmp(&ws[ord[k]], &ws[ord[k - 1]]) != 0) {
            ds[nd].first = ord[k];
            ds[nd].count = 1;
            nd++;
        } else {
            ds[nd - 1].count++;
        }
    }
    int *ord2 = malloc((nd + 1) * sizeof *ord2);
    for (int k = 0; k < nd; k++) ord2[k] = k;
    qsort(ord2, nd, sizeof *ord2, by_count);
    for (int r = 0; r < nd && r < 20; r++) {
        const dist *e = &ds[ord2[r]];
        printf("%d %.*s\n", e->count, ws[e->first].len, (const char *)src + ws[e->first].beg);
    }
    return 0;
}
