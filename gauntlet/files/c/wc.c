// wc: the lines of a file, the count of its newlines (examples/io/wc.oro).
#include "files.h"

int main(int argc, char **argv) {
    binary_out();
    if (argc < 2) { puts("usage: wc FILE"); return 0; }
    size_t n;
    unsigned char *b = read_all(argv[1], &n);
    if (!b) { puts("wc: cannot read that file"); return 0; }
    if (n >= 1u << 24) { puts("wc: file is larger than this tool accepts"); return 0; }
    long lines = 0;
    for (size_t i = 0; i < n; i++) lines += b[i] == '\n';
    printf("%ld\n", lines);
    return 0;
}
