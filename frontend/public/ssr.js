// Страницы рецептов (SSR): кнопка темы и запоминание языка. Внешний файл ради строгой CSP.
(function(){
  var K="racion.theme",N={auto:"light",light:"dark",dark:"auto"};
  var b=document.querySelector("[data-theme-btn]");if(!b)return;
  var L={auto:b.dataset.lAuto,light:b.dataset.lLight,dark:b.dataset.lDark},F=b.dataset.lFmt||"%1 → %2";
  function cur(){try{var v=localStorage.getItem(K);return v==="light"||v==="dark"?v:"auto"}catch(e){return "auto"}}
  function show(t){b.setAttribute("data-theme-btn",t);b.setAttribute("aria-label",F.replace("%1",L[t]).replace("%2",L[N[t]]))}
  show(cur());
  b.addEventListener("click",function(){var t=N[cur()];try{if(t==="auto")localStorage.removeItem(K);else localStorage.setItem(K,t)}catch(e){}
    if(t==="auto")document.documentElement.removeAttribute("data-theme");else document.documentElement.setAttribute("data-theme",t);show(t)});
  // выбранный язык запоминаем для приложения и API
  // лист выбора языка: открывается кнопкой-переводчиком, выбор запоминается для приложения и API
  // замены продуктов на странице рецепта: кнопка у продукта → список вариантов с разницей в ккал и цене;
  // выбор подменяет строку на месте (без сохранения), «вернуть» — обратно
  // «цены для России» → раскрыть фильтры и подвести к стране
  var ld=document.querySelector(".langdlg"),lb=document.querySelector("[data-lang-btn]");
  if(ld&&lb){lb.addEventListener("click",function(){ld.showModal()});var lb2=document.querySelector("[data-lang-btn2]");if(lb2)lb2.addEventListener("click",function(){ld.showModal()});var lc=ld.querySelector("[data-lang-close]");if(lc)lc.addEventListener("click",function(){ld.close()});ld.addEventListener("click",function(e){if(e.target===ld)ld.close()})}
  document.querySelectorAll(".langlist a[data-lang]").forEach(function(a){a.addEventListener("click",function(){
    try{localStorage.setItem("racion.lang",a.dataset.lang)}catch(e){}
    document.cookie="racion_lang="+a.dataset.lang+";path=/;max-age=31536000;samesite=lax"})});
})();

// Страница рецепта: счётчик порций пересчитывает граммовки (data-amount × n), изменённые ячейки мигают reprint.
(function(){
  var out=document.getElementById('portions-out'),list=document.querySelector('.ings');if(!out||!list)return;
  var n=1,cells=document.querySelectorAll('[data-amount]');
  var U=list.dataset,dec=U.dec||',';
  function trim(v){return v.toFixed(2).replace(/\.?0+$/,'').replace('.',dec)}
  function fmt(v,u){if(u==='pcs'){return (v%1?trim(v):v)+' '+U.uPcs}if(u==='ml'){return v>=1000?trim(v/1000)+' '+U.uL:Math.round(v)+' '+U.uMl}return v>=1000?trim(v/1000)+' '+U.uKg:Math.round(v)+' '+U.uG}
  document.querySelectorAll('[data-portions]').forEach(function(b){b.addEventListener('click',function(){
    n=Math.min(12,Math.max(1,n+Number(b.dataset.portions)));out.textContent=n;
    cells.forEach(function(c){c.textContent=fmt(Number(c.dataset.amount)*n,c.dataset.unit);c.classList.remove('is-fresh');void c.offsetWidth;c.classList.add('is-fresh')});
  })});
})();


// Рецепт: лайк, избранное, «поделиться» и комментарии — те же ручки, что у приложения.
(function(){
  var box=document.querySelector('.social');if(!box)return;
  var id=box.dataset.recipe,auth=box.dataset.auth==='1',L=box.dataset;
  function api(method,path,body){return fetch(path,{method:method,headers:{'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined}).then(function(r){if(!r.ok)return r.json().catch(function(){return{}}).then(function(j){throw new Error(j.error||('HTTP '+r.status))});return r.status===204?null:r.json()})}
  function needLogin(){location.href=L.login+'?next='+encodeURIComponent(location.pathname)}
  function toast(text){var t=document.createElement('div');t.className='toast';t.setAttribute('role','status');t.textContent=text;document.body.appendChild(t);setTimeout(function(){t.remove()},2000)}
  function paint(st){
    var like=box.querySelector('[data-like]'),fav=box.querySelector('[data-fav]');
    like.setAttribute('aria-pressed',st.liked?'true':'false');like.setAttribute('aria-label',st.liked?L.lUnlike:L.lLike);box.querySelector('[data-likes]').textContent=st.likes;
    fav.setAttribute('aria-pressed',st.favorite?'true':'false');fav.setAttribute('aria-label',st.favorite?L.lUnfav:L.lFav);box.querySelector('[data-fav-label]').textContent=st.favorite?fav.dataset.on||fav.querySelector('[data-fav-label]').textContent:L.lFav;
    document.querySelectorAll('[data-comments-n]').forEach(function(n){n.textContent=st.comments});
  }
  box.querySelector('[data-like]').addEventListener('click',function(){if(!auth)return needLogin();var on=this.getAttribute('aria-pressed')!=='true';api(on?'PUT':'DELETE','/api/recipes/'+id+'/like').then(paint).catch(function(e){toast(e.message)})});
  var favBtn=box.querySelector('[data-fav]');favBtn.dataset.on=favBtn.querySelector('[data-fav-label]').textContent;
  favBtn.addEventListener('click',function(){if(!auth)return needLogin();var on=this.getAttribute('aria-pressed')!=='true';api(on?'PUT':'DELETE','/api/recipes/'+id+'/favorite').then(function(st){paint(st);favBtn.querySelector('[data-fav-label]').textContent=st.favorite?(L.lInfav||'✓'):L.lFav}).catch(function(e){toast(e.message)})});
  box.querySelector('[data-share]').addEventListener('click',function(){var url=location.origin+location.pathname;if(navigator.share){navigator.share({title:document.title,url:url}).catch(function(){});return}navigator.clipboard.writeText(url).then(function(){toast(L.lCopied)})});
  var form=document.querySelector('[data-comment-form]'),list=document.querySelector('[data-comments]');
  function esc(s){return s.replace(/[&<>"]/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]})}
  function row(c){var li=document.createElement('li');li.className='comment';li.dataset.id=c.id;li.innerHTML='<div class="comment__head">'+(c.avatar?'<img class="comment__ava" src="'+esc(c.avatar)+'" alt="">':'')+'<b>@'+esc(c.nick)+'</b><time class="comment__time"></time>'+(c.mine?'<button type="button" class="comment__del" data-del aria-label="×">×</button>':'')+'</div><div class="comment__body">'+(c.html||esc(c.body))+'</div>'+(c.image?'<img class="comment__img" src="'+esc(c.image)+'" alt="">':'');return li}
  // панель разметки: оборачивает выделение в **…**, *…*, ~~…~~, ставит «- » / «> » в начало строки, вставляет ссылку
  var mdta=form&&form.querySelector('textarea');
  if(form&&mdta){form.querySelectorAll('[data-md]').forEach(function(b){b.addEventListener('click',function(){var s=mdta.selectionStart,e=mdta.selectionEnd,v=mdta.value,sel=v.slice(s,e),kind=b.dataset.md,ins,cs,ce;
    function wrap(m){ins=m+(sel||'')+m;cs=s+m.length;ce=cs+sel.length}
    if(kind==='bold')wrap('**');else if(kind==='italic')wrap('*');else if(kind==='strike')wrap('~~');
    else if(kind==='link'){var u=sel&&/^https?:\/\//.test(sel)?sel:'https://';ins='['+(sel&&!/^https?:\/\//.test(sel)?sel:'')+']('+u+')';cs=s+1;ce=cs+(sel&&!/^https?:\/\//.test(sel)?sel.length:0)}
    else{var ls=v.lastIndexOf('\n',s-1)+1,pre=kind==='list'?'- ':'> ';var block=v.slice(ls,e);ins=block.split('\n').map(function(l){return l.startsWith(pre)?l.slice(pre.length):pre+l}).join('\n');s=ls;cs=s;ce=s+ins.length}
    mdta.value=v.slice(0,s)+ins+v.slice(e);mdta.focus();mdta.setSelectionRange(cs,ce)})})}
  // фото к отзыву: грузим сразу при выборе, в комментарий уходит ссылка
  var photo=form&&form.querySelector('[data-comment-photo]'),photoURL='';
  if(photo){var pin=photo.querySelector('input'),plabel=photo.querySelector('[data-photo-label]'),ptext=plabel.innerHTML;pin.addEventListener('change',function(){var f=pin.files&&pin.files[0];if(!f)return;var fd=new FormData();fd.append('file',f);plabel.textContent='…';
    fetch('/api/uploads?kind=comment',{method:'POST',body:fd}).then(function(r){return r.json().then(function(j){if(!r.ok)throw new Error(j.error||r.status);return j})}).then(function(p){photoURL=p.url;plabel.innerHTML='<img src="'+esc(p.thumb)+'" alt="">'}).catch(function(x){plabel.innerHTML=ptext;toast(x.message)})})}
  if(form){form.addEventListener('submit',function(e){e.preventDefault();var ta=form.querySelector('textarea'),err=form.querySelector('.comments__err');err.hidden=true;
    api('POST','/api/recipes/'+id+'/comments',{body:ta.value,image:photoURL}).then(function(c){list.prepend(row(c));ta.value='';photoURL='';if(photo){photo.querySelector('[data-photo-label]').innerHTML=ptext}var empty=document.querySelector('[data-comments-empty]');if(empty)empty.remove();document.querySelectorAll('[data-comments-n]').forEach(function(n){n.textContent=Number(n.textContent)+1})}).catch(function(x){err.textContent=x.message;err.hidden=false})})}
  if(list){list.addEventListener('click',function(e){var b=e.target.closest('[data-del]');if(!b)return;var li=b.closest('.comment');api('DELETE','/api/comments/'+li.dataset.id).then(function(){li.remove();document.querySelectorAll('[data-comments-n]').forEach(function(n){n.textContent=Math.max(0,Number(n.textContent)-1)})}).catch(function(x){toast(x.message)})})}
})();

// Замены продуктов на странице рецепта.
(function(){
  function fmtQty(a,u){var d=ingsEl.dataset,dec=d.dec||",";function n(x,f){return x.toFixed(f).replace(".",dec)}
    if(u==="g")return a>=1000?n(a/1000,a%1000?1:0)+" "+(d.uKg||"кг"):Math.round(a)+" "+(d.uG||"г");
    if(u==="ml")return a>=1000?n(a/1000,a%1000?1:0)+" "+(d.uL||"л"):Math.round(a)+" "+(d.uMl||"мл");
    return (Math.round(a*2)/2).toString().replace(".",dec)+" "+(d.uPcs||"шт")}
  var ingsEl=document.querySelector(".ings"),sbox=document.querySelector(".social");var id=sbox&&sbox.dataset.recipe;
  function esc(s){return String(s).replace(/[&<>"]/g,function(c){return{"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c]})}
  if(ingsEl&&id){var country=(document.cookie.match(/(?:^|; )racion_country=(\w+)/)||[])[1]||"";
    fetch("/api/recipes/"+encodeURIComponent(id)+"/subs"+(country?"?country="+country:"")).then(function(r){return r.ok?r.json():[]}).then(function(rows){
      var byIng={};rows.forEach(function(r){byIng[r.ingredientId]=r.options});
      ingsEl.querySelectorAll("[data-sub]").forEach(function(b){var opts=byIng[b.dataset.sub];if(!opts||!opts.length)return;b.hidden=false;
        b.addEventListener("click",function(){var row=b.closest(".ings__row"),old=row.querySelector(".subs__pop");if(old){old.remove();return}
          var pop=document.createElement("div");pop.className="subs__pop";pop.setAttribute("role","menu");
          var title=document.createElement("div");title.className="subs__title";title.textContent=ingsEl.dataset.subsTitle||"";pop.appendChild(title);
          var pos=Number(row.querySelector(".ings__qty").dataset.portions||document.getElementById("portions-out")&&document.getElementById("portions-out").textContent||1);
          opts.forEach(function(o){var it=document.createElement("button");it.type="button";it.className="subs__opt";it.setAttribute("role","menuitem");
            var k=Math.round(o.kcalDelta),c=o.costDelta;var d=(k?(k>0?"+":"−")+Math.abs(k)+" "+(ingsEl.dataset.kcal||"ккал"):"")+(c?(k?" · ":"")+(c>0?"+":"−")+Math.abs(c).toLocaleString()+(o.symbol?" "+o.symbol:""):"");
            it.innerHTML="<b>"+esc(o.name)+"</b><span class=\"num\">"+esc(fmtQty(o.amount*pos,o.unit))+"</span>"+(o.note?"<small>"+esc(o.note)+"</small>":"")+(d?"<small class=\"subs__delta\">"+esc(d)+"</small>":"");
            it.addEventListener("click",function(){var nm=row.querySelector(".ings__name"),q=row.querySelector(".ings__qty");if(!row.dataset.orig){row.dataset.orig=nm.firstChild.textContent;row.dataset.origAmt=q.dataset.amount;row.dataset.origUnit=q.dataset.unit}
              nm.firstChild.textContent=o.name+" ";q.dataset.amount=o.amount;q.dataset.unit=o.unit;q.textContent=fmtQty(o.amount*pos,o.unit);row.classList.add("is-swapped");
              var back=row.querySelector(".subs__back");if(!back){back=document.createElement("button");back.type="button";back.className="subs__back";back.textContent=ingsEl.dataset.subsBack||"↺";back.addEventListener("click",function(){nm.firstChild.textContent=row.dataset.orig;q.dataset.amount=row.dataset.origAmt;q.dataset.unit=row.dataset.origUnit;q.textContent=fmtQty(Number(row.dataset.origAmt)*pos,row.dataset.origUnit);row.classList.remove("is-swapped");back.remove()});nm.appendChild(back)}
              pop.remove()});
            pop.appendChild(it)});
          row.appendChild(pop);
          setTimeout(function(){document.addEventListener("click",function close(e){if(!pop.contains(e.target)&&!b.contains(e.target)){pop.remove();document.removeEventListener("click",close)}})},0)})})})}
})();

// Каталог без перезагрузки: клик по стране, фильтру, странице и поиск подменяют <main> ответом сервера
// Метрика подключена inline в layout.html; живой каталог меняет адрес через pushState — сообщаем о просмотре
(function(){
  if(!window.ym)return;var id=112818312;
  var push=history.pushState;history.pushState=function(){var r=push.apply(this,arguments);try{window.ym(id,"hit",location.href)}catch(e){}return r};
})();
// (тот же HTML), адрес меняется через pushState, «назад» работает. Фильтры остаются раскрытыми.
(function(){
  var main=document.querySelector("main.pages");if(!main||!main.querySelector(".catside"))return;
  var busy=null,sideOpen=false;
  // на телефоне боковая панель фильтров — экран поверх каталога; на широком экране она всегда слева
  function openSide(on){sideOpen=on;var s=main.querySelector(".catside");if(s)s.classList.toggle("is-open",on);document.body.classList.toggle("catside-open",on&&window.innerWidth<900)}
  main.addEventListener("click",function(e){var o=e.target.closest("[data-filters-open]"),c=e.target.closest("[data-filters-close]");if(o){openSide(true);var f=main.querySelector(".catside a, .catside summary");if(f)f.focus()}if(c)openSide(false)});
  document.addEventListener("keydown",function(e){if(e.key==="Escape"&&sideOpen)openSide(false)});
  function load(url,push,scrollTo){
    if(busy)busy.abort();busy=new AbortController();
    main.classList.add("is-loading");
    var wasOpen=!!(main.querySelector(".catside__country")||{}).open;
    // не прыгать: помним прокрутку страницы и панели фильтров (на телефоне она скроллится сама)
    var side=main.querySelector(".catside"),sideTop=side?side.scrollTop:0,pageY=window.scrollY;
    fetch(url,{signal:busy.signal,headers:{"Accept":"text/html"},credentials:"same-origin"}).then(function(r){return r.text()}).then(function(html){
      var doc=new DOMParser().parseFromString(html,"text/html"),next=doc.querySelector("main.pages");
      if(!next){location.href=url;return}
      // панель фильтров и колонку результатов меняем по содержимому, а не элементами: панель не моргает
      // (класс is-open остаётся), её прокрутка и прокрутка страницы на месте, фокус не теряется зря
      var ns=next.querySelector(".catside"),nm=next.querySelector(".catmain");
      if(side&&ns&&nm&&main.querySelector(".catmain")){side.innerHTML=ns.innerHTML;main.querySelector(".catmain").innerHTML=nm.innerHTML;var nc=next.querySelector(".colstrip"),oc=main.querySelector(".colstrip");if(oc&&nc)oc.innerHTML=nc.innerHTML;else if(oc&&!nc)oc.remove()}
      else main.innerHTML=next.innerHTML;
      document.title=doc.title;
      var cd=main.querySelector(".catside__country");if(cd&&wasOpen)cd.open=true;
      var s2=main.querySelector(".catside");if(s2)s2.scrollTop=sideTop;
      if(sideOpen)openSide(true);
      if(!scrollTo)window.scrollTo(0,Math.min(pageY,document.documentElement.scrollHeight-window.innerHeight));
      if(push)history.pushState({catalog:1},"",url);
      main.classList.remove("is-loading");busy=null;
      if(scrollTo){var el=main.querySelector(scrollTo);if(el)el.scrollIntoView({block:"start"})}
    }).catch(function(e){if(e.name!=="AbortError")location.href=url});
  }
  main.addEventListener("click",function(e){
    var a=e.target.closest("a");if(!a||!main.contains(a))return;
    if(a.classList.contains("pages__country")){var cd=main.querySelector(".catside__country");if(cd)cd.open=true;openSide(true);if(window.innerWidth>=900)return;e.preventDefault();return}
    var inFilters=a.closest(".filters")||a.closest(".catside"),inPager=a.closest(".pager"),inTabs=a.closest(".cattabs");
    if(!inFilters&&!inPager&&!inTabs)return;
    if(e.metaKey||e.ctrlKey||e.shiftKey||a.target==="_blank")return;
    e.preventDefault();
    load(a.href,true,inPager?".rgrid":null);
  });
  main.addEventListener("submit",function(e){
    var f=e.target;if(!f.classList.contains("pages__search"))return;
    e.preventDefault();
    var q=new URLSearchParams(new FormData(f)).toString();
    load(f.getAttribute("action")+(q?"?"+q:""),true,".rgrid");
  });
  window.addEventListener("popstate",function(){load(location.href,false,null)});
  history.replaceState({catalog:1},"",location.href);
})();

// Фото продукта по иконке: картинка в body с position: fixed — справа от иконки, не влезает — слева;
// по вертикали по центру иконки, прижата к краям окна. На тач — по нажатию.
(function(){
  var SIZE=160,GAP=10,img=null,cur=null;
  function show(b){var r=b.getBoundingClientRect(),x=r.right+GAP;if(x+SIZE>window.innerWidth-8)x=r.left-GAP-SIZE;if(x<8)x=8;
    var y=Math.min(Math.max(8,r.top+r.height/2-SIZE/2),window.innerHeight-SIZE-8);
    if(!img){img=document.createElement("img");img.className="pic__float";img.alt="";img.width=SIZE;img.height=SIZE;document.body.appendChild(img)}
    img.src=b.dataset.pic;img.style.left=x+"px";img.style.top=y+"px";img.style.display="block";cur=b;b.setAttribute("aria-expanded","true")}
  function hide(){if(img)img.style.display="none";if(cur)cur.setAttribute("aria-expanded","false");cur=null}
  var near=function(e){return e.target&&e.target.closest?e.target.closest(".pic__btn"):null};
  document.addEventListener("mouseover",function(e){var b=near(e);if(b)show(b)});
  document.addEventListener("mouseout",function(e){var b=near(e);if(b&&!b.contains(e.relatedTarget))hide()});
  document.addEventListener("focusin",function(e){var b=near(e);if(b)show(b)});
  document.addEventListener("focusout",function(e){var b=near(e);if(b)hide()});
  document.addEventListener("click",function(e){var b=near(e);if(!b)return;e.preventDefault();e.stopPropagation();if(cur===b&&img&&img.style.display!=="none")hide();else show(b)});
  window.addEventListener("scroll",function(){if(cur)show(cur)},{passive:true});
})();
