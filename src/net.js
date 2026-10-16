/* ===== ONLINE CO-OP (web build). Firestore = matchmaking/signaling, WebRTC data channels = game traffic.
   The host runs the real game; guests send their stick input and draw the host's snapshots. ===== */
const RTC_CFG={iceServers:[{urls:['stun:stun.l.google.com:19302','stun:stun1.l.google.com:19302','stun:stun2.l.google.com:19302']}]};
let NETH=0,NETG=0,LOB=null,LOBRET=0,NETLAST=0,NID=0,NG=null,INVUNSUB=null,INVSEEN={};
const NET={peers:{},host:null,unsubs:[]};
const CODEAL='ABCDEFGHJKMNPQRSTUVWXYZ23456789';
const myName=()=>frName();
function lobMsg(s,bad){const m=$('lobmsg');if(m){m.textContent=s||'';m.style.color=bad?'#ff8a8a':''}}
function iceDone(pc,ms){return new Promise(r=>{if(pc.iceGatheringState==='complete')return r();const t=setTimeout(r,ms||3500);pc.addEventListener('icegatheringstatechange',()=>{if(pc.iceGatheringState==='complete'){clearTimeout(t);r()}})})}
function chSend(ch,o){try{if(ch&&ch.readyState==='open')ch.send(typeof o==='string'?o:JSON.stringify(o))}catch(e){}}
/* ---------- profile a guest sends so the host builds their Pokemon with THEIR upgrades ---------- */
function myProf(cid){const s0=SEL,ps0=PS,psn0=PSN;SEL=cid;setPS(cid);const ulv={};for(const k of ULK())ulv[k]=ul(k);const o={ulv,mr:mrank(cid),PS:Object.assign({},PS),PSN};SEL=s0;PS=ps0;PSN=psn0;return o}
/* =================== HOST =================== */
async function hostLobby(){if(!FBU)return lobMsg('Sign in with Google first.',1);await leaveLobby(true);
 let code='';for(let t=0;t<8;t++){code='';for(let i=0;i<5;i++)code+=CODEAL[Math.random()*CODEAL.length|0];const ex=await FB.collection('lobbies').doc(code).get().catch(()=>null);if(ex&&!ex.exists)break}
 await FB.collection('lobbies').doc(code).set({host:FBU.uid,hostName:myName(),open:true,ts:Date.now()});
 NETH=1;LOB={code,me:FBU.uid,host:FBU.uid,players:[{uid:FBU.uid,name:myName(),cid:SEL,ready:1}],state:'lobby'};
 NET.unsubs.push(FB.collection('lobbies').doc(code).collection('join').onSnapshot(sn=>{sn.docChanges().forEach(ch=>{if(ch.type==='added'){const d=ch.doc.data();if(d.offer&&!d.answer)hostAccept(ch.doc.id,d)}})},e=>lobMsg('Lobby error: '+(e.code||e.message),1)));
 lobUI();lobBroadcast()}
async function hostAccept(guid,d){if(NET.peers[guid])return;if(LOB.players.length>=4||LOB.state!=='lobby'){return}
 const pc=new RTCPeerConnection(RTC_CFG),pe_={pc,uid:guid,name:d.name||'Trainer',c:null,s:null,lr:performance.now()+15000};NET.peers[guid]=pe_;
 pc.ondatachannel=ev=>{const ch=ev.channel;if(ch.label==='c'){pe_.c=ch;ch.onmessage=m=>hostMsg(guid,m.data);ch.onclose=()=>peerGone(guid)}else{pe_.s=ch;ch.onmessage=m=>hostMsg(guid,m.data)}};
 pc.onconnectionstatechange=()=>{if(['failed','closed'].includes(pc.connectionState))peerGone(guid)};
 try{await pc.setRemoteDescription({type:'offer',sdp:d.offer});const a=await pc.createAnswer();await pc.setLocalDescription(a);await iceDone(pc);
  await FB.collection('lobbies').doc(LOB.code).collection('join').doc(guid).update({answer:pc.localDescription.sdp})}catch(e){lobMsg('Could not connect a player ('+(e.code||e.message)+')',1);peerGone(guid)}}
function hostMsg(guid,raw){let m;try{m=JSON.parse(raw)}catch(e){return}const pe_=NET.peers[guid];if(!pe_)return;pe_.lr=performance.now();if(m.t==='hb')return;
 if(m.t==='in'){const i=pe_.pi;if(i!=null&&PL[i])PL[i].inp={x:+m.x||0,y:+m.y||0};return}  if(m.t==='dash'){const i=pe_.pi;if(i!=null&&PL[i])PL[i].dq=T;return}
 if(m.t==='hello'){if(LOB.state!=='lobby'||LOB.players.length>=4){chSend(pe_.c,{t:'full'});return}pe_.name=String(m.name||'Trainer').slice(0,16);pe_.vw=m.vw||900;pe_.vh=m.vh||900;
  if(!LOB.players.some(p=>p.uid===guid))LOB.players.push({uid:guid,name:pe_.name,cid:m.cid,ready:0,prof:m.prof});lobBroadcast();lobUI();return}
 if(m.t==='pick'){const p=LOB.players.find(p=>p.uid===guid);if(!p)return;const tk=lobTaken(m.cid,guid);if(tk&&m.cid!==p.cid){chSend(pe_.c,{t:'taken',cid:m.cid,by:tk.name});return}p.cid=m.cid;p.prof=m.prof;p.ready=0;lobBroadcast();lobUI();return}
 if(m.t==='ready'){const p=LOB.players.find(p=>p.uid===guid);if(p&&!p.dup){p.ready=m.r?1:0;lobBroadcast();lobUI()}return}
 if(m.t==='rvote'){if(pe_.pi!=null&&ROUTE)routeVote(pe_.pi,m.ix|0);return}
 if(m.t==='pause'){if(state==='play'){NETSUP=1;pauseGame();NETSUP=0;pauseBy(pe_.name);netBcast({t:'pause',by:pe_.name},guid)}return}
 if(m.t==='resume'){if(state==='pause'){NETSUP=1;resumeGame();NETSUP=0;netBcast({t:'resume'},guid)}return}
 if(m.t==='ans'){if(pe_.pi!=null)coopAnswer(pe_.pi,m.ix|0);return}
 if(m.t==='bye')peerGone(guid)}
function peerGone(guid){const pe_=NET.peers[guid];if(!pe_)return;delete NET.peers[guid];try{pe_.pc.close()}catch(e){}
 if(LOB){if(LOB.state==='lobby'){LOB.players=LOB.players.filter(p=>p.uid!==guid);lobBroadcast();lobUI()}else if(pe_.pi!=null&&PL[pe_.pi]){const c=PL[pe_.pi];c.gone=1;c.q=[];c.wait=null;FX.push({k:'txt',x:P.x,y:P.y-70,s:'⛔ '+c.name+' disconnected',c:'#ff8a8a',l:3});if(state==='lvl')coopPrompt();if(ROUTE)routeCheck()}}}
const lobTaken=(cid,uid)=>LOB&&LOB.players.find(p=>p.uid!==uid&&p.cid===cid);
const cnm=k=>CHR[k]?CHR[k].n:(starters.find(a=>a[0]===k)||[0,k])[1];
function lobBroadcast(){if(!NETH||!LOB)return;LOB.players.forEach(p=>{p.dup=p.uid!==LOB.host&&!!lobTaken(p.cid,p.uid)?1:0;if(p.dup)p.ready=0});const o={t:'lobby',code:LOB.code,rl:runLabel(),players:LOB.players.map(p=>({uid:p.uid,name:p.name,cid:p.cid,ready:p.ready,dup:p.dup}))};for(const g in NET.peers)chSend(NET.peers[g].c,o);
 FB.collection('lobbies').doc(LOB.code).set({n:LOB.players.length},{merge:true}).catch(()=>{})}
function hostStart(){if(!NETH||!LOB)return;LOB.players[0].cid=SEL;const ps=LOB.players.filter(p=>p.uid===FBU.uid||NET.peers[p.uid]);if(ps.length<2)return lobMsg('Need at least one friend in the lobby.',1);if(new Set(ps.map(p=>p.uid===FBU.uid?SEL:p.cid)).size<ps.length)return lobMsg('Two players picked the same Pokémon. Each player needs a different one.',1);if(ps.some(p=>p.uid!==FBU.uid&&!p.ready))return lobMsg('Waiting for everyone to tap Ready.',1);
 LOB.state='play';FB.collection('lobbies').doc(LOB.code).set({open:false},{merge:true}).catch(()=>{});
 const list=ps.map(p=>p.uid===FBU.uid?{cid:SEL,kind:'local',name:p.name}:{cid:p.cid,kind:'net',name:p.name,prof:p.prof,uid:p.uid});
 beginCoop(list,null);list.forEach((q,i)=>{if(q.uid&&NET.peers[q.uid])NET.peers[q.uid].pi=i});
 const run=JSON.parse(JSON.stringify(RUN));for(const g in NET.peers){const pe_=NET.peers[g];if(pe_.pi==null)continue;chSend(pe_.c,{t:'start',me:pe_.pi,run,WS,BS,BI,LI,players:PL.map(c=>({name:c.name,cid:c.P.cid}))})}
 NETLAST=0}
function netHooks(){COROUTE=(ev,R)=>{if(!NETH)return;if(ev==='open')netBcast({t:'route',opts:R.opts,cur:R.cur,votes:R.votes,figs:R.figs});else if(ev==='votes')netBcast({t:'rvotes',votes:R.votes});else if(ev==='end')netBcast({t:'rend',ix:R.ix,tt:R.t,li:R.li})};COPROMPT=(i,w)=>{const pe_=Object.values(NET.peers).find(p=>p.pi===i);if(!pe_)return coopAnswer(i,0);const c=PL[i];chSend(pe_.c,{t:'prompt',k:w.k,o:w.o,sf:w.sf,pc:w.pc,ml:c.ml,evo:c.evo,pl:c.pl})};}
window.COEND=S=>{if(!NETH)return;for(const g in NET.peers){const pe_=NET.peers[g];if(pe_.pi==null)continue;const c=PL[pe_.pi];chSend(pe_.c,{t:'end',stand:S,me:{k:c.P.k||0,rc:c.P.rc||0,tk:c.P.tk||0,lv:c.P.lv,ek:c.P.ek||0,bk:c.P.bk||0,T,win:S.win,DMG:c.DMG,dead:!!c.P.dead,mbk:c.P.mbk||0,tdk:c.P.tdk||0,ccn:c.P.ccn||0,pcn:c.P.pcn||0,rv:c.P.rsc||0,pl:PL.length,mvp:(c.P.k||0)>0&&(c.P.k||0)===Math.max(...PL.map(q=>q.P.k||0)),ab:c.PS&&c.PS.ab}})}
 if(LOB)LOB.state='lobby';setTimeout(()=>{if(LOB&&NETH){LOB.players=LOB.players.filter(p=>p.uid===FBU.uid||NET.peers[p.uid]);LOB.players.forEach(p=>{if(p.uid!==FBU.uid)p.ready=0});FB.collection('lobbies').doc(LOB.code).set({open:true},{merge:true}).catch(()=>{});lobBroadcast()}},500)};
/* ---------- snapshots ---------- */
const NSKIP=new Set(['h','tg','ref','fn','src','guard','nt','ntT','o','bv','ban','ulv','q','g']);
function ser(o){const r={};for(const k in o){if(NSKIP.has(k)||(k[0]==='_'&&k!=='_sid'))continue;const v=o[k],t=typeof v;
 if(t==='number'){if(isFinite(v))r[k]=Math.round(v*10)/10}else if(t==='string'||t==='boolean')r[k]=v;
 else if(v&&t==='object'){if(Array.isArray(v)){if(v.length<=12&&v.every(x=>typeof x!=='object'||(x&&!Array.isArray(x)&&Object.keys(x).length<=5)))r[k]=v}else if(!(v instanceof Set||v instanceof Map)){const ks=Object.keys(v);if(ks.length<=5&&ks.every(q=>typeof v[q]!=='object'&&typeof v[q]!=='function'))r[k]=v}}}return r}
const ESTAT=['fz','su','cm','cf','fe','rt','bt','pt','sl','fl','wu','cwu','ps','pf','dtr'];
/* enemy -> compact array [id,x,y,r,sprite,type,hp%,extra?]; extra only carries what's active right now */
function serE(e){const x={};for(const k of ESTAT){const v=e[k];if(v>0)x[k]=Math.round(v*100)/100}for(const k of ['boss','elite','mini','big','ug'])if(e[k])x[k]=1;for(const k of ['rk','sp2','k'])if(e[k])x[k]=e[k];
 if(e.af&&e.af.length)x.af=e.af;if(e.sh>0)x.sh=Math.round(e.sh/e.mhp*1000)/10;if(e.cwu>0){x.cang=Math.round(e.cang*100)/100;x.sp=Math.round(e.sp)}if(e.wu>0&&e.ca!==undefined)x.ca=Math.round(e.ca*100)/100;if(e.boss){x.ph=e.ph||1;if(e.bd)x.bd={n:e.bd.n,em:e.bd.em}}
 const a=[e.id,Math.round(e.x),Math.round(e.y),e.r,e._sid||0,e.type,Math.max(0,Math.round(e.hp/e.mhp*100))];for(const k in x){a.push(x);break}return a}
function deE(a){const e={id:a[0],x:a[1],y:a[2],r:a[3],_sid:a[4]||null,type:a[5],hp:a[6],mhp:100};if(a[7])Object.assign(e,a[7]);if(e._sid===null)delete e._sid;return e}
const inV=(o,cx,cy,hw,hh)=>Math.abs(o.x-cx)<hw&&Math.abs(o.y-cy)<hh;
let NDIE=[];
function snapFor(i,pe_){const me=PL[i].P,hw=(pe_.vw||900)/2+160,hh=(pe_.vh||900)/2+160,cx=me.dead?(PL[camIdx()]||PL[i]).P.x:me.x,cy=me.dead?(PL[camIdx()]||PL[i]).P.y:me.y;
 const L=(a,f)=>{const o=[];for(const x of a)if(x.x===undefined||inV(x,cx,cy,hw,hh))o.push(f?f(x):ser(x));return o};
 for(const e of E){if(e.id===undefined)e.id=++NID;espId(e)}for(const p of PR)if(p.id===undefined)p.id=++NID;
 const pls=PL.map((c,j)=>{const p=c.P,o={x:Math.round(p.x),y:Math.round(p.y),hp:Math.round(p.hp),mhp:p.mhp,d:p.dead?1:0,g:c.gone?1:0,cid:p.cid,fx:p.fx||1,inv:p.inv>0?1:0,mv:p.mv?1:0,st:p.st||0,wb:p.webT>0?Math.round(p.webT*10)/10:0,sa:Math.round((p.sa||0)*100)/100,f:[Math.round(c.face.x*100)/100,Math.round(c.face.y*100)/100],
  OB:c.OB.map(b=>[Math.round(b.x),Math.round(b.y),b.t,b.mv,Math.round(b.rs*10)/10]),CL:c.CLN.map(b=>[Math.round(b.x),Math.round(b.y)]),SH:c.SH,ml:c.ml,pl:c.pl,atk:p.atkT!=null?Math.round((T-p.atkT)*100)/100:9};
  if(j===i){o.lv=p.lv;o.xp=Math.round(p.xp);o.nx=p.nx;o.rc=Math.round(p.rc||0);o.tk=p.tk||0;o.evo=c.evo;o.k=p.k||0;o.pm=p.pm||0;o.pbl=p.pbl||0;o.phs=p.phs||0;o.mg=p.mg||0;o.bat=p.bat||0;o.bel=p.bel||0}return o});
 return JSON.stringify({t:'s',T:Math.round(T*100)/100,BI,BS,LI,LM:LMSP.map(o=>[Math.round(o.x),Math.round(o.y),o.lm,o.sc]),kl:kills,en:endless,lvw:state==='lvl'?1:0,P:pls,
  E:L(E,serE),PR:L(PR),FX:L(FX.filter(f=>f.k!=='flash'||f.l>0)),ZN:L(ZN),EB:L(EB),HZ:L(HZ),G:L(G,g=>[Math.round(g.x),Math.round(g.y),g.v>3?2:g.v?1:0,g.c?1:0,g.tk?1:0]),
  CH:L(CH),CY:L(CY),MC:L(MC),IT:L(IT),PC:PCN?{x:Math.round(PCN.x),y:Math.round(PCN.y),p:Math.round(PCN.p*100)/100}:null,BN:COBAN,DD:NDIE.filter(d=>inV(d,cx,cy,hw,hh)).map(d=>d.id),gu:gust?ser(gust):null})}
function netHostTick(){if(!NETH||!COOP)return;const now=performance.now();if(now-NETLAST<66)return;NETLAST=now;
 for(const g in NET.peers){const pe_=NET.peers[g];if(pe_.pi==null||!pe_.s)continue;if(pe_.s.bufferedAmount>256e3)continue;chSend(pe_.s,snapFor(pe_.pi,pe_))}NDIE=[]}
/* =================== GUEST =================== */
async function joinLobby(code){if(!FBU)return lobMsg('Sign in with Google first.',1);code=String(code||'').toUpperCase().replace(/[^A-Z0-9]/g,'');if(code.length!==5)return lobMsg('Room codes are 5 characters.',1);
 await leaveLobby(true);lobMsg('Connecting…');const ld=await FB.collection('lobbies').doc(code).get().catch(e=>null);if(!ld||!ld.exists)return lobMsg('❌ No lobby with that code.',1);if(!ld.data().open)return lobMsg('❌ That lobby is in a game. Try again when it finishes.',1);
 const pc=new RTCPeerConnection(RTC_CFG),c=pc.createDataChannel('c'),s=pc.createDataChannel('s',{ordered:false,maxRetransmits:0});NET.host={pc,c,s,code,lr:performance.now()+15000};NETG=0;
 LOB={code,me:FBU.uid,host:ld.data().host,players:[],state:'lobby',hostName:ld.data().hostName};lobUI();
 c.onopen=()=>{lobMsg('');chSend(c,{t:'hello',name:myName(),cid:SEL,prof:myProf(SEL),vw:W,vh:H})};c.onmessage=m=>guestMsg(m.data);s.onmessage=m=>guestMsg(m.data);c.onclose=()=>hostGone();
 pc.onconnectionstatechange=()=>{if(['failed','closed'].includes(pc.connectionState))hostGone()};
 const of=await pc.createOffer();await pc.setLocalDescription(of);await iceDone(pc);
 const jr=FB.collection('lobbies').doc(code).collection('join').doc(FBU.uid);await jr.set({name:myName(),offer:pc.localDescription.sdp,ts:Date.now()});
 const to=setTimeout(()=>{if(c.readyState!=='open'){lobMsg('❌ Could not reach the host. Their network may block direct connections.',1)}},15000);
 NET.unsubs.push(jr.onSnapshot(async sn=>{const d=sn.data();if(d&&d.answer&&!pc.remoteDescription){try{await pc.setRemoteDescription({type:'answer',sdp:d.answer})}catch(e){lobMsg('Connection error: '+e.message,1)}}}));
 c.addEventListener('open',()=>clearTimeout(to))}
function hostGone(){if(!NET.host)return;NET.host=null;if(NETG&&(state==='play'||state==='lvl'||state==='pause')){NETG=0;$('pause').classList.remove('on');state='play';guestFinish(null,'Host disconnected · your rewards were banked');LOB=null}else{LOB=null;lobUI();lobMsg('The host closed the lobby.',1)}NETG=0}
function guestMsg(raw){let m;try{m=JSON.parse(raw)}catch(e){return}if(NET.host)NET.host.lr=performance.now();if(m.t==='hb')return;
 if(m.t==='s'){if(NETG)netApply(m);return}
 if(m.t==='lobby'){if(!LOB)return;LOB.players=m.players;LOB.code=m.code;LOB.rl=m.rl;lobUI();return}
 if(m.t==='taken'){const me=LOB&&LOB.players.find(p=>p.uid===FBU.uid);if(me)SEL=me.cid;lobMsg('❌ '+m.by+' is already using '+cnm(m.cid)+'. Pick a different Pokémon.',1);lobUI();return}
 if(m.t==='route'){if(!NETG)return;NG.route={opts:m.opts,cur:m.cur,votes:m.votes||{},coop:true,figs:m.figs};guestRouteUI();return}
 if(m.t==='rvotes'){if(NG&&NG.route){NG.route.votes=m.votes;guestRouteUI()}return}
 if(m.t==='rend'){const R=NG&&NG.route;if(!R){$('route').classList.remove('on');return}$('rtwait').textContent='Next stop: '+locN(m.tt,m.li);setTimeout(()=>routeAnim(R,m.ix,LOCAL,()=>{if(NG)NG.route=null;$('route').classList.remove('on')}),500);return}
 if(m.t==='boss'){if(!NETG)return;useP(LOCAL);const msg=bossUnlock(m.ev.bi,m.ev.et,m.ev.fin,m.ev.sg,P.cid,P.bv);if(msg.length)LOCBAN={s:msg,until:performance.now()+4500};try{save()}catch(e){}return}
 if(m.t==='pause'){if(state==='play'){NETSUP=1;pauseGame();NETSUP=0;pauseBy(m.by)}return}
 if(m.t==='resume'){if(state==='pause'){NETSUP=1;resumeGame();NETSUP=0}return}
 if(m.t==='full'){lobMsg('❌ That lobby is full or already playing.',1);leaveLobby(true);return}
 if(m.t==='start'){guestStart(m);return}
 if(m.t==='prompt'){guestPrompt(m);return}
 if(m.t==='end'){guestFinish(m);return}if(m.t==='bye')hostGone()}
function guestStart(m){LOB.state='play';const me=m.players[m.me];NETG=1;COOP=0;begin(me.cid);RUN=m.run;DAILY=null;
 LI=m.LI||0;enterBiome(m.BI,true);BS=m.BS;WS=m.WS;OBM.clear();OBF.clear();HZP.clear();E=[];G=[];PR=[];FX=[];ZN=[];EB=[];HZ=[];
 const base={};cS(base);PL=m.players.map((q,i)=>{if(i===m.me){base.P.pi=i;Object.assign(base,{name:q.name,kind:'local',inp:{x:0,y:0},q:[],showing:0,wait:null,gone:0});return base}
  const p={x:0,y:0,hp:1,mhp:1,cid:q.cid,type:ctype(q.cid),ty:ctypes(q.cid),inv:0,st:0,pi:i,sp:125};return {P:p,ml:{},mt:{},evo:{},pl:Object.fromEntries(Object.keys(PASS).map(k=>[k,0])),PS:Object.assign({},PSD),PSN:'',SH:{},CLN:[],OB:[],face:{x:1,y:0},DMG:{},DLOG:[],DTOT:{},KB:null,dmx:1,kind:'net',name:q.name,inp:{},q:[],gone:0}});
 LOCAL=m.me;CUR=m.me;COOP=1;XPS={xp:0,lv:1,nx:5};NG={last:0,int:66,map:new Map(),inT:0,snapT:0,lvw:0,recvT:performance.now()};T=0;P.x=0;P.y=0;
 document.querySelectorAll('.ov.on').forEach(o=>o.classList.remove('on'));state='play';last=performance.now();requestAnimationFrame(loop)}
function guestPrompt(m){if(!NETG)return;useP(LOCAL);Object.assign(ml,m.ml);for(const k in m.ml)if(mt[k]===undefined)mt[k]=0;for(const k in evo)if(!m.evo[k])delete evo[k];Object.assign(evo,m.evo);Object.assign(pl,m.pl);
 if(m.k==='lv'){NG.prompt=m;choices(true,m.o)}else if(m.k==='shrine'){NG.prompt=m;shrineUI(m,ix=>{guestAnswer(ix);state='play';last=performance.now();requestAnimationFrame(loop)})}}
function guestAnswer(ix){chSend(NET.host&&NET.host.c,{t:'ans',ix});NG.prompt=null;$('lvl').classList.remove('on');$('lvtitle').textContent='⏳ Waiting for the others…';$('choices').innerHTML='';$('lvtools').innerHTML='';$('lvl').classList.add('on');NG.waitShown=1}
/* apply one host snapshot: replace the world, keep per-object animation state, set up interpolation */
function netApply(m){const now=performance.now();NG.int=NG.last?Math.min(200,Math.max(30,now-NG.last)):66;NG.last=now;NG.snapT=m.T;NG.recvT=now;
 if(m.LI!==undefined)LI=m.LI;if(m.BI!==BI){enterBiome(m.BI,false);}if(m.LM){const k=JSON.stringify(m.LM);if(k!==NG.lmk){NG.lmk=k;LMSP=m.LM.map(a=>({x:a[0],y:a[1],r:30,lm:a[2],sc:a[3]}));OBF.clear()}}if(false){BS=m.BS;HZP.clear();OBF.clear()}else if(m.BS!==BS){BS=m.BS;HZP.clear();OBF.clear()}
 const old=NG.map,nm=new Map();const keep=(o,ks)=>{const q=o.id!==undefined?old.get(o.id):null;if(q){for(const k of ks)if(q[k]!==undefined)o[k]=q[k];o._ox=q._rx??q.x;o._oy=q._ry??q.y}else{o._ox=o.x;o._oy=o.y}o._tx=o.x;o._ty=o.y;if(o.id!==undefined)nm.set(o.id,o)};
 for(const id of m.DD||[]){const q=old.get(id);if(q&&q._dv)try{addDie(q)}catch(e){}}
 m.E=m.E.map(deE);for(const e of m.E)keep(e,['_ph','_fx','_bv','_dv','_sc']);for(const p of m.PR)keep(p,[]);NG.map=nm;
 E=m.E;PR=m.PR;FX=m.FX;ZN=m.ZN;EB=m.EB;HZ=m.HZ;CH=m.CH;CY=m.CY;MC=m.MC;IT=m.IT;G=m.G.map(a=>({x:a[0],y:a[1],v:a[2]===2?5:a[2],c:a[3]?(a[3]):0,tk:a[4]}));gust=m.gu;kills=m.kl;endless=m.en;
 PCN=m.PC;COBAN=m.BN;if(COBAN&&!NG.bn){NG.bn=1}if(!COBAN)NG.bn=0;
 for(let j=0;j<m.P.length;j++){const s=m.P[j],c=PL[j];if(!c)continue;const p=c.P;
  if(j===LOCAL){p.hx=s.x;p.hy=s.y;if(p.x===0&&p.y===0||s.d||Math.hypot(s.x-p.x,s.y-p.y)>160){p.x=s.x;p.y=s.y}p.lv=s.lv;p.xp=s.xp;p.nx=s.nx;p.rc=s.rc;p.tk=s.tk;p.k=s.k;for(const k of ['pm','pbl','phs','mg','bat','bel'])p[k]=s[k];
   for(const k in s.ml){if(ml[k]===undefined)mt[k]=0}for(const k in ml)if(s.ml[k]===undefined)delete ml[k];Object.assign(ml,s.ml);for(const k in evo)if(!s.evo[k])delete evo[k];Object.assign(evo,s.evo);Object.assign(pl,s.pl)}
  else{p._ox=p._rx??s.x;p._oy=p._ry??s.y;p._tx=s.x;p._ty=s.y;if(p.x===0&&p.y===0){p.x=s.x;p.y=s.y}p.fx=s.fx;p.mv=!!s.mv;c.face={x:s.f[0],y:s.f[1]};c.ml=s.ml;c.pl=s.pl}
  p.hp=s.hp;p.mhp=s.mhp;p.dead=!!s.d;c.gone=!!s.g;p.inv=s.inv?.5:0;p.st=s.st;p.webT=s.wb;p.sa=s.sa;if(s.atk<.05)p.atkT=m.T;
  c.OB=s.OB.map(a=>({x:a[0],y:a[1],t:a[2],mv:a[3],rs:a[4]}));c.CLN=s.CL.map(a=>({x:a[0],y:a[1]}));c.SH=s.SH;if(j===LOCAL){OB=c.OB;CLN=c.CLN;SH=c.SH}}
 if(!m.lvw&&NG.waitShown&&!NG.prompt){NG.waitShown=0;$('lvl').classList.remove('on')}
 if(m.lvw&&!NG.prompt&&!NG.waitShown){$('lvtitle').textContent='⏳ Waiting for the others to choose…';$('choices').innerHTML='';$('lvtools').innerHTML='';$('lvl').classList.add('on');NG.waitShown=1}}
function readInput(){let x=(keys.d||keys.arrowright?1:0)-(keys.a||keys.arrowleft?1:0),y=(keys.s||keys.arrowdown?1:0)-(keys.w||keys.arrowup?1:0);if(tc){x=mv.x;y=mv.y}else if(x&&y){x*=.707;y*=.707}if(GP.on&&!tc){const m=Math.hypot(GP.lx,GP.ly);if(m>.2){const k=Math.min(1,(m-.2)/.6)/m;x=GP.lx*k;y=GP.ly*k}}return {x,y}}
/* guest frame: predict own movement, interpolate everyone else, draw */
function netFrame(dt){useP(LOCAL);const now=performance.now(),el=Math.max(0,(now-(NG.recvT||now))/1000),a=Math.min(1.25,el*1000/NG.int);T=Math.max(T||0,(NG.snapT||0)+Math.min(.25,el));if(!isFinite(T))T=NG.snapT||0;for(const c of PL){const q=c.P;if(!isFinite(q.arate))q.arate=1;if(!isFinite(q.ani))q.ani=0;if(!isFinite(q.aniT))q.aniT=T}
 const q=readInput();if(!P.dead&&!NG.prompt&&!NG.waitShown){P.mv=!!(q.x||q.y);if(P.mv){const l=Math.hypot(q.x,q.y);face={x:q.x/l,y:q.y/l};P.x+=q.x*P.sp*dt;P.y+=q.y*P.sp*dt;if(!P.ty.includes('flying')&&!P.ty.includes('ghost'))push(P,12)}
  if(P.hx!==undefined){{const far=Math.hypot(P.hx-P.x,P.hy-P.y)>45,rt=far?16:(P.mv?2.2:6);P.x+=(P.hx-P.x)*Math.min(1,dt*rt);P.y+=(P.hy-P.y)*Math.min(1,dt*rt)}}}else if(P.hx!==undefined){P.x=P.hx;P.y=P.hy}
 if(now-NG.inT>50){NG.inT=now;chSend(NET.host&&NET.host.s,{t:'in',x:Math.round(q.x*100)/100,y:Math.round(q.y*100)/100})}
 const lerp=o=>{if(o._tx===undefined)return;o._rx=o._ox+(o._tx-o._ox)*a;o._ry=o._oy+(o._ty-o._oy)*a;o.x=o._rx;o.y=o._ry};
 for(const e of E)lerp(e);for(const p of PR)lerp(p);for(let j=0;j<PL.length;j++)if(j!==LOCAL)lerp(PL[j].P);
 shake=Math.max(0,shake-dt);draw();
 {hset('lv','t','Lv '+P.lv);hudBars();hudEq();hset('cn','t','🪙'+Math.floor(P.rc||0)+(P.tk?' 🎟️'+P.tk:''));hset('tm','t',(endless?'ENDLESS ':'')+BIO[BI]+' · '+fmt(T)+(endless?'':' / '+fmt(RUN.len))+(now-NG.recvT>1500?' · ⏸ waiting for host':''))}}
function guestFinish(m,note){const S=m&&m.stand;NETG=0;COOP=0;useP(LOCAL);const me=m&&m.me;if(me){P.rc=me.rc;P.tk=me.tk;P.lv=me.lv;P.ek=me.ek;P.bk=me.bk;T=me.T;DMG=me.DMG||{};P.mbk=me.mbk||0;P.tdk=me.tdk||0;P.ccn=me.ccn||0;P.pcn=me.pcn||0;CORES={pl:me.pl||2,rv:me.rv||0,mvp:!!me.mvp,ab:me.ab}}kills=me?me.k:(P.k||0);P.hp=1;KB=null;
 if(LOB)LOB.state='lobby';$('lvl').classList.remove('on');finish(me?me.win:!!won);try{const box=$('kb');box.innerHTML=(note?`<p style="color:#ffb3b3">${note}</p>`:'')+(S?standHTML(S):'');$('et').textContent=S&&S.win?'Victory! Co-op run over':S&&S.rows.every(r=>!r.alive||r.gone)?'Team knocked out':'Run ended'}catch(e){}}
/* =================== lobby UI + invites =================== */
async function leaveLobby(quiet){for(const u of NET.unsubs)try{u()}catch(e){}NET.unsubs=[];
 if(NETH&&LOB){for(const g in NET.peers){chSend(NET.peers[g].c,{t:'bye'});try{NET.peers[g].pc.close()}catch(e){}}NET.peers={};try{await FB.collection('lobbies').doc(LOB.code).delete()}catch(e){}}
 if(NET.host){chSend(NET.host.c,{t:'bye'});try{NET.host.pc.close()}catch(e){}try{await FB.collection('lobbies').doc(NET.host.code).collection('join').doc(FBU.uid).delete()}catch(e){}NET.host=null}
 NETH=0;NETG=0;LOB=null;if(!quiet)lobUI()}
function lobPick(){LOBRET=1;show('chars')}
function lobUI(){const el=$('lobb');if(!el)return;if(!FBU){el.innerHTML='<p>Sign in with Google on the main menu to play online co-op.</p>';return}
 const nm=k=>CHR[k]?CHR[k].n:(starters.find(a=>a[0]===k)||[0,k])[1];
 if(!LOB){el.innerHTML=`<p style="max-width:460px">Play together with up to 3 friends. Everyone shares XP, level-ups wait for all players, and fallen teammates can be revived at a 🏥 Pokémon Center.</p>
  <button class="big" onclick="hostLobby()">👑 Host a game</button><div style="display:flex;gap:8px;justify-content:center;flex-wrap:wrap;margin-top:8px"><input id="lobcode" class="srch" style="width:150px;text-transform:uppercase;letter-spacing:3px;text-align:center" maxlength="5" placeholder="CODE" autocomplete="off"><button onclick="joinLobby($('lobcode').value)">Join</button></div>`;return}
 const host=LOB.host===FBU.uid,me=LOB.players.find(p=>p.uid===FBU.uid);
 let h=`<div class="selcard" style="align-items:center"><small>Room code</small><b style="font-size:28px;letter-spacing:5px;color:var(--acc)">${LOB.code}</b><small>${host?'Friends can join with this code or an invite':'Host: '+(LOB.hostName||'')}</small></div><div class="lobp">`;
 for(let i=0;i<4;i++){const p=LOB.players[i];h+=p?`<div class="lobr" style="border-color:${PCOL[i]}"><b style="color:${PCOL[i]}">${p.uid===LOB.host?'👑 ':''}${(p.name+'').replace(/[<>&]/g,'')}${p.uid===FBU.uid?' (you)':''}</b><span>${nm(p.uid===FBU.uid?SEL:p.cid)}</span><small>${p.dup?'⚠ Same Pokémon as another player':p.uid===LOB.host?'Host':p.ready?'✅ Ready':'⏳ Not ready'}</small></div>`:`<div class="lobr empty"><small>Open slot</small></div>`}
 h+=`</div><div class="lobset"><span>⚙ ${host?runLabel():(LOB.rl||'Host\'s run setup')}</span>${host?'<button class="sm" onclick="LOBRET=1;show(\'setup\')">Change</button>':''}</div><div style="display:flex;gap:8px;flex-wrap:wrap;justify-content:center"><button onclick="lobPick()">🐾 Change Pokémon</button>`;
 if(host)h+=`<button class="big" onclick="hostStart()" ${LOB.players.length<2?'disabled style="opacity:.5"':''}>▶ Start (${LOB.players.length}/4)</button>`;else if(me)h+=`<button class="big" onclick="guestReady()" ${me.dup?'disabled style="opacity:.5"':''}>${me.dup?'Pick another Pokémon':me.ready?'✋ Not ready':'✅ Ready'}</button>`;
 h+=`<button onclick="leaveLobby()">🚪 Leave</button></div>`;
 if(host){const fr=Object.keys(FRS).filter(id=>FRS[id].status==='friends'&&!LOB.players.some(p=>p.uid===id));h+=`<h3>Invite friends</h3><div style="width:min(460px,100%);display:flex;flex-direction:column;gap:6px">${fr.length?fr.map(id=>{const p=FRPROF[id]||{},on=frOnline(p);return `<div class="frr"><span class="frn"><i class="dot${on?' on':''}"></i><b>${(p.name||FRS[id].name||'Trainer').replace(/[<>&]/g,'')}</b><small>${on?' Online':' Offline'}</small></span><button class="sm" onclick="lobInvite('${id}',this)">📨 Invite</button></div>`}).join(''):'<small style="opacity:.7">Add friends in 👥 Friends to invite them.</small>'}</div><small style="opacity:.7">The run uses your ⚙ Run setup. Each player earns their own coins and mastery.</small>`}
 el.innerHTML=h}
function guestReady(){const me=LOB&&LOB.players.find(p=>p.uid===FBU.uid);if(!me||!NET.host)return;chSend(NET.host.c,{t:'ready',r:me.ready?0:1})}
async function lobInvite(fid,b){if(!LOB)return;try{b.disabled=true;await FB.collection('users').doc(fid).collection('invites').doc(FBU.uid).set({code:LOB.code,name:myName(),ts:Date.now()});b.textContent='✅ Sent'}catch(e){b.textContent='❌';lobMsg('Invite failed ('+(e.code||e.message)+')',1)}}
function invStart(u){if(INVUNSUB)try{INVUNSUB()}catch(e){}INVUNSUB=FB.collection('users').doc(u.uid).collection('invites').onSnapshot(sn=>{sn.docChanges().forEach(ch=>{if(ch.type==='removed')return;const d=ch.doc.data();if(!d||Date.now()-(d.ts||0)>180000||INVSEEN[ch.doc.id+d.ts])return;INVSEEN[ch.doc.id+d.ts]=1;invShow(ch.doc.id,d)})},()=>{})}
function invShow(from,d){if(state==='play'||state==='lvl'){return}const el=$('invpop');el.innerHTML='';const t=document.createElement('div');t.innerHTML=`<b>🤝 ${(d.name||'A friend').replace(/[<>&]/g,'')}</b> invited you to co-op!`;const j=document.createElement('button');j.textContent='Join';j.className='sm frok';const n=document.createElement('button');n.textContent='Decline';n.className='sm';
 const done=()=>{el.classList.remove('on');FB.collection('users').doc(FBU.uid).collection('invites').doc(from).delete().catch(()=>{})};j.onclick=()=>{done();show('lobby');joinLobby(d.code)};n.onclick=done;el.append(t,j,n);el.classList.add('on');setTimeout(()=>{if(el.firstChild===t)el.classList.remove('on')},60000)}

/* heartbeat: both sides ping every second; 10s of silence = disconnected (WebRTC alone can take 30s+ to notice) */
setInterval(()=>{const now=performance.now();
 if(NETH){for(const g in NET.peers){const pe_=NET.peers[g];if(pe_.c&&pe_.c.readyState==='open')chSend(pe_.c,{t:'hb'});if(now-pe_.lr>10000)peerGone(g)}}
 if(NET.host){if(NET.host.c.readyState==='open')chSend(NET.host.c,{t:'hb'});if(now-NET.host.lr>10000)hostGone()}},1000);
addEventListener('pagehide',()=>{try{if(NETH)for(const g in NET.peers)chSend(NET.peers[g].c,{t:'bye'});if(NET.host)chSend(NET.host.c,{t:'bye'})}catch(e){}});

/* ---- pause sync, character lock, leaving a run ---- */
let NETSUP=0;
function netBcast(o,skip){for(const g in NET.peers){if(g===skip)continue;const pe_=NET.peers[g];if(pe_.pi!=null)chSend(pe_.c,o)}}
function pauseBy(n){try{$('pause').querySelector('h1').textContent='⏸ Paused by '+n}catch(e){}}
window.onPauseNet=on=>{if(NETSUP)return;try{$('pause').querySelector('h1').textContent=on?'⏸ Paused':'Paused'}catch(e){}
 if(NETH&&COOP)netBcast(on?{t:'pause',by:myName()}:{t:'resume'});else if(NETG&&NET.host)chSend(NET.host.c,{t:on?'pause':'resume'})};
function guestQuit(){const h=NET.host;NET.host=null;if(h){chSend(h.c,{t:'bye'});setTimeout(()=>{try{h.pc.close()}catch(e){}},300)}$('pause').classList.remove('on');state='play';NETG=0;guestFinish(null,'You left the co-op run · your rewards were banked');LOB=null}
function lobReturn(){if(!LOB)return;if(NET.host){chSend(NET.host.c,{t:'pick',cid:SEL,prof:myProf(SEL)})}if(NETH){const tk=lobTaken(SEL,FBU.uid);if(tk){lobMsg('❌ '+tk.name+' is already using '+cnm(SEL)+'. Pick a different Pokémon.',1);SEL=LOB.players[0].cid}else LOB.players[0].cid=SEL;lobBroadcast()}}
window.CHTAKEN=(d,t)=>{if(!LOB||LOB.state!=='lobby'||!FBU)return;const tk=LOB.players.find(p=>p.uid!==FBU.uid&&p.cid===t);if(!tk)return;d.style.opacity=.5;const m=document.createElement('small');m.style.color='#ff8a8a';m.textContent='🔒 '+tk.name+' is using this Pokémon';d.appendChild(m);d.onclick=null};

/* boss rewards reach every guest's own save */
window.COBOSS=ev=>{if(NETH)netBcast({t:'boss',ev})};

/* guest side of the route vote */
function guestRouteUI(){const R=NG&&NG.route;if(!R)return;routeUI(R,LOCAL,ix=>{if(R.votes[LOCAL]!==undefined)return;R.votes[LOCAL]=ix;chSend(NET.host&&NET.host.c,{t:'rvote',ix});guestRouteUI()})}
