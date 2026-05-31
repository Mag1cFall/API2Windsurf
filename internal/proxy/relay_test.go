package proxy

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"api2windsurf/internal/protocol"
)

func decodeToolFrames(raw []byte) []struct{ ID, Name, Args string } {
	var out []struct{ ID, Name, Args string }
	pos := 0
	for pos+5 <= len(raw) {
		flag := raw[pos]
		ln := int(binary.BigEndian.Uint32(raw[pos+1 : pos+5]))
		pos += 5
		if pos+ln > len(raw) {
			break
		}
		payload := raw[pos : pos+ln]
		pos += ln
		if flag&0x02 != 0 {
			continue
		}
		for _, f := range protocol.ScanForTest(payload) {
			if f.Num != 6 || f.Wire != 2 {
				continue
			}
			var tc struct{ ID, Name, Args string }
			for _, sf := range protocol.ScanForTest(f.Data) {
				if sf.Wire != 2 {
					continue
				}
				switch sf.Num {
				case 1:
					tc.ID = string(sf.Data)
				case 2:
					tc.Name = string(sf.Data)
				case 3:
					tc.Args = string(sf.Data)
				}
			}
			out = append(out, tc)
		}
	}
	return out
}

func TestToolAccumulatorEmitsCompleteArgs(t *testing.T) {
	cases := []struct {
		name   string
		deltas []toolDelta
		want   map[string]string
	}{
		{
			name:   "cumulative-single-chunk",
			deltas: []toolDelta{{Index: 0, ID: "c1", Name: "get_weather", Args: `{"city":"Tokyo"}`}},
			want:   map[string]string{"get_weather": `{"city":"Tokyo"}`},
		},
		{
			name: "fragmented-args",
			deltas: []toolDelta{
				{Index: 0, ID: "c2", Name: "search"},
				{Index: 0, Args: `{"que`},
				{Index: 0, Args: `ry":"a`},
				{Index: 0, Args: `pex"}`},
			},
			want: map[string]string{"search": `{"query":"apex"}`},
		},
		{
			name: "parallel-two-tools",
			deltas: []toolDelta{
				{Index: 0, ID: "a", Name: "tool_a"},
				{Index: 1, ID: "b", Name: "tool_b"},
				{Index: 0, Args: `{"x":1}`},
				{Index: 1, Args: `{"y":2}`},
			},
			want: map[string]string{"tool_a": `{"x":1}`, "tool_b": `{"y":2}`},
		},
		{
			name:   "no-args-tool",
			deltas: []toolDelta{{Index: 0, ID: "c", Name: "get_time"}},
			want:   map[string]string{"get_time": "{}"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acc := newToolAccumulator()
			for _, d := range tc.deltas {
				acc.add(d)
			}
			var seq uint64
			var frames []byte
			acc.flush("bot-x", &seq, func(b []byte) { frames = append(frames, b...) })

			got := decodeToolFrames(frames)
			if len(got) != len(tc.want) {
				t.Fatalf("expected %d tool frames, got %d: %+v", len(tc.want), len(got), got)
			}
			for _, g := range got {
				want, ok := tc.want[g.Name]
				if !ok {
					t.Fatalf("unexpected tool %q", g.Name)
				}
				var probe any
				if err := json.Unmarshal([]byte(g.Args), &probe); err != nil {
					t.Fatalf("tool %q args not valid JSON: %q (%v)", g.Name, g.Args, err)
				}
				if g.Args != want {
					t.Fatalf("tool %q args mismatch: want %q got %q", g.Name, want, g.Args)
				}
			}
		})
	}
}
