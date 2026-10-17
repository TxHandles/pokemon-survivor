//go:build !js

package net

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Firestore through its REST API, for the desktop build: the same lobbies/{code} and join/{uid}
// documents the browser builds use, so a Windows player can host or join a browser player's room.
// The desktop signs in anonymously (Firebase Authentication → Anonymous must be enabled on the
// project), which satisfies the security rules' "signed in" checks.

const (
	fbAPIKey  = "AIzaSyArpClEYGkFOq552fSgMYEwRIQlcJs5NJU"
	fbProject = "pokemon-survivors-e8213"
)

var fbDocs = "https://firestore.googleapis.com/v1/projects/" + fbProject + "/databases/(default)/documents"

type fbRest struct {
	mu    sync.Mutex
	uid   string
	token string
	exp   time.Time
	name  string
	cl    *http.Client
}

func newFBRest() *fbRest {
	return &fbRest{cl: &http.Client{Timeout: 15 * time.Second}, name: localName()}
}

func (f *fbRest) available() (bool, string) { return true, "" }

// login signs in anonymously (once) and reports the uid.
func (f *fbRest) login(cb func(uid, name, err string)) {
	go func() {
		if err := f.ensureToken(); err != nil {
			cb("", "", err.Error())
			return
		}
		cb(f.uid, f.name, "")
	}()
}

func (f *fbRest) ensureToken() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" && time.Now().Before(f.exp) {
		return nil
	}
	resp, err := f.cl.Post("https://identitytoolkit.googleapis.com/v1/accounts:signUp?key="+fbAPIKey, "application/json", strings.NewReader(`{"returnSecureToken":true}`))
	if err != nil {
		return fmt.Errorf("could not reach the sign-in service (%v)", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var r struct {
		IDToken   string `json:"idToken"`
		LocalID   string `json:"localId"`
		ExpiresIn string `json:"expiresIn"`
		Error     struct{ Message string }
	}
	json.Unmarshal(b, &r)
	if resp.StatusCode != 200 || r.IDToken == "" {
		msg := r.Error.Message
		if msg == "ADMIN_ONLY_OPERATION" || strings.Contains(msg, "OPERATION_NOT_ALLOWED") {
			return fmt.Errorf("anonymous sign-in is not enabled on the Firebase project (Authentication → Sign-in method → Anonymous)")
		}
		return fmt.Errorf("sign-in failed: %s", nonEmpty(msg, resp.Status))
	}
	f.token, f.uid = r.IDToken, r.LocalID
	f.exp = time.Now().Add(50 * time.Minute)
	return nil
}

// doc is a Firestore document body.
type fsDoc struct {
	Name   string                    `json:"name,omitempty"`
	Fields map[string]map[string]any `json:"fields"`
}

func fsStr(s string) map[string]any { return map[string]any{"stringValue": s} }
func fsBool(b bool) map[string]any  { return map[string]any{"booleanValue": b} }
func fsInt(n int64) map[string]any  { return map[string]any{"integerValue": fmt.Sprint(n)} }

func (d *fsDoc) str(k string) string {
	if d == nil || d.Fields[k] == nil {
		return ""
	}
	s, _ := d.Fields[k]["stringValue"].(string)
	return s
}

func (d *fsDoc) boolean(k string, def bool) bool {
	if d == nil || d.Fields[k] == nil {
		return def
	}
	b, ok := d.Fields[k]["booleanValue"].(bool)
	if !ok {
		return def
	}
	return b
}

func (d *fsDoc) id() string {
	if i := strings.LastIndexByte(d.Name, '/'); i >= 0 {
		return d.Name[i+1:]
	}
	return d.Name
}

// call does an authenticated Firestore REST request. 404 is returned as status with no error.
func (f *fbRest) call(method, path string, q string, body any) (int, []byte, error) {
	if err := f.ensureToken(); err != nil {
		return 0, nil, err
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, fbDocs+path+q, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := f.cl.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, nil
}

func (f *fbRest) get(path string) (*fsDoc, int, error) {
	st, b, err := f.call("GET", path, "", nil)
	if err != nil || st != 200 {
		return nil, st, err
	}
	var d fsDoc
	if json.Unmarshal(b, &d) != nil {
		return nil, st, fmt.Errorf("bad document")
	}
	return &d, st, nil
}

// patch writes fields (creating the document if needed); only the given fields are touched.
func (f *fbRest) patch(path string, fields map[string]map[string]any) error {
	q := ""
	for k := range fields {
		if q == "" {
			q = "?"
		} else {
			q += "&"
		}
		q += "updateMask.fieldPaths=" + k
	}
	st, b, err := f.call("PATCH", path, q, fsDoc{Fields: fields})
	if err != nil {
		return err
	}
	if st != 200 {
		return fmt.Errorf("firestore %d: %s", st, trim(string(b)))
	}
	return nil
}

func trim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

func (f *fbRest) setOpen(code string, open bool) {
	if code == "" {
		return
	}
	go f.patch("/lobbies/"+code, map[string]map[string]any{"open": fsBool(open)})
}

func (f *fbRest) listen(name string, onCode func(string), onOffer func(id, name, sdp string, reply func(string)), onErr func(string)) func() {
	stop := make(chan struct{})
	var code string
	go func() {
		if err := f.ensureToken(); err != nil {
			onErr(err.Error())
			return
		}
		for try := 0; ; try++ {
			code = randCode()
			if _, st, err := f.get("/lobbies/" + code); err != nil {
				onErr("Could not open a room: " + err.Error())
				return
			} else if st == 404 || try >= 8 {
				break
			}
		}
		err := f.patch("/lobbies/"+code, map[string]map[string]any{
			"host": fsStr(f.uid), "hostName": fsStr(name), "open": fsBool(true), "ts": fsInt(time.Now().UnixMilli()), "eng": fsStr("go"), "proto": fsInt(Protocol)})
		if err != nil {
			onErr("Could not open a room: " + err.Error())
			return
		}
		onCode(code)
		seen := map[string]bool{}
		for {
			select {
			case <-stop:
				f.call("DELETE", "/lobbies/"+code, "", nil)
				return
			case <-time.After(time.Second):
			}
			st, b, err := f.call("GET", "/lobbies/"+code+"/join", "", nil)
			if err != nil || st != 200 {
				continue
			}
			var r struct{ Documents []fsDoc }
			json.Unmarshal(b, &r)
			for i := range r.Documents {
				d := r.Documents[i]
				id := d.id()
				if seen[id] || d.str("offer") == "" || d.str("answer") != "" {
					continue
				}
				seen[id] = true
				go onOffer(id, nonEmpty(d.str("name"), "Trainer"), d.str("offer"), func(answer string) {
					f.patch("/lobbies/"+code+"/join/"+id, map[string]map[string]any{"answer": fsStr(answer)})
				})
			}
		}
	}()
	return func() { close(stop) }
}

func (f *fbRest) offer(code, name, sdp string, onAnswer func(string), onErr func(string)) func() {
	stop := make(chan struct{})
	go func() {
		if err := f.ensureToken(); err != nil {
			onErr(err.Error())
			return
		}
		d, st, err := f.get("/lobbies/" + code)
		if err != nil {
			onErr("Could not look up the room: " + err.Error())
			return
		}
		if st == 404 || d == nil {
			onErr("No room with that code.")
			return
		}
		if d.str("eng") != "go" {
			onErr("That room was made by the web version of the game; this version can only join rooms hosted by the same version.")
			return
		}
		if !d.boolean("open", true) {
			onErr("That room is in a run. Try again when it finishes.")
			return
		}
		path := "/lobbies/" + code + "/join/" + f.uid
		if err := f.patch(path, map[string]map[string]any{"name": fsStr(name), "offer": fsStr(sdp), "ts": fsInt(time.Now().UnixMilli())}); err != nil {
			onErr("Could not join: " + err.Error())
			return
		}
		defer f.call("DELETE", path, "", nil)
		for {
			select {
			case <-stop:
				return
			case <-time.After(time.Second):
			}
			d, _, err := f.get(path)
			if err == nil && d != nil && d.str("answer") != "" {
				onAnswer(d.str("answer"))
				return
			}
		}
	}()
	return func() { close(stop) }
}
