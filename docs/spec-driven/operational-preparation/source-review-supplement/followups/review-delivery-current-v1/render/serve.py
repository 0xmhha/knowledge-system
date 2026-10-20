import json,os,time
from pathlib import Path
from http.server import HTTPServer,BaseHTTPRequestHandler
root=Path(__file__).parent;events=[]
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  if self.path=='/favicon.ico':status=204;data=b'';typ='image/x-icon'
  elif self.path in('/','/index.html','/mermaid.min.js'):
   path=root/('index.html'if self.path in('/','/index.html')else'mermaid.min.js');status=200;data=path.read_bytes();typ='text/html'if path.suffix=='.html'else'text/javascript'
  else:status=404;data=b'';typ='text/plain'
  events.append(dict(method='GET',path=self.path,status=status,bytes=len(data)))
  self.send_response(status);self.send_header('Content-Type',typ);self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
server=HTTPServer(('127.0.0.1',0),Handler);server.timeout=.5
(root/'server.json').write_text(json.dumps(dict(pid=os.getpid(),pgid=os.getpgid(0),url='http://127.0.0.1:'+str(server.server_port))))
start=time.monotonic()
try:
 while not(root/'STOP').exists()and time.monotonic()-start<600:server.handle_request()
finally:
 server.server_close();(root/'server-receipt.json').write_text(json.dumps(dict(closed=True,stopped_by_owned_marker=(root/'STOP').exists(),requests=events),indent=2)+'\n')
