// roundtrip: put then get on the store, the variable, the length, a read at
// an offset, and a missing file's kind (examples/io/roundtrip.oro).
#include "files.h"

int main(int argc, char **argv) {
    binary_out();
    if (argc < 2) { puts("usage: roundtrip FILE"); return 1; }
    static const unsigned char data[] = {104, 105, 32, 226, 130, 172, 10};
    FILE *f = fopen(argv[1], "wb");
    if (!f || fwrite(data, 1, sizeof data, f) != sizeof data) { puts("roundtrip: cannot write that file: other"); return 1; }
    fclose(f);
    size_t n;
    unsigned char *b = read_all(argv[1], &n);
    if (!b) { puts("roundtrip: cannot read that file: other"); return 1; }
    fwrite(b, 1, n, stdout);
    const char *v = getenv("ORO_ROUNDTRIP");
    printf("%s\n%zu\n", v ? v : "", n);
    f = fopen(argv[1], "rb");
    unsigned char buf[4];
    fseek(f, 3, SEEK_SET);
    size_t k = fread(buf, 1, 4, f);
    fclose(f);
    printf("%zu %03d\n", k % 10, buf[0]);
    char missing[1024];
    snprintf(missing, sizeof missing, "%s.missing", argv[1]);
    f = fopen(missing, "rb");
    puts(f ? "found" : "not-found");
    return 0;
}
