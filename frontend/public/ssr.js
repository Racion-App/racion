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
  // страна и валюта: тот же лист; ссылка ?country=XX сохраняет остальные параметры и сбрасывает страницу
  var cd=document.querySelector(".countrydlg"),cb=document.querySelector("[data-country-btn]");
  if(cd&&cb){cb.addEventListener("click",function(){cd.showModal()});var cc=cd.querySelector("[data-country-close]");if(cc)cc.addEventListener("click",function(){cd.close()});cd.addEventListener("click",function(e){if(e.target===cd)cd.close()});
    cd.addEventListener("click",function(e){var a=e.target.closest("[data-country]");if(!a)return;e.preventDefault();var u=new URL(location.href);u.searchParams.set("country",a.dataset.country);u.searchParams.delete("p");location.href=u.toString()})}
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
  var rating=box.querySelector('[data-rating]');
  if(rating){var stars=rating.querySelectorAll('[data-star]'),meta=rating.querySelector('[data-rating-meta]');
    function paintStars(n){stars.forEach(function(b){b.classList.toggle('is-on',+b.dataset.star<=n)})}
    function plural(n){var m=n%10,h=n%100;if(rating.dataset.lFew&&m>=2&&m<=4&&(h<10||h>=20))return rating.dataset.lFew;if(m===1&&h!==11)return rating.dataset.lOne;return rating.dataset.lMany}
    stars.forEach(function(b){b.addEventListener('mouseenter',function(){paintStars(+b.dataset.star)});b.addEventListener('mouseleave',function(){paintStars(+rating.querySelector('.rating__stars').dataset.my)});
      b.addEventListener('click',function(){var n=+b.dataset.star;api('POST','/api/recipes/'+id+'/rating',{stars:n}).then(function(st){rating.querySelector('.rating__stars').dataset.my=st.myRating||Math.round(st.rating);paintStars(+rating.querySelector('.rating__stars').dataset.my);meta.textContent=st.rating.toFixed(1)+' · '+st.ratings+' '+plural(st.ratings);toast(rating.dataset.lThanks)}).catch(function(e){toast(e.message)})})});}
  box.querySelector('[data-share]').addEventListener('click',function(){var url=location.origin+location.pathname;if(navigator.share){navigator.share({title:document.title,url:url}).catch(function(){});return}navigator.clipboard.writeText(url).then(function(){toast(L.lCopied)})});
  var form=document.querySelector('[data-comment-form]'),list=document.querySelector('[data-comments]');
  function esc(s){return s.replace(/[&<>"]/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]})}
  function row(c){var li=document.createElement('li');li.className='comment';li.dataset.id=c.id;li.innerHTML='<div class="comment__head">'+(c.avatar?'<img class="comment__ava" src="'+esc(c.avatar)+'" alt="">':'')+'<b>@'+esc(c.nick)+'</b><time class="comment__time"></time>'+(c.mine?'<button type="button" class="comment__del" data-del aria-label="×">×</button>':'')+'</div><div class="comment__body">'+(c.html||esc(c.body))+'</div>'+(c.image?'<img class="comment__img" src="'+esc(c.image)+'" alt="">':'');return li}
  // панель форматирования живёт в mdedit.js (contenteditable), textarea здесь только хранилище Markdown
  // фото к отзыву: грузим сразу при выборе, в комментарий уходит ссылка
  // внешняя ссылка из комментария: сначала предупреждение, куда ведёт, потом переход в новой вкладке
  var leave=document.querySelector('[data-leave]');
  if(leave){document.addEventListener('click',function(e){var a=e.target.closest('.comment__body a[href]');if(!a)return;var u;try{u=new URL(a.href)}catch(x){return}
      if(u.host===location.host||!/^https?:$/.test(u.protocol))return;e.preventDefault();
      leave.querySelector('[data-leave-host]').textContent=u.host;leave.querySelector('[data-leave-url]').textContent=u.href.length>90?u.href.slice(0,87)+'…':u.href;leave.querySelector('[data-leave-go]').href=u.href;leave.showModal()});
    leave.querySelectorAll('[data-leave-no]').forEach(function(b){b.addEventListener('click',function(){leave.close()})});
    leave.querySelector('[data-leave-go]').addEventListener('click',function(){leave.close()});
    leave.addEventListener('click',function(e){if(e.target===leave)leave.close()})}
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
// «Поделиться» на страницах подборок: системный диалог или копирование адреса
(function(){
  var b=document.querySelector("[data-share-page]");if(!b)return;
  b.addEventListener("click",function(){var url=location.origin+location.pathname;if(navigator.share){navigator.share({title:document.title,url:url}).catch(function(){});return}
    navigator.clipboard.writeText(url).then(function(){b.classList.add("is-done");setTimeout(function(){b.classList.remove("is-done")},1500)})});
})();
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
      initSlider();
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
  // ползунок цены: подпись меняется на ходу, запрос уходит при отпускании; крайнее правое = без ограничения
  // двойной ползунок цены «от — до»: бегунки не пересекаются, заливка между ними, подпись на ходу,
  // запрос при отпускании; крайние положения = без ограничения (параметры pmin/price снимаются)
  function money(v,sym,dec){var s=dec>0?v.toFixed(dec).replace(".",","):String(Math.round(v));return sym==="$"||sym==="£"?sym+s:s+" "+sym}
  function sliderState(box){
    var lo=box.querySelector("[data-price-min]"),hi=box.querySelector("[data-price-max]"),min=Number(box.dataset.min),max=Number(box.dataset.max);
    var a=Number(lo.value),b=Number(hi.value);if(a>b){var t=a;a=b;b=t;lo.value=a;hi.value=b}
    return {lo:lo,hi:hi,a:a,b:b,min:min,max:max}
  }
  function paintSlider(box){
    var s=sliderState(box),fill=box.querySelector("[data-price-fill]"),out=main.querySelector("[data-price-out]"),sym=box.dataset.symbol,dec=Number(box.dataset.decimals);
    var p=function(v){return (v-s.min)/(s.max-s.min)*100};
    if(fill){fill.style.left=p(s.a)+"%";fill.style.right=(100-p(s.b))+"%"}
    // верхний бегунок перекрывает нижний у правого края: тот, что ближе к середине, ловит указатель
    s.lo.style.zIndex=s.a>s.min+(s.max-s.min)*0.9?3:2;s.hi.style.zIndex=2;
    if(!out)return;
    var fromOn=s.a>s.min,toOn=s.b<s.max,m=function(v){return money(v,sym,dec)};
    out.textContent=!fromOn&&!toOn?out.dataset.any:fromOn&&toOn?out.dataset.range.replace("{0}",m(s.a)).replace("{1}",m(s.b)):fromOn?out.dataset.from.replace("{0}",m(s.a)):out.dataset.upto.replace("{0}",m(s.b));
  }
  function initSlider(){var box=main.querySelector("[data-price-slider]");if(box)paintSlider(box)}
  initSlider();
  main.addEventListener("input",function(e){var box=e.target.closest("[data-price-slider]");if(box)paintSlider(box)});
  main.addEventListener("change",function(e){
    var box=e.target.closest("[data-price-slider]");if(!box)return;
    var s=sliderState(box),u=new URL(location.href);
    if(s.a>s.min)u.searchParams.set("pmin",String(s.a));else u.searchParams.delete("pmin");
    if(s.b<s.max)u.searchParams.set("price",String(s.b));else u.searchParams.delete("price");
    u.searchParams.delete("p");
    load(u.toString(),true,null);
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
  var SIZE=160,GAP=10,img=null,cur=null,by=null,shownAt=0; // by: чем открыли (hover|focus|click) — тап на телефоне даёт mouseover и focus раньше click
  function show(b){var r=b.getBoundingClientRect(),x=r.right+GAP;if(x+SIZE>window.innerWidth-8)x=r.left-GAP-SIZE;if(x<8)x=8;
    var y=Math.min(Math.max(8,r.top+r.height/2-SIZE/2),window.innerHeight-SIZE-8);
    if(!img){img=document.createElement("img");img.className="pic__float";img.alt="";img.width=SIZE;img.height=SIZE;img.setAttribute("popover","manual");document.body.appendChild(img)}
    img.src=b.dataset.pic;img.style.left=x+"px";img.style.top=y+"px";img.style.display="";try{if(!img.matches(":popover-open"))img.showPopover()}catch(e){img.style.display="block"}cur=b;shownAt=Date.now();b.setAttribute("aria-expanded","true")}
  function open(b,how){if(cur!==b||by!=="click")by=how;show(b)}
  function hide(){by=null;if(img){try{img.hidePopover()}catch(e){}img.style.display="none"}if(cur)cur.setAttribute("aria-expanded","false");cur=null}
  var near=function(e){var b=e.target&&e.target.closest?e.target.closest(".pic__btn"):null;return b&&!b.disabled?b:null};
  // фото не загрузилось (нет сети): вместо «сломанной картинки» браузера — значок, кнопка выключена
  function broken(t){var b=t.closest(".pic__btn");if(!b||b.disabled)return;
    b.classList.add("is-broken");b.disabled=true;b.innerHTML='<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>'}
  document.addEventListener("error",function(e){var t=e.target;if(t&&t.classList&&t.classList.contains("pic__thumb"))broken(t)},true);
  Array.prototype.forEach.call(document.querySelectorAll(".pic__thumb"),function(t){if(t.complete&&t.naturalWidth===0)broken(t)});
  document.addEventListener("mouseover",function(e){var b=near(e);if(b)open(b,"hover")});
  document.addEventListener("mouseout",function(e){var b=near(e);if(b&&!b.contains(e.relatedTarget)&&by==="hover")hide()});
  document.addEventListener("focusin",function(e){var b=near(e);if(b)open(b,"focus")});
  document.addEventListener("focusout",function(e){var b=near(e);if(b)hide()});
  document.addEventListener("click",function(e){var b=near(e);if(!b)return;e.preventDefault();e.stopPropagation();if(cur===b&&by==="click")hide();else open(b,"click")});
  // прокрутка или касание вне кнопки закрывают фото: на телефоне фокус с кнопки сам не уходит
  // фокус на кнопке сам чуть прокручивает страницу — такую прокрутку сразу после показа не считаем
  document.addEventListener("scroll",function(){if(cur&&Date.now()-shownAt>400)hide()},{passive:true,capture:true});
  document.addEventListener("touchstart",function(e){if(cur&&!cur.contains(e.target))hide()},{passive:true,capture:true});
  document.addEventListener("pointerdown",function(e){if(cur&&!cur.contains(e.target))hide()},{passive:true,capture:true});
})();
