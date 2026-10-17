//go:build !js

package net

import (
	"errors"
	"io"
	"math/rand"
	"net/http"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
)

// Desktop networking: WebRTC data channels through pion, signalled through Firestore's REST API
// (the same lobby documents the browser builds use) or, for testing, an HTTP relay.

const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// FixedCode, when set, is used as the room code instead of a random one (testing).
var FixedCode string

// SignalBase is the HTTP test relay's base URL (NewNet("http")).
var SignalBase = "http://localhost:8773"

func randCode() string {
	if FixedCode != "" {
		return FixedCode
	}
	b := make([]byte, 5)
	for i := range b {
		b[i] = codeAlphabet[rand.Intn(len(codeAlphabet))]
	}
	return string(b)
}

var rtcConfig = webrtc.Configuration{ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302", "stun:stun1.l.google.com:19302"}}}}

// rtcAPI: no mDNS candidates (not needed between real addresses; noisy on some systems).
var rtcAPI = func() *webrtc.API {
	se := webrtc.SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	return webrtc.NewAPI(webrtc.WithSettingEngine(se))
}()

// ---- data channel connection ----

type pionConn struct {
	pc     *webrtc.PeerConnection
	dc     *webrtc.DataChannel
	mu     sync.Mutex
	q      []Msg
	open   bool
	closed bool
}

func newPionConn(pc *webrtc.PeerConnection) *pionConn {
	c := &pionConn{pc: pc}
	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		switch st {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateDisconnected:
			c.mu.Lock()
			c.closed = true
			c.mu.Unlock()
		}
	})
	return c
}

func (c *pionConn) attach(dc *webrtc.DataChannel, onOpen func()) {
	c.dc = dc
	dc.OnOpen(func() {
		c.mu.Lock()
		c.open = true
		c.mu.Unlock()
		if onOpen != nil {
			onOpen()
		}
	})
	dc.OnClose(func() { c.mu.Lock(); c.closed = true; c.mu.Unlock() })
	dc.OnMessage(func(m webrtc.DataChannelMessage) {
		var msg Msg
		if m.IsString {
			msg.Text = string(m.Data)
		} else {
			msg.Bin = append([]byte{}, m.Data...)
		}
		c.mu.Lock()
		c.q = append(c.q, msg)
		c.mu.Unlock()
	})
	if dc.ReadyState() == webrtc.DataChannelStateOpen {
		c.mu.Lock()
		c.open = true
		c.mu.Unlock()
		if onOpen != nil {
			onOpen()
		}
	}
}

func (c *pionConn) Send(b []byte) {
	if c.Open() {
		c.dc.Send(b)
	}
}
func (c *pionConn) SendText(s string) {
	if c.Open() {
		c.dc.SendText(s)
	}
}
func (c *pionConn) Recv() []Msg {
	c.mu.Lock()
	o := c.q
	c.q = nil
	c.mu.Unlock()
	return o
}
func (c *pionConn) Open() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.open && !c.closed && c.dc != nil && c.dc.ReadyState() == webrtc.DataChannelStateOpen
}
func (c *pionConn) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
func (c *pionConn) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	if c.dc != nil {
		c.dc.Close()
	}
	c.pc.Close()
}

// ---- signalling ----

type signal interface {
	listen(name string, onCode func(code string), onOffer func(id, name, sdp string, reply func(answer string)), onErr func(string)) func()
	offer(code, name, sdp string, onAnswer func(string), onErr func(string)) func()
	available() (bool, string)
	login(cb func(uid, name, err string))
	setOpen(code string, open bool)
}

type rtcNet struct{ sig signal }

// NewNet returns the desktop Net: Firestore signalling by default, or the HTTP test relay ("http").
func NewNet(mode string) Net {
	if mode == "http" {
		return &rtcNet{sig: &httpSignal{}}
	}
	return &rtcNet{sig: newFBRest()}
}

func (n *rtcNet) Available() (bool, string) { return n.sig.available() }

func (n *rtcNet) Login(cb func(uid, name, err string)) { n.sig.login(cb) }

func (n *rtcNet) SetRoomOpen(code string, open bool) { n.sig.setOpen(code, open) }

// gather waits for ICE gathering to finish (or a timeout) and returns the local description.
func gather(pc *webrtc.PeerConnection) string {
	done := webrtc.GatheringCompletePromise(pc)
	select {
	case <-done:
	case <-time.After(3500 * time.Millisecond):
	}
	if d := pc.LocalDescription(); d != nil {
		return d.SDP
	}
	return ""
}

func (n *rtcNet) Host(name string, onCode func(string), onPeer func(Peer), onErr func(string)) func() {
	var mu sync.Mutex
	var conns []*pionConn
	stop := n.sig.listen(name, onCode, func(id, gname, sdp string, reply func(string)) {
		pc, err := rtcAPI.NewPeerConnection(rtcConfig)
		if err != nil {
			onErr("WebRTC is not available: " + err.Error())
			return
		}
		c := newPionConn(pc)
		mu.Lock()
		conns = append(conns, c)
		mu.Unlock()
		pc.OnDataChannel(func(dc *webrtc.DataChannel) {
			c.attach(dc, nil)
			onPeer(Peer{UID: id, Name: gname, Conn: c})
		})
		if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp}); err != nil {
			onErr("Could not connect a player (" + err.Error() + ")")
			return
		}
		ans, err := pc.CreateAnswer(nil)
		if err == nil {
			err = pc.SetLocalDescription(ans)
		}
		if err != nil {
			onErr("Could not connect a player (" + err.Error() + ")")
			return
		}
		reply(gather(pc))
	}, onErr)
	return func() {
		stop()
		mu.Lock()
		for _, c := range conns {
			c.Close()
		}
		mu.Unlock()
	}
}

func (n *rtcNet) Join(code, name string, onConn func(Conn), onErr func(string)) func() {
	pc, err := rtcAPI.NewPeerConnection(rtcConfig)
	if err != nil {
		onErr("WebRTC is not available: " + err.Error())
		return func() {}
	}
	c := newPionConn(pc)
	ordered := true
	dc, err := pc.CreateDataChannel("c", &webrtc.DataChannelInit{Ordered: &ordered})
	if err != nil {
		onErr("WebRTC is not available: " + err.Error())
		return func() {}
	}
	c.attach(dc, func() { onConn(c) })
	var stop func()
	var mu sync.Mutex
	stopped := false
	go func() {
		off, err := pc.CreateOffer(nil)
		if err == nil {
			err = pc.SetLocalDescription(off)
		}
		if err != nil {
			onErr("WebRTC is not available: " + err.Error())
			return
		}
		sdp := gather(pc)
		mu.Lock()
		if stopped {
			mu.Unlock()
			return
		}
		stop = n.sig.offer(code, name, sdp, func(answer string) {
			if pc.RemoteDescription() != nil {
				return
			}
			if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
				onErr("Connection error: " + err.Error())
			}
		}, onErr)
		mu.Unlock()
	}()
	return func() {
		mu.Lock()
		stopped = true
		if stop != nil {
			stop()
		}
		mu.Unlock()
		c.Close()
	}
}

// ---- HTTP test relay (tools/sigserver.py) ----

type httpSignal struct{}

func (httpSignal) available() (bool, string) { return true, "" }

func (httpSignal) login(cb func(uid, name, err string)) {
	cb("local-"+randCode()+randCode(), localName(), "")
}

func (httpSignal) setOpen(string, bool) {}

func httpDo(method, url, body string) (int, string, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	cl := &http.Client{Timeout: 10 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

func (httpSignal) listen(name string, onCode func(string), onOffer func(id, name, sdp string, reply func(string)), onErr func(string)) func() {
	code := randCode()
	stop := make(chan struct{})
	go func() {
		if _, _, err := httpDo("POST", SignalBase+"/sig/host?code="+code, name); err != nil {
			onErr("Could not open a room: " + err.Error())
			return
		}
		onCode(code)
		seen := map[string]bool{}
		for {
			select {
			case <-stop:
				return
			case <-time.After(500 * time.Millisecond):
			}
			_, t, err := httpDo("GET", SignalBase+"/sig/offers?code="+code, "")
			if err != nil {
				continue
			}
			for _, ln := range strings.Split(t, "\n") {
				parts := strings.SplitN(ln, "\t", 3)
				if len(parts) < 3 || seen[parts[0]] {
					continue
				}
				seen[parts[0]] = true
				id := parts[0]
				go onOffer(id, parts[1], strings.ReplaceAll(parts[2], "\x01", "\n"), func(answer string) {
					httpDo("POST", SignalBase+"/sig/answer?code="+code+"&id="+id, answer)
				})
			}
		}
	}()
	return func() { close(stop) }
}

func (httpSignal) offer(code, name, sdp string, onAnswer func(string), onErr func(string)) func() {
	id := randCode() + randCode()
	stop := make(chan struct{})
	go func() {
		st, _, err := httpDo("POST", SignalBase+"/sig/offer?code="+code+"&id="+id+"&name="+name, sdp)
		if err != nil {
			onErr("Could not join: " + err.Error())
			return
		}
		if st != 200 {
			onErr("No room with that code.")
			return
		}
		for {
			select {
			case <-stop:
				return
			case <-time.After(500 * time.Millisecond):
			}
			st, t, err := httpDo("GET", SignalBase+"/sig/answer?code="+code+"&id="+id, "")
			if err == nil && st == 200 && t != "" {
				onAnswer(t)
				return
			}
		}
	}()
	return func() { close(stop) }
}

var errNoNet = errors.New("no network")

func localName() string {
	if u, err := user.Current(); err == nil {
		n := u.Username
		if i := strings.LastIndexAny(n, `\/`); i >= 0 {
			n = n[i+1:]
		}
		if n != "" {
			return n
		}
	}
	return "Player"
}
