package depthlife

import (
	"errors"
	"testing"
)

func TestDecodeAttachmentBadBlobs(t *testing.T) {
	cases := map[string][]byte{
		"short":     {1, 2, 3},
		"bad-magic": make([]byte, attachmentHeaderSize),
		"truncated": EncodeAttachment(1, 2, 2, []uint16{1, 2, 3})[:14],
	}
	for name, raw := range cases {
		if _, err := DecodeAttachment(raw); !errors.Is(err, ErrBadBlob) {
			t.Fatalf("%s: err=%v, want ErrBadBlob", name, err)
		}
	}
}

func TestDecodeTemplateBadBlobs(t *testing.T) {
	if _, err := DecodeTemplate(nil); !errors.Is(err, ErrBadBlob) {
		t.Fatalf("nil blob err=%v", err)
	}
	good := makeAttachment(t, 1, 1, 1, []uint16{7})
	att, _ := DecodeAttachment(good)
	tpl := &Template{
		Machine: Host, Eye: Left, AttID: att.ID, Version: 1,
		Width: 1, Height: 1, Z: []uint16{7},
		Threshold: 1, Coverage: 1, Passed: true, Mask: []bool{true},
	}
	blob, err := tpl.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got := tpl.TaggedEye(); got != Left || tpl.TaggedAttachment() != att.ID {
		t.Fatalf("template tags wrong: eye=%s att=%s", got, tpl.TaggedAttachment())
	}
	broken := append([]byte(nil), blob...)
	broken[4] = 9
	if _, err := DecodeTemplate(broken); !errors.Is(err, ErrBadBlob) {
		t.Fatalf("bad machine tag err=%v, want ErrBadBlob", err)
	}
	short := append([]byte(nil), blob...)
	short = short[:len(short)-2]
	if _, err := DecodeTemplate(short); !errors.Is(err, ErrBadBlob) {
		t.Fatalf("truncated blob err=%v, want ErrBadBlob", err)
	}
}
