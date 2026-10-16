// ---- Friends (web build) ----
let FRS={},FRUNSUB=null,FRPROF={},FRSEEN=null,FRCODE='',FRBUSY=false;
const FRAL='ABCDEFGHJKMNPQRSTUVWXYZ23456789';
const frName=()=>String(opt('name')||(FBU&&FBU.displayName||'').split(' ')[0]||'Trainer').slice(0,16);
const frOnline=p=>p&&p.seen&&Date.now()-p.seen<150000;
async function frStart(u){frStop();try{
 const me=FB.collection('users').doc(u.uid);let p=(await me.get()).data()||{};
 if(!p.code){for(let t=0;t<6&&!p.code;t++){let c='';for(let i=0;i<6;i++)c+=FRAL[Math.floor(Math.random()*FRAL.length)];
   try{await FB.collection('codes').doc(c).set({uid:u.uid});p.code=c}catch(e){}}}
 FRCODE=p.code||'';await me.set({name:frName(),code:FRCODE,seen:Date.now()},{merge:true});
 FRSEEN=setInterval(()=>{if(!document.hidden&&FBU)me.set({seen:Date.now(),name:frName()},{merge:true}).catch(()=>{})},60000);
 FRUNSUB=me.collection('friends').onSnapshot(sn=>{FRS={};sn.forEach(d=>FRS[d.id]=d.data());frBadge();if($('friends').classList.contains('on'))frUI();try{if($('lobby')&&$('lobby').classList.contains('on'))frProfiles().then(lobUI)}catch(e){}},()=>{});
 frBadge()}catch(e){console.warn('friends',e)}}
function frStop(){if(FRUNSUB){try{FRUNSUB()}catch(e){}FRUNSUB=null}if(FRSEEN){clearInterval(FRSEEN);FRSEEN=null}FRS={};FRCODE=''}
function frBadge(){const b=$('frb');if(!b)return;const n=Object.values(FRS).filter(f=>f.status==='in').length;b.textContent='👥 Friends'+(n?' ('+n+')':'')}
async function frProfiles(){const ids=Object.keys(FRS);await Promise.all(ids.map(async id=>{try{FRPROF[id]=(await FB.collection('users').doc(id).get()).data()||{}}catch(e){}}))}
async function frAdd(){const inp=$('frcode'),msg=$('frmsg'),c=String(inp.value||'').toUpperCase().replace(/[^A-Z0-9]/g,'');msg.textContent='';
 if(c.length!==6)return msg.textContent='Friend codes are 6 characters.';if(c===FRCODE)return msg.textContent='That\'s your own code!';if(FRBUSY)return;FRBUSY=true;
 try{const s=await FB.collection('codes').doc(c).get();if(!s.exists){msg.textContent='❌ No trainer has that code.';return}
  const fid=s.data().uid;if(FRS[fid]&&FRS[fid].status==='friends'){msg.textContent='You\'re already friends.';return}
  if(FRS[fid]&&FRS[fid].status==='in'){await frAccept(fid);msg.textContent='✅ They had already asked, you\'re now friends!';inp.value='';return}
  const fp=(await FB.collection('users').doc(fid).get()).data()||{};const now=Date.now();
  await FB.collection('users').doc(fid).collection('friends').doc(FBU.uid).set({status:'in',name:frName(),ts:now});
  await FB.collection('users').doc(FBU.uid).collection('friends').doc(fid).set({status:'out',name:String(fp.name||'Trainer').slice(0,16),ts:now});
  msg.textContent='✅ Request sent to '+(fp.name||'that trainer')+'!';inp.value=''}
 catch(e){msg.textContent='❌ Could not send request ('+(e.code||e.message)+')'}finally{FRBUSY=false}}
async function frAccept(fid){const now=Date.now(),f=FRS[fid]||{};
 await FB.collection('users').doc(fid).collection('friends').doc(FBU.uid).update({status:'friends',name:frName(),ts:now});
 await FB.collection('users').doc(FBU.uid).collection('friends').doc(fid).set({status:'friends',name:f.name||'Trainer',ts:now})}
async function frRemove(fid){try{await FB.collection('users').doc(FBU.uid).collection('friends').doc(fid).delete()}catch(e){}try{await FB.collection('users').doc(fid).collection('friends').doc(FBU.uid).delete()}catch(e){}}
async function frUI(){const L=$('frl');if(!FBU){L.innerHTML='<p>Sign in with Google to add friends.</p>';$('frmine').textContent='—';return}
 $('frmine').textContent=FRCODE||'…';await frProfiles();L.innerHTML='';
 const groups=[['in','Friend requests'],['friends','Friends'],['out','Sent requests']];let any=false;
 for(const [st,title] of groups){const ids=Object.keys(FRS).filter(id=>FRS[id].status===st);if(!ids.length)continue;any=true;
  if(st==='friends')ids.sort((a,b)=>(frOnline(FRPROF[b])?1:0)-(frOnline(FRPROF[a])?1:0));
  const h=document.createElement('h3');h.textContent=title+' ('+ids.length+')';L.appendChild(h);
  for(const id of ids){const p=FRPROF[id]||{},r=document.createElement('div');r.className='frr';
   const nm=document.createElement('span');nm.className='frn';const on=frOnline(p);
   nm.innerHTML='<i class="dot'+(st==='friends'&&on?' on':'')+'"></i>';const t=document.createElement('b');t.textContent=p.name||FRS[id].name||'Trainer';nm.appendChild(t);
   const sub=document.createElement('small');sub.textContent=st==='friends'?(on?' Online':(p.seen?' Last seen '+frAgo(p.seen):' Offline')):st==='in'?' wants to be friends':' pending…';nm.appendChild(sub);r.appendChild(nm);
   const bx=document.createElement('span');const mk=(lbl,fn,cls)=>{const b=document.createElement('button');b.className='sm'+(cls?' '+cls:'');b.textContent=lbl;b.onclick=async()=>{b.disabled=true;try{await fn()}catch(e){$('frmsg').textContent='❌ '+(e.code||e.message)}};bx.appendChild(b)};
   if(st==='in'){mk('Accept',()=>frAccept(id),'frok');mk('Decline',()=>frRemove(id))}
   else if(st==='out')mk('Cancel',()=>frRemove(id));
   else{const rm=()=>{const b=bx.querySelector('.frrm');if(b.dataset.s)return frRemove(id);b.dataset.s=1;b.textContent='Sure?';b.disabled=false};mk('Remove',rm,'frrm')}
   r.appendChild(bx);L.appendChild(r)}}
 if(!any)L.innerHTML='<p style="opacity:.7">No friends yet. Share your code, or enter a friend\'s code above.</p>'}
const frAgo=t=>{const m=Math.round((Date.now()-t)/60000);return m<60?m+'m ago':m<1440?Math.round(m/60)+'h ago':Math.round(m/1440)+'d ago'};
async function frCopy(){const m=$('frmsg');try{await navigator.clipboard.writeText(FRCODE);m.textContent='📋 Code copied!'}catch(e){m.textContent='Your code: '+FRCODE}}
