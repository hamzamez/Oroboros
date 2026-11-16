// The file programs' hand-written C (gauntlet/files/README.md): what a person
// writes, the C runtime's stdio, the whole file read at once as os.ReadFile
// does, the output written in one call.
#define _CRT_SECURE_NO_WARNINGS
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <io.h>

// stdout in binary mode: the programs write LF, and text mode would write CRLF
static void binary_out(void) { _setmode(_fileno(stdout), _O_BINARY); }

// read_all is the file's bytes and their count, or NULL.
static unsigned char *read_all(const char *path, size_t *n) {
    FILE *f = fopen(path, "rb");
    if (!f) return NULL;
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    unsigned char *b = malloc(sz > 0 ? (size_t)sz : 1);
    *n = fread(b, 1, (size_t)sz, f);
    fclose(f);
    return b;
}
