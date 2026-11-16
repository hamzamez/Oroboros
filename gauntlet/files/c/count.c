// count: lines, words and bytes, as POSIX wc defines them (examples/io/count.oro).
#include "files.h"

int main(int argc, char **argv) {
    binary_out();
    if (argc < 2) { puts("usage: count FILE"); return 0; }
    size_t n;
    unsigned char *b = read_all(argv[1], &n);
    if (!b) { puts("count: cannot read that file"); return 0; }
    if (n >= 1u << 24) { puts("count: file is larger than this tool accepts"); return 0; }
    long lines = 0, words = 0;
    int in_word = 0;
    for (size_t i = 0; i < n; i++) {
        unsigned char c = b[i];
        lines += c == '\n';
        if (c == ' ' || (c >= 9 && c <= 13)) in_word = 0;
        else if (!in_word) { in_word = 1; words++; }
    }
    printf("%ld\n%ld\n%zu\n", lines, words, n);
    return 0;
}
