package protocol

type TestField struct {
	Num  uint64
	Wire uint64
	Uint uint64
	Data []byte
}

func ScanForTest(buf []byte) []TestField {
	fields := scan(buf)
	out := make([]TestField, len(fields))
	for i, f := range fields {
		out[i] = TestField(f)
	}
	return out
}
