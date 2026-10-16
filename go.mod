module github.com/TxHandles/pokemon-survivor

go 1.24.7

// golang.org/x modules are fetched from their official GitHub mirrors
replace (
	golang.org/x/exp => github.com/golang/exp v0.0.0-20240909161429-701f63a606c0
	golang.org/x/image => github.com/golang/image v0.20.0
	golang.org/x/mod => github.com/golang/mod v0.21.0
	golang.org/x/sync => github.com/golang/sync v0.8.0
	golang.org/x/sys => github.com/golang/sys v0.25.0
	golang.org/x/text => github.com/golang/text v0.18.0
	golang.org/x/tools => github.com/golang/tools v0.25.0
)

require (
	github.com/hajimehoshi/ebiten/v2 v2.8.8
	golang.org/x/image v0.20.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20240911145611-4856209ac325 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.3.3 // indirect
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/go-text/typesetting v0.2.0 // indirect
	github.com/hajimehoshi/go-mp3 v0.3.4 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
)
