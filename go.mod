module github.com/TxHandles/pokemon-survivor

go 1.24.7

// golang.org/x modules are fetched from their official GitHub mirrors
replace (
	golang.org/x/exp => github.com/golang/exp v0.0.0-20240909161429-701f63a606c0
	golang.org/x/image => github.com/golang/image v0.20.0
	golang.org/x/mod => github.com/golang/mod v0.21.0
	golang.org/x/sync => github.com/golang/sync v0.8.0
	golang.org/x/sys => github.com/golang/sys v0.30.0
	golang.org/x/text => github.com/golang/text v0.22.0
	golang.org/x/tools => github.com/golang/tools v0.25.0
)

require (
	github.com/hajimehoshi/ebiten/v2 v2.8.8
	github.com/hajimehoshi/go-mp3 v0.3.4
	github.com/pion/webrtc/v4 v4.0.14
	golang.org/x/image v0.20.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20240911145611-4856209ac325 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.3.3 // indirect
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/go-text/typesetting v0.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/pion/datachannel v1.5.10 // indirect
	github.com/pion/dtls/v3 v3.0.4 // indirect
	github.com/pion/ice/v4 v4.0.8 // indirect
	github.com/pion/interceptor v0.1.37 // indirect
	github.com/pion/logging v0.2.3 // indirect
	github.com/pion/mdns/v2 v2.0.7 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.15 // indirect
	github.com/pion/rtp v1.8.13 // indirect
	github.com/pion/sctp v1.8.37 // indirect
	github.com/pion/sdp/v3 v3.0.11 // indirect
	github.com/pion/srtp/v3 v3.0.4 // indirect
	github.com/pion/stun/v3 v3.0.0 // indirect
	github.com/pion/transport/v3 v3.0.7 // indirect
	github.com/pion/turn/v4 v4.0.0 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
)

replace golang.org/x/crypto => github.com/golang/crypto v0.33.0

replace golang.org/x/net => github.com/golang/net v0.35.0
