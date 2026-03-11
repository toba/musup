package scan

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

// encodeUTF16LE encodes a Go string as a null-terminated UTF-16LE byte slice.
func encodeUTF16LE(s string) []byte {
	runes := utf16.Encode([]rune(s))
	runes = append(runes, 0) // null terminator
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return buf
}

// buildASF constructs a minimal ASF byte stream with the given metadata.
//
//nolint:gosec // test helper: integer conversions are safe for small test values
func buildASF(title, author, albumArtist, albumTitle string, trackNum int) []byte {
	var buf bytes.Buffer

	// Build child objects first to compute sizes.
	var children bytes.Buffer
	childCount := uint32(0)

	// Content Description Object (title + author).
	if title != "" || author != "" {
		childCount++
		titleBytes := encodeUTF16LE(title)
		authorBytes := encodeUTF16LE(author)
		var copyrightBytes, descBytes, ratingBytes []byte

		var obj bytes.Buffer
		lengths := [5]uint16{
			uint16(len(titleBytes)),
			uint16(len(authorBytes)),
			uint16(len(copyrightBytes)),
			uint16(len(descBytes)),
			uint16(len(ratingBytes)),
		}
		_ = binary.Write(&obj, binary.LittleEndian, lengths)
		obj.Write(titleBytes)
		obj.Write(authorBytes)

		objSize := uint64(24 + obj.Len())
		_ = binary.Write(&children, binary.LittleEndian, guidContentDesc)
		_ = binary.Write(&children, binary.LittleEndian, objSize)
		children.Write(obj.Bytes())
	}

	// Extended Content Description Object.
	if albumArtist != "" || albumTitle != "" || trackNum > 0 {
		childCount++
		var obj bytes.Buffer

		descriptors := 0
		var descs bytes.Buffer
		writeDesc := func(name string, val []byte) {
			descriptors++
			nameBytes := encodeUTF16LE(name)
			_ = binary.Write(&descs, binary.LittleEndian, uint16(len(nameBytes)))
			descs.Write(nameBytes)
			_ = binary.Write(&descs, binary.LittleEndian, uint16(0)) // type: string
			_ = binary.Write(&descs, binary.LittleEndian, uint16(len(val)))
			descs.Write(val)
		}
		writeDescDWORD := func(name string, val uint32) {
			descriptors++
			nameBytes := encodeUTF16LE(name)
			_ = binary.Write(&descs, binary.LittleEndian, uint16(len(nameBytes)))
			descs.Write(nameBytes)
			_ = binary.Write(&descs, binary.LittleEndian, uint16(3)) // type: DWORD
			_ = binary.Write(&descs, binary.LittleEndian, uint16(4))
			_ = binary.Write(&descs, binary.LittleEndian, val)
		}

		if albumArtist != "" {
			writeDesc("WM/AlbumArtist", encodeUTF16LE(albumArtist))
		}
		if albumTitle != "" {
			writeDesc("WM/AlbumTitle", encodeUTF16LE(albumTitle))
		}
		if trackNum > 0 {
			writeDescDWORD("WM/TrackNumber", uint32(trackNum))
		}

		_ = binary.Write(&obj, binary.LittleEndian, uint16(descriptors))
		obj.Write(descs.Bytes())

		objSize := uint64(24 + obj.Len())
		_ = binary.Write(&children, binary.LittleEndian, guidExtContentDesc)
		_ = binary.Write(&children, binary.LittleEndian, objSize)
		children.Write(obj.Bytes())
	}

	// Header Object: GUID + size(8) + childCount(4) + reserved(2) + children
	hdrSize := 16 + 8 + 4 + 2 + uint64(children.Len())
	_ = binary.Write(&buf, binary.LittleEndian, guidHeaderObject)
	_ = binary.Write(&buf, binary.LittleEndian, hdrSize)
	_ = binary.Write(&buf, binary.LittleEndian, childCount)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	buf.Write(children.Bytes())

	return buf.Bytes()
}

func TestParseASF_FullMetadata(t *testing.T) {
	data := buildASF("My Song", "The Author", "The Album Artist", "Great Album", 7)
	r := bytes.NewReader(data)

	artist, album, title, trackNum := parseASF(r)

	if artist != "The Album Artist" {
		t.Errorf("artist = %q, want %q", artist, "The Album Artist")
	}
	if album != "Great Album" {
		t.Errorf("album = %q, want %q", album, "Great Album")
	}
	if title != "My Song" {
		t.Errorf("title = %q, want %q", title, "My Song")
	}
	if trackNum != 7 {
		t.Errorf("trackNumber = %d, want %d", trackNum, 7)
	}
}

func TestParseASF_ContentDescOnly(t *testing.T) {
	data := buildASF("Track Title", "Artist Name", "", "", 0)
	r := bytes.NewReader(data)

	artist, album, title, trackNum := parseASF(r)

	if artist != "Artist Name" {
		t.Errorf("artist = %q, want %q", artist, "Artist Name")
	}
	if album != "" {
		t.Errorf("album = %q, want empty", album)
	}
	if title != "Track Title" {
		t.Errorf("title = %q, want %q", title, "Track Title")
	}
	if trackNum != 0 {
		t.Errorf("trackNumber = %d, want 0", trackNum)
	}
}

func TestParseASF_AlbumArtistOverridesAuthor(t *testing.T) {
	data := buildASF("Song", "Author", "Album Artist", "Album", 1)
	r := bytes.NewReader(data)

	artist, _, _, _ := parseASF(r)

	if artist != "Album Artist" {
		t.Errorf("artist = %q, want %q (albumArtist should override author)", artist, "Album Artist")
	}
}

func TestParseASF_EmptyFile(t *testing.T) {
	r := bytes.NewReader(nil)
	artist, album, title, trackNum := parseASF(r)

	if artist != "" || album != "" || title != "" || trackNum != 0 {
		t.Errorf("expected zero values for empty input, got (%q, %q, %q, %d)", artist, album, title, trackNum)
	}
}

func TestParseASF_BadGUID(t *testing.T) {
	data := make([]byte, 30)
	r := bytes.NewReader(data)
	artist, album, title, trackNum := parseASF(r)

	if artist != "" || album != "" || title != "" || trackNum != 0 {
		t.Errorf("expected zero values for bad GUID, got (%q, %q, %q, %d)", artist, album, title, trackNum)
	}
}

func TestReadASF_Fixture(t *testing.T) {
	path := filepath.Join("testdata", "fixture.wma")
	artist, album, title, trackNum := readASF(path)

	if artist != "3 Doors Down" {
		t.Errorf("artist = %q, want %q", artist, "3 Doors Down")
	}
	if album != "The Better Life" {
		t.Errorf("album = %q, want %q", album, "The Better Life")
	}
	if title != "Smack" {
		t.Errorf("title = %q, want %q", title, "Smack")
	}
	if trackNum != 10 {
		t.Errorf("trackNumber = %d, want %d", trackNum, 10)
	}
}

func TestDecodeUTF16LE(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"empty", nil, ""},
		{"single byte", []byte{0x41}, ""},
		{"ascii", encodeUTF16LE("hello"), "hello"},
		{"unicode", encodeUTF16LE("café"), "café"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeUTF16LE(tt.in)
			if got != tt.want {
				t.Errorf("decodeUTF16LE() = %q, want %q", got, tt.want)
			}
		})
	}
}
