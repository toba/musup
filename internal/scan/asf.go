package scan

import (
	"encoding/binary"
	"io"
	"os"
	"strings"
	"unicode/utf16"
)

// ASF/WMV GUIDs (binary representation, little-endian mixed-endian format).
var (
	guidHeaderObject   = [16]byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}
	guidContentDesc    = [16]byte{0x33, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}
	guidExtContentDesc = [16]byte{0x40, 0xA4, 0xD0, 0xD2, 0x07, 0xE3, 0xD2, 0x11, 0x97, 0xF0, 0x00, 0xA0, 0xC9, 0x5E, 0xA8, 0x50}
)

// readASF extracts metadata from an ASF/WMA file.
func readASF(path string) (artist, album, title string, trackNumber int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	return parseASF(f)
}

func parseASF(r io.ReadSeeker) (artist, album, title string, trackNumber int) {
	// Read Header Object: 16-byte GUID + 8-byte size + 4-byte child count + 2 reserved
	var hdrGUID [16]byte
	if err := binary.Read(r, binary.LittleEndian, &hdrGUID); err != nil {
		return
	}
	if hdrGUID != guidHeaderObject {
		return
	}

	var hdrSize uint64
	if err := binary.Read(r, binary.LittleEndian, &hdrSize); err != nil {
		return
	}

	var childCount uint32
	if err := binary.Read(r, binary.LittleEndian, &childCount); err != nil {
		return
	}

	// Skip 2 reserved bytes.
	if _, err := r.Seek(2, io.SeekCurrent); err != nil {
		return
	}

	var albumArtist string

	for i := uint32(0); i < childCount; i++ {
		var objGUID [16]byte
		if err := binary.Read(r, binary.LittleEndian, &objGUID); err != nil {
			return
		}
		var objSize uint64
		if err := binary.Read(r, binary.LittleEndian, &objSize); err != nil {
			return
		}
		if objSize < 24 {
			return
		}
		dataSize := int64(objSize) - 24 // already read GUID + size

		switch objGUID {
		case guidContentDesc:
			var a, t string
			a, t = parseContentDesc(r, dataSize)
			if artist == "" {
				artist = a
			}
			if title == "" {
				title = t
			}
		case guidExtContentDesc:
			var ea, et, eaa string
			var etn int
			eaa, ea, et, etn = parseExtContentDesc(r, dataSize)
			if albumArtist == "" {
				albumArtist = eaa
			}
			if album == "" {
				album = ea
			}
			if title == "" {
				title = et
			}
			if trackNumber == 0 {
				trackNumber = etn
			}
		default:
			if _, err := r.Seek(dataSize, io.SeekCurrent); err != nil {
				return
			}
		}
	}

	if albumArtist != "" {
		artist = albumArtist
	}

	return
}

// parseContentDesc reads the Content Description Object fields.
// Fields: TitleLen, AuthorLen, CopyrightLen, DescLen, RatingLen (each uint16),
// then the string data in that order.
func parseContentDesc(r io.ReadSeeker, _ int64) (author, title string) {
	var lengths [5]uint16
	if err := binary.Read(r, binary.LittleEndian, &lengths); err != nil {
		return
	}

	titleBytes := make([]byte, lengths[0])
	if _, err := io.ReadFull(r, titleBytes); err != nil {
		return
	}
	title = decodeUTF16LE(titleBytes)

	authorBytes := make([]byte, lengths[1])
	if _, err := io.ReadFull(r, authorBytes); err != nil {
		return
	}
	author = decodeUTF16LE(authorBytes)

	// Skip copyright + description + rating.
	skip := int64(lengths[2]) + int64(lengths[3]) + int64(lengths[4])
	if skip > 0 {
		if _, err := r.Seek(skip, io.SeekCurrent); err != nil {
			return
		}
	}

	return
}

// parseExtContentDesc reads the Extended Content Description Object.
// Returns albumArtist, album, title, trackNumber.
func parseExtContentDesc(r io.ReadSeeker, _ int64) (albumArtist, album, title string, trackNumber int) {
	var count uint16
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return
	}

	for i := uint16(0); i < count; i++ {
		var nameLen uint16
		if err := binary.Read(r, binary.LittleEndian, &nameLen); err != nil {
			return
		}
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(r, nameBytes); err != nil {
			return
		}
		name := decodeUTF16LE(nameBytes)

		var valType uint16
		if err := binary.Read(r, binary.LittleEndian, &valType); err != nil {
			return
		}
		var valLen uint16
		if err := binary.Read(r, binary.LittleEndian, &valLen); err != nil {
			return
		}
		valBytes := make([]byte, valLen)
		if _, err := io.ReadFull(r, valBytes); err != nil {
			return
		}

		nameUpper := strings.ToUpper(name)
		switch nameUpper {
		case "WM/ALBUMTITLE":
			album = decodeUTF16LE(valBytes)
		case "WM/ALBUMARTIST":
			albumArtist = decodeUTF16LE(valBytes)
		case "WM/TRACKNUMBER":
			if valType == 0 { // string
				trackNumber = parseTrackString(decodeUTF16LE(valBytes))
			} else if valType == 3 && len(valBytes) >= 4 { // DWORD
				trackNumber = int(binary.LittleEndian.Uint32(valBytes))
			}
		case "WM/TRACK":
			if trackNumber == 0 {
				if valType == 3 && len(valBytes) >= 4 {
					trackNumber = int(binary.LittleEndian.Uint32(valBytes)) + 1 // WM/Track is 0-based
				}
			}
		case "TITLE":
			title = decodeUTF16LE(valBytes)
		}
	}

	return
}

func parseTrackString(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// decodeUTF16LE decodes a UTF-16LE byte slice, stripping null terminators.
func decodeUTF16LE(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	// Convert bytes to uint16 slice.
	n := len(b) / 2
	u16 := make([]uint16, n)
	for i := range n {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	// Strip trailing null.
	for len(u16) > 0 && u16[len(u16)-1] == 0 {
		u16 = u16[:len(u16)-1]
	}
	return string(utf16.Decode(u16))
}
