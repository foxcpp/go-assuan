package common

import "testing"

func TestEscapeParams(t *testing.T) {
	if escapeParameters("\r\n%") != "%0D%0A%25" {
		t.Error("\\r\\n% should be escaped to %0D%0A%25")
	}
	if escapeParameters("foobar\\") != "foobar%5C" {
		t.Error("foobar\\ should be escaped to foobar%5C")
	}
}

func TestUnescapeParams(t *testing.T) {
	res, err := unescapeParameters("%0D%0A%25%5C")
	if err != nil {
		t.Error("unescape %0D%0A%25%5C:", err)
		t.FailNow()
	}
	if res != "\r\n%\\" {
		t.Error("%0D%0A%25%5C should be de-escaped to \\r\\n%\\")
	}

	// https://github.com/foxcpp/go-assuan/pull/1
	res, err = unescapeParameters("+++")
	if err != nil {
		t.Error("unescape +++:", err)
		t.FailNow()
	}
	if res != "+++" {
		t.Error("common.unescapeParameters removes + from output")
	}
}
