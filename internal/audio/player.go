package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gen2brain/malgo"
	mp3 "github.com/hajimehoshi/go-mp3"
)

func PlayFile(path string, volume int) error {
	if volume < 0 || volume > 100 {
		return errors.New("volume deve estar entre 0 e 100")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		return fmt.Errorf("não foi possível decodificar MP3: %w", err)
	}
	pcm, err := io.ReadAll(decoder)
	if err != nil {
		return fmt.Errorf("erro lendo áudio: %w", err)
	}
	if len(pcm) == 0 {
		return errors.New("arquivo MP3 vazio")
	}

	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return fmt.Errorf("inicializando contexto de áudio: %w", err)
	}
	defer ctx.Free()

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = 2
	deviceConfig.SampleRate = uint32(decoder.SampleRate())
	deviceConfig.PerformanceProfile = malgo.Conservative
	deviceConfig.PeriodSizeInMilliseconds = 50

	reader := &pcmReader{data: pcm, volume: float32(volume) / 100}
	device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: func(output, input []byte, frameCount uint32) {
			_ = input
			reader.read(output)
		},
	})
	if err != nil {
		return fmt.Errorf("abrindo dispositivo de áudio: %w", err)
	}
	defer device.Uninit()

	if err := device.Start(); err != nil {
		return fmt.Errorf("iniciando reprodução: %w", err)
	}
	bytesPerSecond := int(decoder.SampleRate()) * 2 * 2
	durationSeconds := (float64(len(pcm)) / float64(bytesPerSecond)) + 0.15
	if durationSeconds < 0.05 {
		durationSeconds = 0.05
	}
	<-reader.doneAfter(durationSeconds)
	return nil
}

type pcmReader struct {
	data   []byte
	offset int
	volume float32
}

func (r *pcmReader) read(dst []byte) {
	for i := 0; i+1 < len(dst); i += 2 {
		if r.offset+1 >= len(r.data) {
			dst[i], dst[i+1] = 0, 0
			continue
		}
		sample := int16(binary.LittleEndian.Uint16(r.data[r.offset : r.offset+2]))
		scaled := int32(float32(sample) * r.volume)
		if scaled > 32767 {
			scaled = 32767
		}
		if scaled < -32768 {
			scaled = -32768
		}
		binary.LittleEndian.PutUint16(dst[i:i+2], uint16(int16(scaled)))
		r.offset += 2
	}
	if len(dst)%2 != 0 {
		dst[len(dst)-1] = 0
	}
}

func (r *pcmReader) doneAfter(seconds float64) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		timer := timeNewTimer(seconds)
		<-timer
		close(ch)
	}()
	return ch
}
