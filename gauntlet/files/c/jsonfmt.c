// jsonfmt: a JSON pretty-printer, two-space indent (examples/io/jsonfmt.oro):
// one streaming pass, the layout the program's `render` writes. `fresh` is
// whether the last thing written opened a container, so the next element
// starts a line; a closer after an element starts one too.
#include "files.h"

static int numeric(int c) { return (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E'; }
static int alpha(int c) { return c >= 'a' && c <= 'z'; }

static char *out;
static size_t op;

static void nl(int d) {
    out[op++] = '\n';
    for (int k = 0; k < 2 * d; k++) out[op++] = ' ';
}

int main(int argc, char **argv) {
    binary_out();
    if (argc < 2) { puts("usage: jsonfmt FILE"); return 0; }
    size_t n;
    unsigned char *s = read_all(argv[1], &n);
    if (!s) { puts("jsonfmt: cannot read that file"); return 0; }
    if (n >= 1u << 16) { puts("jsonfmt: file is larger than this tool accepts"); return 0; }
    out = malloc(n * 132 + 2); // each byte writes at most 1 + 2·64 + 2
    size_t i = 0;
    int d = 0, fresh = 0;
    while (i < n && d < 64) {
        int c = s[i];
        if (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
            i++;
        } else if (c == '{' || c == '[') {
            if (fresh) nl(d);
            out[op++] = (char)c;
            i++; d++; fresh = 1;
        } else if (c == '}' || c == ']') {
            d = d < 1 ? 0 : d - 1;
            if (!fresh) nl(d);
            out[op++] = (char)c;
            i++; fresh = 0;
        } else if (c == ',') {
            out[op++] = ',';
            nl(d);
            i++; fresh = 0;
        } else if (c == ':') {
            out[op++] = ':';
            out[op++] = ' ';
            i++;
        } else if (c == '"' || numeric(c) || alpha(c)) {
            size_t j = i + 1;
            if (c == '"') {
                while (j < n) {
                    if (s[j] == '\\') { if (j + 1 >= n) { j++; break; } j += 2; continue; }
                    if (s[j] == '"') { j++; break; }
                    j++;
                }
            } else if (numeric(c)) {
                while (j < n && numeric(s[j])) j++;
            } else {
                while (j < n && alpha(s[j])) j++;
            }
            if (j > n) j = n;
            if (fresh) nl(d);
            memcpy(out + op, s + i, j - i);
            op += j - i;
            i = j; fresh = 0;
        } else {
            i++;
        }
    }
    out[op++] = '\n';
    fwrite(out, 1, op, stdout);
    return 0;
}
