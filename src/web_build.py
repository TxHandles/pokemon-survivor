# Website build: adds Firebase sign-in, cloud saves, friends and online co-op to the single-file game.
# Run through tools/build.py.
import sys,re
import os
HERE=os.path.dirname(os.path.abspath(__file__))
src=sys.argv[1]
out=sys.argv[2]
s=open(src).read()
def rep(a,b,n=1):
    global s
    c=s.count(a)
    assert c==n,(a[:70],c)
    s=s.replace(a,b)

FBV='10.12.2'
head=''.join(f'<script src="https://www.gstatic.com/firebasejs/{FBV}/firebase-{m}-compat.js"></script>\n' for m in ['app','auth','firestore'])
rep('<title>Pokemon Survivor</title>\n','<title>Pokemon Survivor</title>\n'+head)

# account bar on the main menu
rep('<div class="cur"><span>🪙 <b id="coins">0</b></span>','<div id="acct" class="acct"></div>\n<div class="cur"><span>🪙 <b id="coins">0</b></span>')
rep('.ov.on{display:flex}','.ov.on{display:flex}\n.acct{display:flex;gap:8px;align-items:center;justify-content:center;font-size:13px;min-height:30px}.acct .gbtn{background:#fff;color:#222;border-color:#fff;font-weight:700;display:inline-flex;gap:6px;align-items:center}.acct .gbtn b{color:#4285f4;font-size:15px}')

# wording
rep("setCS('\\u2601 Fresh start saved to your Claude account')","setCS('\\u2601 Fresh start saved to your account')")
rep("setCS('\\u2601 Synced to your Claude account')","setCS('\\u2601 Synced to your account')",2)
rep("(global boards need your Claude account)","(sign in with Google to see global boards)")

# replace initCloud with a Firebase version
i=s.find('async function initCloud(){');j=s.find('\nlet SEL=',i)
assert i>0 and j>i
web=r'''
const FBC={apiKey:"AIzaSyArpClEYGkFOq552fSgMYEwRIQlcJs5NJU",authDomain:"pokemon-survivors-e8213.firebaseapp.com",projectId:"pokemon-survivors-e8213",storageBucket:"pokemon-survivors-e8213.firebasestorage.app",messagingSenderId:"718817782433",appId:"1:718817782433:web:1803f4b323b3e78cc71ee0"};
let FB=null,FBU=null,FBUNSUB=null,FBVIS=0;
const fbEnc=d=>{const o={j:JSON.stringify(d)};for(const k in d){const v=d[k];if(v!==null&&v!==undefined&&typeof v!=='object')o[k]=v}return o};
const fbSnap=s=>({exists:s.exists,id:s.id,raw:s.exists?s.data():null,data(){if(!s.exists)return null;const v=s.data();try{return typeof v.j==='string'?JSON.parse(v.j):v}catch(e){return v}}});
const fbDoc=ref=>({id:ref.id,get:async()=>fbSnap(await ref.get()),set:d=>ref.set(fbEnc(d)),onSnapshot:(f,e)=>ref.onSnapshot(s=>f(fbSnap(s)),e||(()=>{}))});
const fbQ=(col,w,o,l)=>({where:(f,op,v)=>fbQ(col,[...w,[f,op,v]],o,l),orderBy:(f,d)=>fbQ(col,w,[f,d||'asc'],l),limit:n=>fbQ(col,w,o,n),
 get:async()=>{let q=FB.collection(col);if(w.length)w.forEach(a=>q=q.where(a[0],a[1],a[2]));else{if(o)q=q.orderBy(o[0],o[1]);if(l)q=q.limit(l)}
  const sn=await q.get();let docs=sn.docs.map(fbSnap);if(w.length&&o){const m=o[1]==='desc'?-1:1;docs.sort((a,b)=>m*(((a.raw||{})[o[0]]||0)-((b.raw||{})[o[0]]||0)))}if(w.length&&l)docs=docs.slice(0,l);return{docs,size:docs.length,forEach:f=>docs.forEach(f)}}});
const FDB={doc:p=>fbDoc(FB.doc(p)),collection:c=>fbQ(c,[],null,0)};
function acctUI(){const el=$('acct');if(!el)return;el.innerHTML='';if(!window.firebase||TEST)return;
 if(FBU){const sp=document.createElement('span');sp.textContent='👤 '+(FBU.displayName||FBU.email||'Signed in');const b=document.createElement('button');b.className='sm';b.textContent='Sign out';b.onclick=fbSignOut;el.append(sp,b)}
 else{const b=document.createElement('button');b.className='sm gbtn';b.innerHTML='<b>G</b> Sign in with Google';b.onclick=fbSignIn;el.append(b)}}
async function fbSignIn(){const au=firebase.auth(),pr=new firebase.auth.GoogleAuthProvider();pr.setCustomParameters({prompt:'select_account'});
 try{await au.signInWithPopup(pr)}catch(e){const c=e&&e.code||'';if(c==='auth/popup-blocked'||c==='auth/operation-not-supported-in-this-environment'){try{await au.signInWithRedirect(pr)}catch(e2){setCS('Sign-in failed: '+(e2.code||e2.message))}}else if(c!=='auth/popup-closed-by-user'&&c!=='auth/cancelled-popup-request')setCS('Sign-in failed: '+(c||e.message))}}
function fbSignOut(){try{firebase.auth().signOut()}catch(e){}}
const fbFresh=()=>({lbl:{},fl:{},coins:0,u:{},cu:{},gu:{},gm:1,st:{k:0,b:0,t:0,r:0,w:0,mv:0,ev:0,cp:0,a:0,el:0,bi:0,bs:0},cl:{},col:{},ci:{},ms:{},rs:{tier:0,mods:[]},asc:0,hist:[],cs:{},csb:1,bu:{},pu:{},pp:{},opt:SV.opt,wv:WIPE});
async function fbUser(u){FBU=u;acctUI();if(FBUNSUB){try{FBUNSUB()}catch(e){}FBUNSUB=null}
 if(!u){CL=null;UID=null;LBDB=null;return setCS('Saved on this device · sign in with Google to sync across devices and join leaderboards')}
 UID=u.uid;LBDB=FDB;CL=FDB.doc('saves/'+u.uid);setCS('☁ Connecting…');
 try{if(SV.owner&&SV.owner!==u.uid){const s=await CL.get(),c=s.exists?s.data():null;SV=Object.assign({},SV);adopt(c&&c.wv===WIPE?c:fbFresh());SV.ts=c&&c.ts||0;SV.owner=u.uid;loc()}
  SV.owner=u.uid;if(!opt('name')&&u.displayName){SV.opt=SV.opt||{};SV.opt.name=String(u.displayName).split(' ')[0].slice(0,16)}
  await syncCloud();if(SV.lbl&&SV.lbl.dayKey)lbPush();
  FBUNSUB=CL.onSnapshot(sn=>{try{const c=sn.exists?sn.data():null;if(!c||!c.cu||c.wv!==WIPE||!((c.ts||0)>(SV.ts||0)))return;if(state==='play'||state==='lvl'||state==='pause'){CLPEND=c;return}CLPEND=null;adopt(c);try{menu()}catch(e){}setCS('☁ Updated from another device')}catch(e){}})}
 catch(e){setCS('Sync unavailable – saved on this device ('+(e.code||e.message||e)+')')}}
async function initCloud(){try{if(TEST)return setCS('TEST BUILD · separate local save, no cloud');
 try{netHooks()}catch(e){}if(!window.firebase){acctUI();return setCS('Saved on this device only (online features could not load)')}
 if(!firebase.apps.length)firebase.initializeApp(FBC);FB=firebase.firestore();const au=firebase.auth();
 au.getRedirectResult().catch(e=>setCS('Sign-in failed: '+(e.code||e.message)));au.onAuthStateChanged(u=>{fbUser(u)});
 if(!FBVIS){FBVIS=1;addEventListener('visibilitychange',()=>{if(!document.hidden&&CL&&state!=='play')syncCloud()})}}
 catch(e){acctUI();setCS('Sync unavailable – saved on this device')}}'''
s=s[:i]+web.strip()+s[j:]

# ---- friends ----
FR=open(os.path.join(HERE,'friends.js')).read()
rep("['start','chars','shopov','chal','setup','coll','hist','life','dex','daily','lbs','settings','xfer'].forEach(i=>$(i).classList.toggle('on',i===id));",
    "['start','chars','shopov','chal','setup','coll','hist','life','dex','daily','lbs','settings','xfer','friends'].forEach(i=>$(i).classList.toggle('on',i===id));if(id==='friends')frUI();")
rep('<div class="ov tall" id="xfer">','<div class="ov tall" id="friends"><div class="top"><button class="sm" onclick="show(\'start\')">← Back</button><h2>👥 Friends</h2><span></span></div>'
 '<div class="selcard" style="align-items:center"><small>Your friend code</small><b id="frmine" style="font-size:26px;letter-spacing:4px;color:var(--acc)">—</b><button class="sm" onclick="frCopy()">📋 Copy code</button></div>'
 '<div style="display:flex;gap:8px;justify-content:center;flex-wrap:wrap"><input id="frcode" class="srch" style="width:170px;text-transform:uppercase;letter-spacing:3px;text-align:center" maxlength="6" placeholder="FRIEND CODE" autocomplete="off" onkeydown="if(event.key===\'Enter\')frAdd()"><button onclick="frAdd()">➕ Add friend</button></div>'
 '<p id="frmsg" style="font-size:13px;min-height:1.2em;margin:0"></p><div id="frl" style="width:min(520px,100%);display:flex;flex-direction:column;gap:6px"></div></div>\n<div class="ov tall" id="xfer">')
rep('.acct{display:flex;','.frr{display:flex;justify-content:space-between;align-items:center;gap:8px;background:var(--panel);border:1px solid #3a4260;border-radius:10px;padding:8px 10px;text-align:left}.frr .frn{display:flex;align-items:center;gap:6px;flex-wrap:wrap}.frr small{opacity:.7}.frr .frok{border-color:#4caf50}.dot{display:inline-block;width:10px;height:10px;border-radius:50%;background:#555}.dot.on{background:#4cdf6a;box-shadow:0 0 6px #4cdf6a}\n.acct{display:flex;')
rep("const sp=document.createElement('span');sp.textContent='👤 '+(FBU.displayName||FBU.email||'Signed in');const b=document.createElement('button');b.className='sm';b.textContent='Sign out';b.onclick=fbSignOut;el.append(sp,b)",
    "const sp=document.createElement('span');sp.textContent='👤 '+(FBU.displayName||FBU.email||'Signed in');const f=document.createElement('button');f.className='sm';f.id='frb';f.onclick=()=>show('friends');const b=document.createElement('button');b.className='sm';b.textContent='Sign out';b.onclick=fbSignOut;el.append(sp,f,b);frBadge()")
rep("if(!u){CL=null;UID=null;LBDB=null;return","if(!u){frStop();CL=null;UID=null;LBDB=null;return")
rep("await syncCloud();if(SV.lbl&&SV.lbl.dayKey)lbPush();\n  FBUNSUB","await syncCloud();if(SV.lbl&&SV.lbl.dayKey)lbPush();frStart(u);\n  FBUNSUB")
rep("async function initCloud(){try{if(TEST)return setCS('TEST BUILD", FR.strip()+"\nasync function initCloud(){try{if(TEST)return setCS('TEST BUILD")


# ---- online co-op ----
NETJS=open(os.path.join(HERE,'net.js')).read()
rep("async function initCloud(){try{if(TEST)return setCS('TEST BUILD", NETJS.strip()+"\nasync function initCloud(){try{if(TEST)return setCS('TEST BUILD")
rep("try{update(dt);if(state==='play'||state==='lvl')draw()}catch(e){console.error(e)}","try{if(NETG)netFrame(dt);else{update(dt);if(state==='play'||state==='lvl')draw();if(NETH)netHostTick()}}catch(e){console.error(e)}")
rep("function choices(first){const opts=opts_();opts.sort(()=>Math.random()-.5);","function choices(first,forced){const opts=forced?forced.slice():opts_();if(!forced)opts.sort(()=>Math.random()-.5);")
rep("opts.sort((a,b)=>pr(b)-pr(a));const pick=opts.slice(0,3+ul('ch'));","if(!forced)opts.sort((a,b)=>pr(b)-pr(a));const pick=forced?opts:opts.slice(0,3+ul('ch'));")
rep("b.onclick=()=>{if(banMode){P.ban.add(","b.onclick=()=>{if(NETG){guestAnswer(i);state='play';last=performance.now();requestAnimationFrame(loop);return}if(banMode){P.ban.add(")
rep("document.getElementById('lvtitle').textContent=banMode?'Pick a choice to banish':'Level up!';if(!(ul('rr')||ul('bn')))return;","document.getElementById('lvtitle').textContent=banMode?'Pick a choice to banish':'Level up!';if(NETG||!(ul('rr')||ul('bn')))return;")
rep("if(e.hp<=0&&!e.dead){e.dead=1;addDie(e);","if(e.hp<=0&&!e.dead){e.dead=1;addDie(e);if(NETH&&e.id!==undefined)NDIE.push({id:e.id,x:e.x,y:e.y});")
rep('<button onclick="location.reload()">Play again</button></div>','<button onclick="if(LOB){$(\'end\').classList.remove(\'on\');show(\'lobby\')}else location.reload()">Play again</button></div>')
rep("['start','chars','shopov','chal','setup','coll','hist','life','dex','daily','lbs','settings','xfer','friends'].forEach(i=>$(i).classList.toggle('on',i===id));if(id==='friends')frUI();",
    "if(id==='start'&&LOBRET){LOBRET=0;id='lobby';lobReturn()}['start','chars','shopov','chal','setup','coll','hist','life','dex','daily','lbs','settings','xfer','friends','lobby'].forEach(i=>$(i).classList.toggle('on',i===id));if(id==='friends')frUI();if(id==='lobby'){frProfiles().then(lobUI);lobUI()}")
rep('<div class="ov tall" id="friends">','<div class="ov tall" id="lobby"><div class="top"><button class="sm" onclick="show(\'start\')">← Back</button><h2>🤝 Co-op</h2><span></span></div><div id="lobb" style="display:flex;flex-direction:column;align-items:center;gap:10px;width:100%"></div><p id="lobmsg" style="font-size:13px;min-height:1.2em;margin:0"></p></div>\n<div id="invpop" class="invpop"></div>\n<div class="ov tall" id="friends">')
rep('.acct{display:flex;','.lobset{display:flex;gap:8px;align-items:center;justify-content:center;flex-wrap:wrap;font-size:13px;background:var(--panel);border:1px solid #3a4260;border-radius:10px;padding:6px 10px;max-width:460px}.lobp{display:grid;grid-template-columns:1fr 1fr;gap:8px;width:min(460px,100%)}.lobr{display:flex;flex-direction:column;gap:2px;border:2px solid #3a4260;border-radius:10px;padding:8px;background:var(--panel)}.lobr.empty{opacity:.45;border-style:dashed;justify-content:center;align-items:center;min-height:62px}.invpop{position:fixed;left:50%;top:12px;transform:translateX(-50%);z-index:60;background:#1b2030;border:2px solid var(--acc);border-radius:12px;padding:10px 14px;display:none;gap:8px;align-items:center;box-shadow:0 6px 24px #000a;flex-wrap:wrap;justify-content:center;max-width:92vw}.invpop.on{display:flex}\n.acct{display:flex;')
rep("f.className='sm';f.id='frb';f.onclick=()=>show('friends');","f.className='sm';f.id='frb';f.onclick=()=>show('friends');const co=document.createElement('button');co.className='sm';co.textContent='🤝 Co-op';co.onclick=()=>show('lobby');")
rep("el.append(sp,f,b);frBadge()","el.append(sp,f,co,b);frBadge()")
rep("if(!u){frStop();CL=null;","if(!u){frStop();try{leaveLobby(true);if(INVUNSUB)INVUNSUB()}catch(e){}CL=null;")
rep("frStart(u);\n  FBUNSUB","frStart(u);invStart(u);\n  FBUNSUB")


rep('onclick="if(state===\'pause\'){resumeGame();finish(won)}"','onclick="if(NETG){guestQuit()}else if(state===\'pause\'){resumeGame();finish(won)}"')

# own-save marker travels with the save
rep("const pack=()=>{SV.sel=SEL;return","const pack=()=>{SV.sel=SEL;if(UID)SV.owner=UID;return")
open(out,'w').write(s)
print('wrote',out,len(s))
