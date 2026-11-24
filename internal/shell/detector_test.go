package shell

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func detect(chunks ...string) []string {
	var got []string
	d := &Detector{OnEvent: func(ev string) { got = append(got, ev) }}
	for _, c := range chunks {
		_, _ = d.Write([]byte(c))
	}
	return got
}

func TestDetector(t *testing.T) {
	cases := []struct {
		name   string
		chunks []string
		want   []string
	}{
		{"texto simples", []string{"build ok [play success]\n"}, []string{"success"}},
		{"dividido entre pedaços", []string{"[pl", "ay succ", "ess]"}, []string{"success"}},
		{"um byte por vez", splitBytes("x[play error]y"), []string{"error"}},
		{"dois marcadores", []string{"[play success] e [play error]"}, []string{"success", "error"}},
		{"cor no meio", []string{"\x1b[32m[play \x1b[1msuccess\x1b[0m]"}, []string{"success"}},
		{"osc com BEL", []string{"\x1b]777;fuoda;notification\x07"}, []string{"notification"}},
		{"osc com ST dividida", []string{"\x1b]777;fu", "oda;warning\x1b", "\\"}, []string{"warning"}},
		{"osc de outro programa", []string{"\x1b]0;titulo da janela\x07"}, nil},
		{"osc 777 de notificação", []string{"\x1b]777;notify;oi;tchau\x07"}, nil},
		{"nome inválido", []string{"[play suc cess]", "[play ]", "[play a/b]"}, nil},
		{"nome longo demais", []string{"[play " + strings.Repeat("a", 40) + "]"}, nil},
		{"sem play", []string{"[success]", "play success"}, nil},
		{"colchete antes", []string{"[[play success]"}, []string{"success"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := detect(c.chunks...); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestDetectorLongOutputDoesNotGrow(t *testing.T) {
	d := &Detector{}
	big := make([]byte, 1<<20)
	for i := range big {
		big[i] = 'a'
	}
	_, _ = d.Write(big)
	if len(d.text) > maxText {
		t.Fatalf("buffer grew to %d", len(d.text))
	}
}

func TestThrottle(t *testing.T) {
	var got []string
	f := throttle(func(ev string) { got = append(got, ev) }, time.Hour)
	f("success")
	f("success")
	f("error")
	if !reflect.DeepEqual(got, []string{"success", "error"}) {
		t.Fatalf("got %q", got)
	}
}

func splitBytes(s string) []string {
	out := make([]string, len(s))
	for i := range s {
		out[i] = s[i : i+1]
	}
	return out
}
