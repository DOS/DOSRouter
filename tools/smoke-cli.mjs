import { renderReport } from './smoke-report.mjs';
// Process acceptance against synthetic localhost services, with isolated state.
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';
import { execFileSync, spawn } from 'node:child_process';
import { once } from 'node:events';
import fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import path from 'node:path';

assert.equal(process.platform, 'win32', 'This harness isolates USERPROFILE and is Windows-only');
const args = process.argv.slice(2);
const option = name => args[args.indexOf(name) + 1];
assert(args.includes('--binary') && args.includes('--output'), '--binary and --output are required');
const binary = path.resolve(option('--binary'));
const output = path.resolve(option('--output'));
const runtime = path.resolve('.smoke-artifacts', `run-${randomUUID()}`);
await fs.mkdir(runtime, { recursive: true });
await fs.mkdir(output, { recursive: true });
const revision = execFileSync('git', ['rev-parse', '--verify', `${args.includes('--revision') ? option('--revision') : 'HEAD'}^{commit}`], { encoding: 'utf8' }).trim();
const buildInfo = execFileSync('go', ['version', '-m', binary], { encoding: 'utf8' });
assert(buildInfo.includes(`vcs.revision=${revision}`), 'binary and checkout revisions differ');
assert(buildInfo.includes('vcs.modified=false'), 'binary was not built from clean source');
const evidence = { schema: 1, startedAt: new Date().toISOString(), revision,
  binarySha256: createHash('sha256').update(await fs.readFile(binary)).digest('hex'),
  buildInfo: buildInfo.split(/\r?\n/).filter(Boolean),
  scope: 'Actual CLI processes; synthetic localhost gateway; no live provider or payment.',
  checks: [], trace: [], catalogReads: [], browser: null, shutdown: null };
const modes = { stable: 'valid', recovery: 'invalid', concurrency: 'valid' };
const reads = { stable: 0, recovery: 0, concurrency: 0 };
const caseCounts = new Map(), tickWaiters = new Map(), children = [];
let releaseHold, markHold, server;
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const exited = child => child.exitCode !== null || child.signalCode !== null;

const report = () => renderReport(evidence);

async function save() {
  await fs.writeFile(path.join(output,'upstream-smoke.json'),JSON.stringify(evidence,null,2)+'\n');
  await fs.writeFile(path.join(output,'upstream-smoke.html'),report());
}
async function check(id,label,fn) {
  const at=Date.now();
  try { const details=await fn();evidence.checks.push({id,label,pass:true,durationMs:Date.now()-at,details}); }
  catch(error) { evidence.checks.push({id,label,pass:false,durationMs:Date.now()-at,details:{error:error.message}}); }
  await save(); console.log(JSON.stringify({check:id,pass:evidence.checks.at(-1).pass}));
}
const row=(id,pricing,categories=['chat'],billing_mode='paid')=>({id,name:id,categories,billing_mode,pricing,context_window:128000,max_output:32000});
function complete(w,model,tag,stream=false) {
  w.setHeader('X-DOS-Cost-USD',model.startsWith('free/')?'0':'0.001');
  w.setHeader('X-DOS-Request-Id',`fixture-${tag}`);
  if(stream) {
    w.setHeader('Content-Type','text/event-stream');
    w.end(`data: ${JSON.stringify({model,choices:[{index:0,delta:{content:'fixture-stream-ok'},finish_reason:null}]})}\n\ndata: ${JSON.stringify({model,choices:[{index:0,delta:{},finish_reason:'stop'}],usage:{prompt_tokens:10,completion_tokens:5}})}\n\ndata: [DONE]\n\n`);
  } else { w.setHeader('Content-Type','application/json');w.end(JSON.stringify({model,choices:[{index:0,message:{role:'assistant',content:'fixture-ok'},finish_reason:'stop'}],usage:{prompt_tokens:10,completion_tokens:5}})); }
}
async function stopChildren() {
  for(const child of children) if(!exited(child)){child.kill();await once(child,'exit');}
  const closed=await Promise.all(children.map(async c=>{try{await fetch(`http://127.0.0.1:${c.port}/health`,{signal:AbortSignal.timeout(500)});return false;}catch{return true;}}));
  evidence.shutdown={allOwnedProcessesExited:children.every(exited),portsClosed:closed.every(Boolean)};await save();
}
process.on('uncaughtException',async error=>{console.error(error.message);await stopChildren();server?.close();process.exit(1);});
server=http.createServer(async(r,w)=>{
  try {
    const url=new URL(r.url,'http://127.0.0.1');
    if(url.pathname==='/'||url.pathname==='/report'){w.setHeader('Content-Type','text/html; charset=utf-8');w.end(report());return;}
    if(url.pathname==='/evidence.json'||url.pathname==='/upstream-smoke.json'){w.setHeader('Content-Type','application/json');w.end(JSON.stringify(evidence));return;}
    if(url.pathname==='/__finish'&&r.method==='POST'){let body='';for await(const chunk of r)body+=chunk;evidence.browser=JSON.parse(body);await stopChildren();evidence.finishedAt=new Date().toISOString();await save();w.end('ok');return;}
    if(url.pathname==='/__stop'&&r.method==='POST'){await stopChildren();w.end('ok');server.close();return;}
    const gateway=url.pathname.split('/')[1];
    if(!Object.hasOwn(modes,gateway)){w.writeHead(404);w.end();return;}
    if(url.pathname.endsWith('/v1/models')){
      reads[gateway]++;evidence.catalogReads.push({gateway,count:reads[gateway],at:new Date().toISOString(),mode:modes[gateway]});w.setHeader('Content-Type','application/json');
      w.end(JSON.stringify(modes[gateway]==='invalid'?{data:[]}:{data:[row('fixture/reasoner',{input:1,output:1},['chat','reasoning']),row('fixture/expensive',{input:1,output:1000}),row('fixture/zero',{flat:0},['chat'],'flat')]}));
      tickWaiters.get(`${gateway}:${reads[gateway]}`)?.();return;
    }
    if(!url.pathname.endsWith('/v1/chat/completions')){w.writeHead(404);w.end();return;}
    let body='';for await(const chunk of r)body+=chunk;const req=JSON.parse(body),tag=req.metadata?.smoke_case??'browser';
    const count=(caseCounts.get(tag)??0)+1;caseCounts.set(tag,count);
    const record={gateway,tag,model:req.model,sequence:count,status:200,reasoningNormalized:req.messages?.filter(m=>m.role==='assistant').every(m=>typeof m.reasoning_content==='string')??true};evidence.trace.push(record);
    if(tag.startsWith('repeat-failure-')&&req.model==='google/gemini-2.5-flash'||tag==='all-fail'||tag==='explicit-pin-fails'){record.status=502;w.writeHead(502,{'X-DOS-Cost-USD':'0'});w.end('{"error":{"message":"synthetic model failure"}}');return;}
    if(tag==='empty-chain'&&count<=2){w.setHeader('X-DOS-Cost-USD','0');w.end('{"choices":[{"message":{"content":"","tool_calls":[]},"finish_reason":"stop"}]}');return;}
    if(tag==='lost-response'){record.status='socket-closed';r.socket.destroy();return;}
    if(tag==='concurrency-first'){releaseHold=()=>complete(w,req.model,tag);markHold?.();return;}
    complete(w,req.model,tag,req.stream);
  }catch{if(!w.headersSent)w.writeHead(500);w.end('fixture failure');}
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const fixture=`http://127.0.0.1:${server.address().port}`;
async function freePort(){const s=net.createServer();await new Promise(resolve=>s.listen(0,'127.0.0.1',resolve));const port=s.address().port;await new Promise(resolve=>s.close(resolve));return port;}
async function start(name,limits){
  const port=await freePort(),profile=path.join(runtime,name);await fs.mkdir(path.join(profile,'.openclaw','DOS'),{recursive:true});await fs.writeFile(path.join(profile,'.openclaw','DOS','spending.json'),JSON.stringify({limits,history:[]}));
  const env={...process.env,USERPROFILE:profile,DOSROUTER_API_KEY:'',HTTP_PROXY:'',HTTPS_PROXY:'',ALL_PROXY:''};
  const child=spawn(binary,['serve','--port',String(port),'--upstream',`${fixture}/${name}`],{env,windowsHide:true,stdio:['ignore','pipe','pipe']});
  child.port=port;child.profile=profile;child.stderrText='';child.stderr.on('data',d=>{child.stderrText+=d.toString();});child.stdout.resume();children.push(child);
  const base=`http://127.0.0.1:${port}`;
  for(let i=0;i<60;i++){try{const res=await fetch(`${base}/health`,{signal:AbortSignal.timeout(1000)});if(res.ok)return{child,base};}catch{}await sleep(100);}throw Error('CLI did not become ready');
}
async function jsonRequest(base,route,payload,headers={}){
  const res=await fetch(base+route,{method:payload===undefined?'GET':'POST',headers:{'Content-Type':'application/json',...headers},body:payload===undefined?undefined:JSON.stringify(payload),signal:AbortSignal.timeout(10000)});
  const text=await res.text();let body;try{body=JSON.parse(text);}catch{body=text;}return{status:res.status,headers:Object.fromEntries(res.headers),body};
}
async function rawRequest(base,route,payload,headers={}){return new Promise((resolve,reject)=>{const req=http.request(base+route,{method:'POST',headers:{'Content-Type':'application/json',...headers}},res=>{let body='';res.on('data',c=>{body+=c});res.on('end',()=>resolve({status:res.statusCode,body}));});req.setTimeout(10000,()=>req.destroy(new Error('raw HTTP request timed out')));req.on('error',reject);req.end(JSON.stringify(payload));});}
const chat=(base,tag,model='auto',extra={})=>jsonRequest(base,'/v1/chat/completions',{model,messages:[{role:'user',content:'hello'}],max_tokens:16,metadata:{smoke_case:tag},...extra},{'X-Session-Id':randomUUID(),Authorization:'Bearer synthetic-local-smoke'});
const traceFor=tag=>evidence.trace.filter(r=>r.tag===tag);
async function visible(base,id){const res=await jsonRequest(base,'/v1/models');return res.body.data.some(m=>m.id===id);}
async function awaitVisible(base,id){for(let i=0;i<30;i++){if(await visible(base,id))return;await sleep(100);}throw Error(`${id} was not projected`);}
const stable=await start('stable',{perRequest:0.05,session:10});await awaitVisible(stable.base,'fixture/reasoner');
const recovery=await start('recovery',{perRequest:0.05,session:10});
const concurrency=await start('concurrency',{perRequest:1,session:0.015});await awaitVisible(concurrency.base,'fixture/reasoner');
console.log(JSON.stringify({ready:true,fixture,proxy:stable.base,report:fixture+'/report',binarySha256:evidence.binarySha256,revision,pids:children.map(c=>c.pid)}));
await check('health','Health và revision của tiến trình CLI',async()=>{const res=await jsonRequest(stable.base,'/health?full=true');assert.equal(res.body.status,'ok');assert.equal(res.body.version,revision.slice(0,7));return res.body;});
await check('startup-catalog','Catalog mới tự nạp khi khởi động',async()=>{assert(await visible(stable.base,'fixture/reasoner'));assert.equal(reads.stable,1);return{reads:reads.stable,modelVisible:true};});
await check('invalid-startup','Catalog lỗi giữ bản đóng gói',async()=>{assert(!(await visible(recovery.base,'fixture/reasoner')));assert(await visible(recovery.base,'openai/gpt-6.1-sol'));return{bundledFallback:true};});
await check('reasoning','Model reasoning mới chuyển tiếp đúng metadata',async()=>{const res=await chat(stable.base,'reasoning','fixture/reasoner',{messages:[{role:'user',content:'hi'},{role:'assistant',content:'hello'},{role:'user',content:'continue'}]});assert.equal(res.status,200);assert(traceFor('reasoning')[0].reasoningNormalized);return{status:res.status,trace:traceFor('reasoning')};});
await check('free-alias','Alias miễn phí không thành model trả phí',async()=>{const res=await chat(stable.base,'free-alias','v4-flash');assert.equal(res.status,200);assert.equal(traceFor('free-alias')[0].model,'free/nemotron-3.5-lightning');return traceFor('free-alias');});
await check('outcome-memory','Model lỗi lặp lại được chuyển xuống cuối',async()=>{for(let i=0;i<4;i++)assert.equal((await chat(stable.base,`repeat-failure-${i}`)).status,200);for(let i=0;i<3;i++)assert.equal(traceFor(`repeat-failure-${i}`)[0].model,'google/gemini-2.5-flash');assert.notEqual(traceFor('repeat-failure-3')[0].model,'google/gemini-2.5-flash');return evidence.trace.filter(t=>t.tag.startsWith('repeat-failure-'));});
await check('empty-chain','Phản hồi rỗng tiếp tục tới model có kết quả',async()=>{const res=await chat(stable.base,'empty-chain','premium');assert.equal(res.status,200);assert.equal(traceFor('empty-chain').length,3);assert.equal(res.body.choices[0].message.content,'fixture-ok');return traceFor('empty-chain');});
await check('attempt-cap','Chuỗi lỗi dừng sau tối đa 5 model',async()=>{const res=await chat(stable.base,'all-fail','premium');assert.equal(res.status,502);assert.equal(traceFor('all-fail').length,5);return traceFor('all-fail');});
await check('explicit-pin','Model chỉ định không bị thay bằng fallback',async()=>{const res=await chat(stable.base,'explicit-pin-fails','openai/gpt-4o-mini');assert.equal(res.status,502);assert.equal(traceFor('explicit-pin-fails').length,1);return traceFor('explicit-pin-fails');});
await check('lost-send','Mất phản hồi không làm gửi lại request',async()=>{const res=await chat(stable.base,'lost-response');assert.equal(res.status,502);assert.equal(traceFor('lost-response').length,1);return traceFor('lost-response');});
await check('stream','SSE giữ nội dung và dấu kết thúc',async()=>{const res=await chat(stable.base,'stream','fixture/reasoner',{stream:true});assert.equal(res.status,200);assert(res.body.includes('fixture-stream-ok'));assert(res.body.includes('data: [DONE]'));return{status:res.status,done:true};});
await check('expensive-admission','Giá cao bị chặn trước khi gửi',async()=>{const res=await chat(stable.base,'expensive','fixture/expensive',{max_tokens:4096});assert.equal(res.status,429);assert.equal(traceFor('expensive').length,0);return{status:res.status,upstreamSends:0};});
await check('zero-flat','Giá cố định bằng 0 không vượt spend limit',async()=>{const res=await chat(stable.base,'zero-flat','fixture/zero');assert.equal(res.status,429);assert.equal(traceFor('zero-flat').length,0);return{status:res.status,upstreamSends:0};});
await check('local-trust','Host và Origin không tin cậy bị từ chối',async()=>{const statuses=[],before=evidence.trace.length;for(const headers of[{Origin:'https://attacker.invalid'},{Origin:'null'},{Host:'attacker.invalid'},{'Sec-Fetch-Site':'cross-site'}]){const res=await rawRequest(stable.base,'/v1/chat/completions',{},headers);assert.equal(res.status,403);statuses.push(res.status);}assert.equal(evidence.trace.length,before);return{statuses,upstreamSends:0};});
await check('cjk','Prompt CJK dài được nâng tier theo số token',async()=>{const res=await jsonRequest(stable.base,'/debug',{prompt:'中'.repeat(110000),max_tokens:16});assert.equal(res.status,200);assert.equal(res.body.decision.Tier,'COMPLEX');assert(res.body.decision.Reasoning.includes('Input exceeds'));return{tier:res.body.decision.Tier,reason:res.body.decision.Reasoning};});
await check('concurrency','Reservation đồng thời và settlement trả lại ngân sách',async()=>{const held=new Promise(resolve=>{markHold=resolve;});const first=chat(concurrency.base,'concurrency-first','fixture/reasoner',{max_tokens:10000});await Promise.race([held,sleep(10000).then(()=>{throw Error('first request never arrived');})]);const second=await chat(concurrency.base,'concurrency-second','fixture/reasoner',{max_tokens:10000});assert.equal(second.status,429);assert.equal(traceFor('concurrency-second').length,0);releaseHold();assert.equal((await first).status,200);const third=await chat(concurrency.base,'concurrency-third','fixture/reasoner',{max_tokens:10000});assert.equal(third.status,200);return{first:200,whileInFlight:429,afterSettlement:200};});
const stableTick=new Promise(resolve=>tickWaiters.set('stable:2',resolve));
const recoveryTick=new Promise(resolve=>tickWaiters.set('recovery:2',resolve));
modes.stable='invalid';modes.recovery='valid';
console.log(JSON.stringify({waitingFor:'real five-minute catalog refresh',firstReads:evidence.catalogReads}));
await check('scheduled-refresh','Refresh 5 phút và phục hồi catalog',async()=>{await Promise.race([Promise.all([stableTick,recoveryTick]),sleep(330000).then(()=>{throw Error('scheduled refresh deadline exceeded');})]);await awaitVisible(recovery.base,'fixture/reasoner');assert(await visible(stable.base,'fixture/reasoner'));const intervals=['stable','recovery'].map(gateway=>{const entries=evidence.catalogReads.filter(r=>r.gateway===gateway);return{gateway,intervalMs:Date.parse(entries[1].at)-Date.parse(entries[0].at)};});for(const entry of intervals)assert(entry.intervalMs>=295000&&entry.intervalMs<=325000);return{intervals,lastGoodRetained:true,recoveredFromInvalidStartup:true};});
await check('isolated-state','Accounting chỉ ghi vào thư mục thử nghiệm',async()=>{const states=await Promise.all(children.map(async c=>JSON.parse(await fs.readFile(path.join(c.profile,'.openclaw','DOS','spending.json'),'utf8'))));assert(states[0].history.length>0);assert.equal(states[2].history.length,2);return{isolatedProfiles:children.length,historyCounts:states.map(s=>s.history.length),syntheticOnly:true};});
evidence.apiFinishedAt=new Date().toISOString();await save();
console.log(JSON.stringify({apiFinished:true,passed:evidence.checks.filter(c=>c.pass).length,total:evidence.checks.length,fixture,proxy:stable.base,report:fixture+'/report'}));
if(!args.includes('--serve')){await stopChildren();evidence.finishedAt=new Date().toISOString();await save();server.close();process.exitCode=evidence.checks.every(c=>c.pass)&&evidence.shutdown.allOwnedProcessesExited&&evidence.shutdown.portsClosed?0:1;}
process.on('SIGINT',async()=>{await stopChildren();server.close();});
