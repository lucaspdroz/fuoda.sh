FROM golang:1.23-bookworm

# libasound2/libpulse0: backends que o malgo carrega em runtime (ALSA/PulseAudio)
# pulseaudio-utils: paplay/pactl para testar o áudio
# openssh-client: para testar `ssh localhost -p 2222` de dentro do container
RUN apt-get update && apt-get install -y --no-install-recommends \
        libasound2 libasound2-plugins libpulse0 pulseaudio-utils \
        openssh-client make \
    && rm -rf /var/lib/apt/lists/*

# Faz o ALSA rotear para o PulseAudio (WSLg) caso o malgo escolha ALSA
RUN printf 'pcm.!default { type pulse }\nctl.!default { type pulse }\n' > /etc/asound.conf

ENV PATH="/root/.local/bin:${PATH}"
WORKDIR /app

CMD ["bash"]
