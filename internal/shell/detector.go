package shell

import "regexp"

// Detector procura marcadores de som num fluxo de saída de terminal. Ele é
// alimentado com os bytes na ordem em que chegam, em pedaços de qualquer
// tamanho: um marcador dividido entre dois pedaços é detectado uma vez só.
//
// Marcadores aceitos:
//
//   - texto visível: [play <evento>], por exemplo "[play success]". Códigos
//     ANSI (cores) no meio do texto são ignorados;
//   - sequência OSC invisível: ESC ] 777 ; fuoda ; <evento> BEL (ou ESC \),
//     por exemplo: printf '\e]777;fuoda;success\a'.
//
// Nomes de evento: letras, dígitos, "_", "-" e ".", até 32 caracteres.
type Detector struct {
	OnEvent func(event string)

	state int
	text  []byte // texto visível recente, sem sequências de escape
	osc   []byte // conteúdo da OSC em andamento
}

const (
	stNormal = iota
	stEsc    // depois de ESC
	stCSI    // dentro de ESC [ ... (cores, cursor)
	stOSC    // dentro de ESC ] ...
	stOSCEsc // ESC dentro de uma OSC (possível ESC \)
)

const (
	maxText   = 128
	keepText  = 64
	maxOSC    = 256
	oscPrefix = "777;fuoda;"
)

var (
	textMarker = regexp.MustCompile(`\[play ([A-Za-z0-9_.-]{1,32})\]$`)
	eventName  = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,32}$`)
)

// Write alimenta o detector. Nunca falha; implementa io.Writer.
func (d *Detector) Write(p []byte) (int, error) {
	for _, b := range p {
		d.feed(b)
	}
	return len(p), nil
}

func (d *Detector) feed(b byte) {
	switch d.state {
	case stNormal:
		if b == 0x1b {
			d.state = stEsc
			return
		}
		d.text = append(d.text, b)
		if b == ']' {
			if m := textMarker.FindSubmatch(d.text); m != nil {
				d.emit(string(m[1]))
				d.text = d.text[:0]
			}
		}
		if len(d.text) > maxText {
			d.text = append(d.text[:0], d.text[len(d.text)-keepText:]...)
		}
	case stEsc:
		switch b {
		case '[':
			d.state = stCSI
		case ']':
			d.state = stOSC
			d.osc = d.osc[:0]
		default:
			d.state = stNormal // escape de dois bytes
		}
	case stCSI:
		if b >= 0x40 && b <= 0x7e {
			d.state = stNormal
		}
	case stOSC:
		switch b {
		case 0x07:
			d.finishOSC()
		case 0x1b:
			d.state = stOSCEsc
		default:
			if len(d.osc) < maxOSC {
				d.osc = append(d.osc, b)
			}
		}
	case stOSCEsc:
		if b == '\\' {
			d.finishOSC()
			return
		}
		// OSC interrompida por outra sequência de escape.
		d.osc = d.osc[:0]
		d.state = stEsc
		d.feed(b)
	}
}

func (d *Detector) finishOSC() {
	d.state = stNormal
	s := string(d.osc)
	d.osc = d.osc[:0]
	if len(s) > len(oscPrefix) && s[:len(oscPrefix)] == oscPrefix {
		if ev := s[len(oscPrefix):]; eventName.MatchString(ev) {
			d.emit(ev)
		}
	}
}

func (d *Detector) emit(event string) {
	if d.OnEvent != nil {
		d.OnEvent(event)
	}
}
