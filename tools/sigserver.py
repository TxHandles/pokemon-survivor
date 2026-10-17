# serves build/web and relays WebRTC offers / answers under /sig/ (test only)
import http.server,urllib.parse,sys,threading
rooms={}  # code -> {id: [name, offer, answer]}
class H(http.server.SimpleHTTPRequestHandler):
    def log_message(self,*a): pass
    def _q(self):
        u=urllib.parse.urlparse(self.path);return u.path,dict(urllib.parse.parse_qsl(u.query))
    def _send(self,code,body=''):
        b=body.encode();self.send_response(code);self.send_header('Content-Type','text/plain');self.send_header('Content-Length',str(len(b)));self.end_headers();self.wfile.write(b)
    def do_GET(self):
        p,q=self._q()
        if p=='/sig/offers':
            r=rooms.get(q.get('code'),{});self._send(200,''.join(f"{i}\t{v[0]}\t{v[1].replace(chr(10),chr(1))}\n" for i,v in r.items()));return
        if p=='/sig/answer':
            v=rooms.get(q.get('code'),{}).get(q.get('id'))
            if v and v[2]: self._send(200,v[2])
            else: self._send(204)
            return
        super().do_GET()
    def do_POST(self):
        p,q=self._q();n=int(self.headers.get('Content-Length',0));body=self.rfile.read(n).decode()
        if p=='/sig/host': rooms.setdefault(q['code'],{});self._send(200);return
        if p=='/sig/offer':
            if q['code'] not in rooms: self._send(404);return
            rooms[q['code']][q['id']]=[q.get('name','Trainer'),body,''];self._send(200);return
        if p=='/sig/answer':
            rooms[q['code']][q['id']][2]=body;self._send(200);return
        self._send(404)
http.server.ThreadingHTTPServer(('127.0.0.1',int(sys.argv[1])),H).serve_forever()
