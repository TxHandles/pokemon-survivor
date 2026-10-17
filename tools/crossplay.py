import asyncio,subprocess,time,sys,os
from playwright.async_api import async_playwright
# a browser guest joining a room hosted by the native netpeer through the test relay
S=os.path.dirname(os.path.abspath(__file__))
code=sys.argv[1];secs=int(sys.argv[2])
srv=subprocess.Popen(['python3','-I',S+'/sigserver.py','8773'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);time.sleep(1)
peer=subprocess.Popen([os.environ.get('NETPEER',S+'/../netpeer'),'-host',code,'-signal','http://localhost:8773','-secs',str(secs)],stdout=open(S+'/peer_host.log','w'),stderr=subprocess.STDOUT);time.sleep(2)
logs=[]
async def main():
  async with async_playwright() as p:
    b=await p.chromium.launch(args=['--no-sandbox','--use-gl=angle','--use-angle=swiftshader','--enable-unsafe-swiftshader'])
    g=await b.new_page(viewport={'width':400,'height':225});g.on('console',lambda m:logs.append('G '+m.text) if 'gstatic' not in m.text and 'ERR_' not in m.text else None)
    await g.goto('http://localhost:8773/index.html?signal=http&join='+code+'&auto=1&nodraw=1')
    for i in range(secs*2+30):
        await g.wait_for_timeout(1000)
        if peer.poll() is not None: break
    await g.screenshot(path=S+'/go_cross_guest.png',timeout=120000)
    await b.close()
asyncio.run(main())
print('\n'.join(logs[-12:]))
print('--- native host:');print(open(S+'/peer_host.log').read()[-1500:])
peer.terminate();srv.terminate()
