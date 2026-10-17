import asyncio,subprocess,time,sys,os
from playwright.async_api import async_playwright
# host and guest in two separate browsers over real WebRTC, signalled through the test server;
# both auto-ready / auto-start and their local player is bot-driven (?auto=1)
out=sys.argv[1] if len(sys.argv)>1 else 'go_net'
W=int(sys.argv[3]) if len(sys.argv)>3 else 960;H=W*9//16
secs=int(sys.argv[2]) if len(sys.argv)>2 else 40
logs=[]
S=os.path.dirname(os.path.abspath(__file__))
srv=subprocess.Popen(['python3','-I',S+'/sigserver.py','8773'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);time.sleep(1)
async def main():
  async with async_playwright() as p:
    args=['--no-sandbox','--use-gl=angle','--use-angle=swiftshader','--enable-unsafe-swiftshader']
    bh=await p.chromium.launch(args=args);bg=await p.chromium.launch(args=args)
    keep=lambda t:'gstatic' not in t and 'ERR_' not in t and 'net: message' not in t
    h=await bh.new_page(viewport={'width':W,'height':H});h.on('console',lambda m:logs.append('H '+m.text) if keep(m.text) else None)
    await h.goto('http://localhost:8773/index.html?signal=http&host=TEST1&auto=1&nodraw=1');await h.wait_for_timeout(4000)
    g=await bg.new_page(viewport={'width':1280,'height':560})  # a different window shape than the host's;g.on('console',lambda m:logs.append('G '+m.text) if keep(m.text) else None)
    await g.goto('http://localhost:8773/index.html?signal=http&join=TEST1&auto=1&nodraw=1');await g.wait_for_timeout(8000)
    await h.screenshot(path=out+'_start_host.png',timeout=120000);await g.screenshot(path=out+'_start_guest.png',timeout=120000)
    await h.wait_for_timeout(secs*1000)
    await asyncio.gather(h.screenshot(path=out+'_play_host.png',timeout=120000),g.screenshot(path=out+'_play_guest.png',timeout=120000))
    if len(sys.argv)>4 and sys.argv[4]=='drop':
        await bg.close();print('guest closed')
        await h.wait_for_timeout(secs*1000);await h.screenshot(path=out+'_drop_host.png',timeout=120000)
        await bh.close();srv.terminate();return
    await h.wait_for_timeout(secs*1000)
    await asyncio.gather(h.screenshot(path=out+'_play2_host.png',timeout=120000),g.screenshot(path=out+'_play2_guest.png',timeout=120000))
    await bh.close();await bg.close()
  srv.terminate()
try:
    asyncio.run(main())
except Exception as e:
    print('ERR',e)
print('\n'.join(logs[-60:]))
