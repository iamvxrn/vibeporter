let chats = [], filtered = [], selected = null;
let agents = ["claudecode","cursor","opencode","antigravity","kimicode","gemini","windsurf"];
let activeAgent = "all";

async function fetchJSON(url){
  const r = await fetch(url);
  if(!r.ok) throw new Error(url+" "+r.status);
  return r.json();
}

function el(tag, cls, text){
  const e = document.createElement(tag);
  if(cls) e.className = cls;
  if(text) e.textContent = text;
  return e;
}

async function loadStats(){
  try{
    const rows = await fetchJSON('/api/stats');
    const bar = document.getElementById('statsBar');
    bar.innerHTML = rows.map(r=>`<span><b>${r.agent}</b> ${r.chats} chats · ${r.messages} msgs · ~${r.tokens_est} tokens</span>`).join('');
  }catch{}
}

function renderAgentTabs(){
  const c = document.getElementById('agentTabs');
  c.innerHTML = '';
  const allBtn = el('button', activeAgent==='all'?'active':'', 'all');
  allBtn.onclick = ()=>{activeAgent='all'; renderAgentTabs(); filterAndRender();};
  c.appendChild(allBtn);
  for(const ag of agents){
    const b = el('button', activeAgent===ag?'active':'', ag);
    b.onclick = ()=>{activeAgent=ag; renderAgentTabs(); filterAndRender();};
    c.appendChild(b);
  }
  const fromSel = document.getElementById('fromSel');
  const toSel = document.getElementById('toSel');
  [fromSel,toSel].forEach(sel=>{
    sel.innerHTML = '';
    for(const ag of agents){
      const o = document.createElement('option');
      o.value = ag; o.textContent = ag;
      sel.appendChild(o);
    }
  });
  fromSel.value = 'cursor'; toSel.value = 'opencode';
}

async function loadChats(){
  const all = await fetchJSON('/api/chats');
  chats = all;
  filterAndRender();
}

function filterAndRender(){
  const q = document.getElementById('q').value.trim().toLowerCase();
  filtered = chats.filter(c=>{
    if(activeAgent!=='all' && c.Agent!==activeAgent) return false;
    if(!q) return true;
    const hay = (c.Title+' '+c.Project+' '+c.ID).toLowerCase();
    return hay.includes(q);
  });
  renderChatList();
}

function renderChatList(){
  const list = document.getElementById('chatList');
  list.innerHTML = '';
  for(const c of filtered.slice(0,100)){
    const div = el('div','chat');
    if(selected && selected.ID===c.ID) div.classList.add('active');
    div.draggable = true;
    div.ondragstart = (e)=>{ e.dataTransfer.setData('text/plain', JSON.stringify({id:c.ID, agent:c.Agent})); };
    div.onclick = ()=>selectChat(c);
    const title = el('div','title', c.Title||'Untitled');
    const meta = el('div','meta', `${c.Agent} · ${new Date(c.UpdatedAt).toLocaleString()} · ${c.ID.slice(0,8)}`);
    div.appendChild(title); div.appendChild(meta);
    if(c.Project) div.appendChild(el('div','meta', c.Project));
    list.appendChild(div);
  }
}

async function selectChat(c){
  selected = c;
  document.getElementById('fromSel').value = c.Agent;
  renderChatList();
  document.getElementById('previewHeader').textContent = `${c.Agent} · ${c.Title} · ${c.ID}`;
  const preview = document.getElementById('preview');
  preview.innerHTML = 'loading…';
  try{
    const conv = await fetchJSON(`/api/conversation?agent=${encodeURIComponent(c.Agent)}&id=${encodeURIComponent(c.ID)}`);
    preview.innerHTML = '';
    for(const m of conv.messages){
      const div = el('div','msg '+m.role);
      const role = el('div','muted', m.role);
      div.appendChild(role);
      if(m.parts && m.parts.length){
        for(const p of m.parts){
          if(p.kind==='text') div.appendChild(el('div',null, p.text));
          else if(p.kind==='thinking'){
            const bq = el('blockquote',null, p.text);
            div.appendChild(bq);
          } else if(p.kind==='tool_call'){
            const pre = el('pre',null, `Tool ${p.name}: ${p.args_json}`);
            div.appendChild(pre);
          } else if(p.kind==='tool_result'){
            const pre = el('pre',null, p.text);
            div.appendChild(pre);
          }
        }
      } else {
        div.appendChild(el('div',null, m.content||''));
      }
      preview.appendChild(div);
    }
  }catch(e){
    preview.textContent = 'failed: '+e.message;
  }
}

async function doSearch(){
  const q = document.getElementById('q').value.trim();
  if(!q){ filterAndRender(); return; }
  const res = await fetchJSON(`/api/search?q=${encodeURIComponent(q)}&agent=${activeAgent==='all'?'':activeAgent}`);
  // show search hits as filtered
  chats = res.map(h=>({ID:h.id, Title:h.title, Project:h.project, UpdatedAt:new Date().toISOString(), Agent:h.agent, Path:h.path}));
  filtered = chats;
  renderChatList();
  const preview = document.getElementById('preview');
  preview.innerHTML = `<b>${res.length} hits for "${q}"</b>` + res.map(h=>`<div class="msg"><b>${h.agent}/${h.id}</b> — ${h.snippet} <small>matches ${h.matches}</small></div>`).join('');
}

async function doDiff(){
  if(!selected) return alert('select a chat first');
  const from = document.getElementById('fromSel').value;
  const to = document.getElementById('toSel').value;
  const panel = document.getElementById('diffPanel');
  panel.classList.remove('hidden');
  panel.textContent = 'diffing…';
  try{
    const r = await fetchJSON(`/api/diff?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&source=${encodeURIComponent(selected.ID)}`);
    const o = r.from.counts, n = r.to.counts;
    panel.innerHTML = `<b>Diff ${from} → ${to}</b><br>messages ${o.messages} → ${n.messages} · text ${o.text}→${n.text} · thinking ${o.thinking}→${n.thinking} · tool_call ${o.tool_call}→${n.tool_call}<br><small>Tip: check preview before migrate</small>`;
  }catch(e){ panel.textContent='diff failed: '+e.message; }
}

async function doMigrate(){
  if(!selected) return alert('select a chat');
  const from = document.getElementById('fromSel').value;
  const to = document.getElementById('toSel').value;
  const status = document.getElementById('migrateStatus');
  status.textContent = 'migrating…';
  try{
    const r = await fetch('/api/migrate', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({from, to, source: selected.ID})
    });
    const j = await r.json();
    if(!r.ok) throw new Error(j.error || r.statusText);
    status.textContent = `Migrated ${from} → ${to}: ${j.target} — refresh list`;
    // refresh chats for target agent
    const fresh = await fetchJSON(`/api/chats?agent=${encodeURIComponent(to)}`);
    // optionally select new chat
  }catch(e){
    status.textContent = 'migrate failed: '+e.message;
  }
}

document.getElementById('searchBtn').onclick = doSearch;
document.getElementById('q').addEventListener('keydown', e=>{ if(e.key==='Enter') doSearch(); });
document.getElementById('diffBtn').onclick = doDiff;
document.getElementById('migrateBtn').onclick = doMigrate;

document.getElementById('addr').textContent = location.host;
loadStats();
renderAgentTabs();
loadChats();
