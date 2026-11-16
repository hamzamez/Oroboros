package main

import (
	"os"
	"reflect"
	"testing"
)

// NO CARRIAGE RETURN IN A TRACKED TEXT FILE (endings.go). A listing in
// `git ls-files --eol`'s form, each file's bytes planted: the CRLF text file
// is refused, a lone CR too; an LF file, a .bat whose attribute is eol=crlf,
// the -text baseline and a binary are not read as text. THE PLANTED FAULT is
// the CRLF file itself, which must be found, or the step cannot fail.
func TestACarriageReturnInATextFileIsRefused(t *testing.T) {
	listing := "i/lf    w/lf    attr/text=auto eol=lf \tgood.go\n" +
		"i/lf    w/crlf  attr/text=auto eol=lf \tbad.txt\n" +
		"i/lf    w/mixed attr/text=auto eol=lf \tlone.md\n" +
		"i/lf    w/crlf  attr/text eol=crlf    \tbuild.bat\n" +
		"i/lf    w/lf    attr/-text            \tbaseline.ll\n" +
		"i/-text w/-text attr/text=auto eol=lf \timage.png\n" +
		"i/lf    w/      attr/text=auto eol=lf \tdeleted.go\n"
	files := map[string]string{
		"good.go":     "package p\n",
		"bad.txt":     "a\r\nb\r\n",
		"lone.md":     "a\rb\n",
		"build.bat":   "echo\r\n",
		"baseline.ll": "x\r\n",
		"image.png":   "\x89PNG\r\n",
	}
	read := func(p string) ([]byte, error) {
		s, ok := files[p]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(s), nil
	}
	got, err := crlfFiles(listing, read)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bad.txt", "lone.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	// With the planted files rewritten LF, nothing is refused.
	files["bad.txt"], files["lone.md"] = "a\nb\n", "ab\n"
	if got, _ := crlfFiles(listing, read); len(got) != 0 {
		t.Errorf("LF files refused: %v", got)
	}
}
