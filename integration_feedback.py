import json,os,sys,subprocess,threading,queue,time,tempfile,pathlib,http.server,hashlib,struct
ROOT=pathlib.Path(__file__).resolve().parent
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*a):pass
 def do_GET(self):
  if self.path.startswith('/frame'):
   content=b'<button id="inframe" onclick="this.textContent=\'Frame done\'">Frame button</button><input type=file id=framefile>'
  elif self.path.startswith('/download'):
   self.send_response(200);self.send_header('Content-Disposition','attachment; filename=fixture.txt');self.end_headers();self.wfile.write(b'fixture-download');return
  else:
   content=b'''<!doctype html><html><head><title>Zero Ego QA</title></head><body><h1>Fixture</h1>
<button id=dead>Dead</button><button id=toggle onclick="document.getElementById('status').textContent='Changed'">Toggle</button><span id=status>Initial</span>
<input type=checkbox id=checkbox><input id=paste><button id=prompt onclick="document.getElementById('status').textContent=prompt('answer','')">Prompt</button>
<button class=duplicate onclick="this.dataset.clicked='true'">Same</button><button class=duplicate onclick="this.dataset.clicked='true'">Same</button>
<button id=choose onclick="let f=document.createElement('input');f.type='file';f.id='dynamic';document.body.appendChild(f);f.click()">Choose</button>
<input id=upload type=file><a id=dl href=/download>Download</a><iframe name=qa src=/frame></iframe>
<div id=shadow></div><div style="height:1800px">tall page</div><button id=bottom>Bottom</button>
<script>document.getElementById('paste').addEventListener('paste',e=>e.preventDefault());document.getElementById('shadow').attachShadow({mode:'open'}).innerHTML='<button id="shadowbutton">Shadow</button><input id="shadowfile" type="file">';</script></body></html>'''
  self.send_response(200);self.send_header('Content-Type','text/html');self.end_headers();self.wfile.write(content)
 def do_POST(self):
  body=self.rfile.read(int(self.headers.get('Content-Length','0')))
  self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(json.dumps({'body':body.decode(),'contentType':self.headers.get('Content-Type')}).encode())
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start()
port=server.server_address[1];url=f'http://127.0.0.1:{port}/'
outdir=ROOT/'.gotmp'/'live-feedback';outdir.mkdir(parents=True,exist_ok=True);empty=outdir/'empty.txt';empty.write_bytes(b'')
exe=sys.argv[1]
p=subprocess.Popen([exe],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(outdir/'connector.stderr.log').open('wb'),text=True,encoding='utf-8',bufsize=1)
q=queue.Queue()
def reader():
 for line in p.stdout:q.put(line)
threading.Thread(target=reader,daemon=True).start();seq=0;results=[];started=False

def rpc(method,params):
 global seq
 seq+=1;p.stdin.write(json.dumps({'jsonrpc':'2.0','id':seq,'method':method,'params':params},ensure_ascii=False)+'\n');p.stdin.flush()
 while True:
  line=q.get(timeout=120);data=json.loads(line)
  if data.get('id')==seq:return data

def call(name,args={},error=False):
 response=rpc('tools/call',{'name':'ego_lite_'+name,'arguments':args})['result']
 text=response['content'][0]['text'];value=text if response.get('isError') else json.loads(text)
 if response.get('isError') and not error:raise RuntimeError(f'{name}: {value}')
 return value

def check(name,fn):
 try:
  detail=fn();results.append({'test':name,'passed':True,'detail':detail});print('PASS',name,flush=True)
 except Exception as e:
  results.append({'test':name,'passed':False,'error':str(e)});print('FAIL',name,str(e),flush=True)

def expect(value,expected):
 assert value==expected,(value,expected);return value
try:
 rpc('initialize',{'protocolVersion':'2025-06-18'})
 check('spaces-before-start',lambda:call('spaces'))
 start=call('start');started=True;results.append({'test':'start','passed':bool(start.get('spaceId')),'detail':start})
 check('navigate-brief',lambda:expect(call('navigate',{'url':url})['brief'],True))
 check('explicit-snapshot-has-tree',lambda: bool(call('snapshot').get('snapshot')) or (_ for _ in ()).throw(AssertionError('no tree')))
 for expr,want in [('new Promise(r=>r(8))',8),('[1,2].map(x=>x+1)',[2,3]),('"=>"','=>'),('async function(){return 4}',4),('(()=>5)()',5)]:
  check('evaluate:'+expr,lambda expr=expr,want=want:expect(call('evaluate',{'expression':expr}),want))
 check('snapshot-brief-preserved',lambda:expect(call('click',{'selector':'#dead'})['brief'],True))
 check('no-change-dead',lambda:expect(call('click',{'selector':'#dead'})['noVisibleChange'],True))
 check('no-change-real-dom',lambda:expect(call('click',{'selector':'#toggle'})['noVisibleChange'],False))
 check('no-change-checkbox',lambda:expect(call('click',{'selector':'#checkbox'})['noVisibleChange'],False))
 check('nth-filter',lambda:call('click',{'selector':'.duplicate','nth':1}))
 check('nth-target-confirm',lambda:expect(call('evaluate',{'expression':"[...document.querySelectorAll('.duplicate')].map(x=>x.dataset.clicked||'')"}),['','true']))
 check('box-scroll',lambda:call('box',{'selector':'#bottom'}))
 check('box-shadow',lambda:call('box',{'selector':'#shadowbutton'}))
 check('upload-native',lambda:call('upload',{'selector':'#upload','paths':str(empty)}))
 check('upload-shadow',lambda:call('upload',{'selector':'#shadowfile','paths':str(empty)}))
 check('dynamic-filechooser',lambda:call('filechooser',{'trigger':'#choose','paths':str(empty),'timeoutMs':8000}))
 check('insert-text',lambda: (call('focus',{'selector':'#paste'}),call('keyboard',{'action':'insert','text':'中文 insert'}),expect(call('evaluate',{'expression':"document.querySelector('#paste').value"}),'中文 insert'))[-1])
 check('dialog-trigger',lambda:expect(call('dialog',{'trigger':'#prompt','action':'accept','text':'answer','timeoutMs':5000})['handled'],True))
 check('fetch-JSON-preserved',lambda:expect(call('fetch',{'url':'/echo','method':'POST','body':'{"x":1}','headers':{'content-type':'application/json'}})['json']['body'],'{"x":1}'))
 frames=call('frames');child=next((x for x in frames if not x['main']),None)
 check('frame-evaluate',lambda:expect(call('evaluate',{'expression':'document.querySelector("#inframe").textContent','frame':child['id']}),'Frame button'))
 check('frame-click',lambda:call('click',{'selector':'#inframe','frame':child['id']}))
 check('frame-evaluate-after',lambda:expect(call('evaluate',{'expression':'document.querySelector("#inframe").textContent','frame':child['id']}),'Frame done'))
 check('download',lambda:call('download',{'selector':'#dl','saveAs':str(outdir/'download.txt')}))
 check('download-content',lambda:expect((outdir/'download.txt').read_text(),'fixture-download'))
 check('screenshot-clip-scale',lambda:call('screenshot',{'clip':{'x':0,'y':0,'width':200,'height':100},'scale':0.5,'saveAs':str(outdir/'crop.png')}))
 check('screenshot-fullPage',lambda:call('screenshot',{'fullPage':True,'saveAs':str(outdir/'full.png')}))
 check('screenshot-selector',lambda:call('screenshot',{'selector':'#toggle','saveAs':str(outdir/'selector.png')}))
 check('compact-data-url',lambda:expect(call('evaluate',{'expression':"'data:image/png;base64,'+'A'.repeat(20000)"})['truncated'],True))
 check('macro',lambda:expect(call('do',{'steps':[{'tool':'ego_lite_fill','arguments':{'selector':'#paste','text':'macro'}},{'tool':'ego_lite_evaluate','arguments':{'expression':"document.querySelector('#paste').value"}}]})['count'],2))
 # Simulate external close of current page and stale local pointer, preserving p1.
 new=call('new_page');check('new-page',lambda:new)
 check('run-closes-active',lambda:call('run',{'script':'await page.close();return {closed:true};'}))
 check('stale-pointer-recovery',lambda:expect(call('navigate',{'url':url})['page'],'p1'))
 local=outdir/'local file.html';local.write_text('<html><title>Local QA</title><h1>Local</h1></html>',encoding='utf-8')
 check('open-local',lambda:call('open_local',{'path':str(local)}))
 check('open-local-title',lambda:expect(call('evaluate',{'expression':'document.title'}),'Local QA'))
finally:
 if started:
  try:call('finish')
  except Exception as e:results.append({'test':'finish','passed':False,'error':str(e)})
 server.shutdown();p.stdin.close();p.wait(timeout=15)
 report={'version':'1.6.0','exe':exe,'passed':sum(x['passed'] for x in results),'total':len(results),'results':results}
 (outdir/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
 print(json.dumps({'passed':report['passed'],'total':report['total'],'report':str(outdir/'report.json')}),flush=True)
 sys.exit(0 if report['passed']==report['total'] else 1)
