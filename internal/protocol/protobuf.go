package protocol

import "encoding/binary"

const (
	wireVarint = 0
	wireBytes  = 2
	wireI64    = 1
	wireI32    = 5
)

func putUvarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

func readUvarint(buf []byte, off int) (v uint64, next int, ok bool) {
	var shift uint
	for off < len(buf) {
		b := buf[off]
		off++
		v |= uint64(b&0x7f) << shift
		if b < 0x80 {
			return v, off, true
		}
		shift += 7
		if shift >= 64 {
			return 0, off, false
		}
	}
	return 0, off, false
}

func tag(field, wire uint64) uint64 { return field<<3 | wire }

func appendVarintField(dst []byte, field, value uint64) []byte {
	dst = putUvarint(dst, tag(field, wireVarint))
	return putUvarint(dst, value)
}

func appendBytesField(dst []byte, field uint64, data []byte) []byte {
	dst = putUvarint(dst, tag(field, wireBytes))
	dst = putUvarint(dst, uint64(len(data)))
	return append(dst, data...)
}

func appendStringField(dst []byte, field uint64, s string) []byte {
	return appendBytesField(dst, field, []byte(s))
}

type field struct {
	Num  uint64
	Wire uint64
	Uint uint64
	Data []byte
}

func scan(buf []byte) []field {
	var out []field
	off := 0
	for off < len(buf) {
		t, next, ok := readUvarint(buf, off)
		if !ok || next == off {
			break
		}
		off = next
		f := field{Num: t >> 3, Wire: t & 7}
		switch f.Wire {
		case wireVarint:
			v, n, ok := readUvarint(buf, off)
			if !ok {
				return out
			}
			f.Uint, off = v, n
		case wireBytes:
			length, n, ok := readUvarint(buf, off)
			if !ok {
				return out
			}
			off = n
			end := off + int(length)
			if end < off || end > len(buf) {
				return out
			}
			f.Data = buf[off:end]
			off = end
		case wireI64:
			if off+8 > len(buf) {
				return out
			}
			f.Data = buf[off : off+8]
			off += 8
		case wireI32:
			if off+4 > len(buf) {
				return out
			}
			f.Data = buf[off : off+4]
			off += 4
		default:
			return out
		}
		out = append(out, f)
	}
	return out
}

const (
	flagCompressed = 0x01
	flagEndStream  = 0x02
)

const envelopeHeaderLen = 5

func frame(payload []byte, flag byte) []byte {
	out := make([]byte, envelopeHeaderLen+len(payload))
	out[0] = flag
	binary.BigEndian.PutUint32(out[1:5], uint32(len(payload)))
	copy(out[envelopeHeaderLen:], payload)
	return out
}
