// Редактор комментария: пишешь и видишь сразу жирный, список, цитату, без звёздочек.
// Внутри contenteditable, на сервер уходит наш Markdown (то же подмножество, что рендерит бэкенд):
// перед отправкой кладём сериализованный текст в скрытую textarea. Вставленный Markdown превращаем в разметку.
(function(){
  var form=document.querySelector('[data-comment-form]');if(!form)return;
  var ta=form.querySelector('textarea[name=body]');if(!ta)return;
  var ed=document.createElement('div');ed.className='mdedit form-control';ed.contentEditable='true';ed.setAttribute('role','textbox');ed.setAttribute('aria-multiline','true');
  ed.setAttribute('aria-label',ta.getAttribute('aria-label')||'');ed.dataset.ph=ta.placeholder||'';ed.spellcheck=true;
  ta.hidden=true;ta.required=false;ta.tabIndex=-1;ta.parentNode.insertBefore(ed,ta);
  try{document.execCommand('defaultParagraphSeparator',false,'div')}catch(e){}

  // Markdown → HTML (для вставки)
  function esc(s){return s.replace(/[&<>"]/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]})}
  function inline(s){s=esc(s);
    s=s.replace(/`([^`\n]+?)`/g,'<code>$1</code>');
    s=s.replace(/\[([^\]\n]+?)\]\((https?:\/\/[^\s)]+)\)/g,'<a href="$2">$1</a>');
    s=s.replace(/\*\*(.+?)\*\*/g,'<b>$1</b>');
    s=s.replace(/(^|[^*\w])\*([^*\n]+?)\*/g,'$1<i>$2</i>');
    s=s.replace(/(^|[^_\w])_([^_\n]+?)_/g,'$1<i>$2</i>');
    s=s.replace(/~~(.+?)~~/g,'<s>$1</s>');return s}
  function mdToHtml(src){var out='',list='',lines=src.replace(/\r\n/g,'\n').split('\n');
    function close(){if(list==='ul')out+='</ul>';else if(list==='ol')out+='</ol>';else if(list==='quote')out+='</blockquote>';list=''}
    lines.forEach(function(raw){var l=raw.trim();
      if(l===''){close();return}
      if(/^[-*•] /.test(l)){if(list!=='ul'){close();out+='<ul>';list='ul'}out+='<li>'+inline(l.slice(2))+'</li>';return}
      if(/^\d{1,3}\. /.test(l)){if(list!=='ol'){close();out+='<ol>';list='ol'}out+='<li>'+inline(l.replace(/^\d+\. /,''))+'</li>';return}
      if(/^> /.test(l)){if(list!=='quote'){close();out+='<blockquote>';list='quote'}else out+='<br>';out+=inline(l.slice(2));return}
      close();out+='<div>'+inline(l)+'</div>'});
    close();return out}
  function looksMd(s){return /(\*\*|~~|`|\[[^\]]+\]\(https?:\/\/|^\s*([-*•]|\d+\.|>) )/m.test(s)}

  // HTML → Markdown (для отправки)
  function text(n){return n.textContent.replace(/ /g,' ')}
  function inl(n){var r='';n.childNodes.forEach(function(c){r+=one(c)});return r}
  function one(c){
    if(c.nodeType===3)return text(c);
    if(c.nodeType!==1)return '';var t=c.tagName,s=inl(c);
    if(t==='BR')return '\n';
    if(t==='B'||t==='STRONG')return s.trim()?'**'+s.trim()+'**':s;
    if(t==='I'||t==='EM')return s.trim()?'*'+s.trim()+'*':s;
    if(t==='S'||t==='DEL'||t==='STRIKE')return s.trim()?'~~'+s.trim()+'~~':s;
    if(t==='CODE')return '`'+s+'`';
    if(t==='A'&&/^https?:\/\//.test(c.getAttribute('href')||''))return '['+(s||c.getAttribute('href'))+']('+c.getAttribute('href')+')';
    if(t==='DIV'||t==='P')return '\n'+s+'\n';
    return s}
  // Строчные узлы на верхнем уровне (текст, <b>, <a>…) склеиваем в одну строку, блоки — отдельными строками
  function blocks(root){var lines=[],cur='';
    function flush(){if(cur!==''){lines.push(cur);cur=''}}
    root.childNodes.forEach(function(c){
      if(c.nodeType===3){cur+=text(c);return}
      if(c.nodeType!==1)return;var t=c.tagName;
      if(t==='UL'||t==='OL'){flush();var i=0;c.querySelectorAll(':scope > li').forEach(function(li){i++;lines.push((t==='UL'?'- ':i+'. ')+inl(li).trim().replace(/\n+/g,' '))});lines.push('')}
      else if(t==='BLOCKQUOTE'){flush();inl(c).split('\n').forEach(function(l){if(l.trim())lines.push('> '+l.trim())});lines.push('')}
      else if(t==='DIV'||t==='P'){flush();if(c.querySelector('ul,ol,blockquote')){lines=lines.concat(blocks(c));return}lines.push(inl(c).replace(/^\n+|\n+$/g,''))}
      else if(t==='BR'){flush()}
      else cur+=one(c)});
    flush();return lines}
  function toMd(){return blocks(ed).join('\n').replace(/\n{3,}/g,'\n\n').trim()}

  // Очистка вставленного HTML до наших тегов
  var keep={B:'b',STRONG:'b',I:'i',EM:'i',S:'s',DEL:'s',STRIKE:'s',CODE:'code',A:'a',UL:'ul',OL:'ol',LI:'li',BLOCKQUOTE:'blockquote',BR:'br',DIV:'div',P:'div'};
  function clean(node){var frag=document.createDocumentFragment();
    node.childNodes.forEach(function(c){
      if(c.nodeType===3){frag.appendChild(document.createTextNode(c.nodeValue));return}
      if(c.nodeType!==1)return;var tag=keep[c.tagName];var inner=clean(c);
      if(!tag){frag.appendChild(inner);return}
      var el=document.createElement(tag);
      if(tag==='a'){var h=c.getAttribute('href')||'';if(!/^https?:\/\//.test(h)){frag.appendChild(inner);return}el.setAttribute('href',h)}
      el.appendChild(inner);frag.appendChild(el)});
    return frag}
  function insertHtml(html){var box=document.createElement('div');box.innerHTML=html;var tmp=document.createElement('div');tmp.appendChild(clean(box));document.execCommand('insertHTML',false,tmp.innerHTML)}
  ed.addEventListener('paste',function(e){var cd=e.clipboardData;if(!cd)return;var html=cd.getData('text/html'),txt=cd.getData('text/plain');
    e.preventDefault();
    if(html&&!looksMd(txt))insertHtml(html);
    else if(txt&&looksMd(txt))insertHtml(mdToHtml(txt));
    else document.execCommand('insertText',false,txt)});

  // Панель
  var cmds={bold:'bold',italic:'italic',strike:'strikeThrough',list:'insertUnorderedList'};
  form.querySelectorAll('[data-md]').forEach(function(b){
    b.addEventListener('mousedown',function(e){e.preventDefault()});
    b.addEventListener('click',function(){ed.focus();var k=b.dataset.md;
      if(cmds[k])document.execCommand(cmds[k]);
      else if(k==='quote'){var inQ=document.queryCommandValue('formatBlock').toLowerCase()==='blockquote';document.execCommand('formatBlock',false,inQ?'div':'blockquote')}
      else if(k==='link'){openLink();return}
      sync()})});
  // Ссылка: своя строка ввода под панелью вместо системного prompt; выделение запоминаем и возвращаем
  var linkRow=form.querySelector('[data-md-linkrow]'),linkIn=linkRow&&linkRow.querySelector('input'),savedRange=null;
  function openLink(){if(!linkRow)return;var sel=window.getSelection();savedRange=sel.rangeCount?sel.getRangeAt(0).cloneRange():null;
    var t=String(sel);linkIn.value=/^https?:\/\//.test(t)?t:'https://';linkRow.hidden=false;linkIn.focus();linkIn.setSelectionRange(linkIn.value.length,linkIn.value.length)}
  function closeLink(){if(!linkRow)return;linkRow.hidden=true;ed.focus();if(savedRange){var sel=window.getSelection();sel.removeAllRanges();sel.addRange(savedRange)}}
  function applyLink(){var u=linkIn.value.trim();var err=linkRow.querySelector('[data-md-linkerr]');if(!/^https?:\/\/\S+\.\S+$/.test(u)){err.textContent=linkIn.dataset.bad||'';err.hidden=false;linkIn.classList.add('is-invalid');linkIn.focus();return}err.hidden=true;linkIn.classList.remove('is-invalid');
    closeLink();var t=String(window.getSelection());
    if(t)document.execCommand('createLink',false,u);else{var r=window.getSelection().rangeCount?window.getSelection().getRangeAt(0):null,prev=r&&r.startContainer.nodeType===3?r.startContainer.textContent.slice(0,r.startOffset):'',sp=prev&&!/\s$/.test(prev)?' ':'';document.execCommand('insertHTML',false,sp+'<a href="'+esc(u)+'">'+esc(u)+'</a>')}sync()}
  if(linkRow){linkRow.querySelector('[data-md-linkok]').addEventListener('click',applyLink);linkRow.querySelector('[data-md-linkno]').addEventListener('click',closeLink);
    linkIn.addEventListener('keydown',function(e){if(e.key==='Enter'){e.preventDefault();applyLink()}else if(e.key==='Escape'){e.preventDefault();closeLink()}});linkIn.addEventListener('input',function(){linkIn.classList.remove('is-invalid');var err=linkRow.querySelector('[data-md-linkerr]');if(err)err.hidden=true})}
  function sync(){var md=toMd();origSet.call(ta,md);
    form.querySelectorAll('[data-md]').forEach(function(b){var k=b.dataset.md,on=false;try{on=cmds[k]?document.queryCommandState(cmds[k]):k==='quote'?document.queryCommandValue('formatBlock').toLowerCase()==='blockquote':false}catch(e){}b.setAttribute('aria-pressed',on?'true':'false')});
    ed.classList.toggle('is-empty',!ed.textContent.trim());
    var n=form.querySelector('[data-md-count]');if(n)n.textContent=md.length>800?md.length+'/1000':''}
  ed.addEventListener('input',sync);
  document.addEventListener('selectionchange',function(){if(document.activeElement===ed)sync()});
  // ssr.js читает ta.value при отправке и пишет '' после успеха: пустое значение чистит и редактор
  var desc=Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value'),origSet=desc.set;
  Object.defineProperty(ta,'value',{get:function(){return desc.get.call(this)},set:function(v){origSet.call(this,v);if(v==='')ed.innerHTML='';ed.classList.toggle('is-empty',!ed.textContent.trim())}});
  sync();
})();
