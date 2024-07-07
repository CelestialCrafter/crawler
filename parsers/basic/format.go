package basic

import (
	"bytes"
	"log"
	"regexp"
)

func removeExtraWhitespace(b []byte) []byte {
	out := b
	repeatedNewline, err := regexp.Compile("\n{2,}")
	if err != nil {
		log.Fatal("coult not compile regexp", "error", err)
	}

	out = bytes.ReplaceAll(out, []byte("\r"), []byte("\n"))
	out = repeatedNewline.ReplaceAllLiteral(out, []byte("\n"))
	return out
}
