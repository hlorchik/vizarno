import Analysis from './Analysis.jsx';
import ProductProfile from './ProductProfile.jsx';
import SeriesLibrary from './SeriesLibrary.jsx';
import React, {useEffect, useState} from 'react';
import {createRoot} from 'react-dom/client';
import './style.css';
import './workspace.css';

async function api(path, options) {
  const controller = new AbortController();
  const timer = setTimeout(()=>controller.abort(), /ai-images/.test(path) ? 900000 : /ai-brief|render-check/.test(path) ? 45000 : 20000);
  try {
    const res = await fetch('/api/' + path, {...options, signal:controller.signal});
    if (res.status === 204) return;
    const body = await res.json().catch(()=>{throw new Error('Сервер вернул непонятный ответ. Повторите запрос.')});
    if (!res.ok) throw Object.assign(new Error(body?.error || 'Сервис недоступен'), {status: res.status});
    return body;
  } catch(err) {
    if(controller.signal.aborted) throw new Error('Сервер не ответил вовремя. Данные формы сохранены на экране; попробуйте ещё раз.');
    throw err;
  } finally {clearTimeout(timer)}
}
function Auth({onLogin}) {
  const [signup, setSignup] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  async function submit(e) {
    e.preventDefault(); setBusy(true); setError('');
    const values = Object.fromEntries(new FormData(e.currentTarget));
    try {
      await api(signup ? 'register' : 'login', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(values)});
      await onLogin();
    } catch (err) {setError(err.message)} finally {setBusy(false)}
  }
  return <section className="auth"><div><small>СТУДИЯ КАРТОЧКИ ТОВАРА</small><h1>Обложка<br/>под лупой<span>.</span></h1><p>Посмотрите, как товар выглядит в миниатюре. Проверьте файл и сравните обложку до и после правок.</p></div>
    <form onSubmit={submit}><small>КАБИНЕТ ПРОДАВЦА</small><h2>{signup?'Создать кабинет':'Войти в кабинет'}</h2>
    <label>Электронная почта<input name="Email" type="email" autoComplete="email" required/></label>
    <label>Пароль<input name="Password" type="password" minLength="8" autoComplete={signup?'new-password':'current-password'} required/></label>
    {error && <p role="alert">{error}</p>}<button disabled={busy}>{signup?'Зарегистрироваться':'Войти'}</button>
    <button type="button" className="link" onClick={()=>{setSignup(!signup);setError('')}}>{signup?'У меня есть аккаунт':'Создать аккаунт'}</button></form></section>
}

function PasswordForm({onChanged}) {
  const [error,setError]=useState(""),[busy,setBusy]=useState(false);
  async function submit(e){
    e.preventDefault();
    const data=Object.fromEntries(new FormData(e.currentTarget));
    if(data.New!==data.confirm){setError("Новые пароли не совпадают.");return}
    setBusy(true);setError("");
    try {
      await api("account/password",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({Current:data.Current,New:data.New})});
      onChanged();
    } catch(err){setError(err.message)} finally{setBusy(false)}
  }
  return <details><summary>Сменить пароль</summary><form onSubmit={submit}>
    <label>Текущий пароль<input name="Current" type="password" autoComplete="current-password" required/></label>
    <label>Новый пароль<input name="New" type="password" autoComplete="new-password" minLength={8} required/></label>
    <label>Повторите новый пароль<input name="confirm" type="password" autoComplete="new-password" required/></label>
    <p>После сохранения потребуется снова войти на всех устройствах.</p>
    {error && <p role="alert">{error}</p>}<button disabled={busy}>Сохранить пароль</button>
  </form></details>
}
function Report({version, heading}) {
  const r=version.report;
  return <article className="report"><small>{heading}</small><div className="thumbnail"><img src={'/api/covers/'+version.id} alt="Миниатюра обложки"/></div>
  <p className="muted">{new Date(version.created).toLocaleString('ru-RU')}</p>
  {r.checks.map(c=><section key={c.label}><strong>{c.label}</strong><b className={c.ok?'ok':'fix'}>{c.ok?'В пределах ориентира':'Стоит проверить'}</b><p>{c.note}</p></section>)}</article>
}
function Questionnaire({previous}) {
  const [questions,setQuestions]=useState(null), [error,setError]=useState('');
  useEffect(()=>{
    if(questions) return;
    let active=true;
    setError('');
    api('questions').then(items=>{if(active)setQuestions(items)})
      .catch(err=>{if(active)setError(err.message)});
    return ()=>{active=false};
  },[questions]);
  return <fieldset><legend>Сценарий покупки</legend>
    <input type="hidden" name="questionnaire" value="yes"/>
      <p>Анкету можно сохранять частями. Для генерации заполните все ответы конкретно: вариант «Пока неизвестно» считается вопросом, который ещё нужно уточнить.</p>
      {error && <p role="alert">{error}</p>}
      {!questions && !error && <p>Загружаем вопросы…</p>}
      {questions?.map(q=><label key={q.key}>{q.label}{q.options ?
        <select name={q.key} defaultValue={previous?.[q.key] || ''}>
          <option value="">Выберите ответ</option>{q.options.map(o=><option key={o} value={o}>{o==='Пока неизвестно'?'Пока неизвестно — генерация будет недоступна':o}</option>)}
        </select> :
        <textarea name={q.key} maxLength={400} defaultValue={previous?.[q.key] || ''}/>}</label>)}
  </fieldset>;
}
function Materials({previous}){
  const [files,setFiles]=useState([]);
  return <fieldset><legend>Материалы предложения</legend>
    <label>Заголовок на площадке<input name="headline" maxLength={300} defaultValue={previous?.headline || ''}/></label>
    <label>Описание на площадке<textarea name="description" rows={4} maxLength={6000} defaultValue={previous?.description || ''}/></label>
    <label>Слайды после обложки (до четырёх)<input type="file" name="gallery" accept="image/jpeg,image/png" multiple onChange={e=>setFiles(Array.from(e.target.files))}/></label>
    <p className="muted">Для каждой версии приложите её полный набор изображений. Загружайте отдельно фото товара и слайды. Заголовок рядом с товаром на странице укажите в поле выше.</p>
    {files.map((file,i)=><label key={i+'-'+file.name}>Слайд {i+2} · {file.name}: текст на изображении (по желанию)<input name="caption" maxLength={600}/></label>)}
  </fieldset>
}
const panels={profile:'Профиль',materials:'Материалы',analysis:'Анализ и ТЗ',generate:'Генерация'};
const workshopPath=(id,panel)=>'#workshop/'+(id||'new')+'/'+panel;
const requiredQuestionnaireFields=['buyer','occasion','priority','channel','visual','task','concern','barriers','keyCriteria','motive','message','basis','benefitField','advantage','benefit','proof','endUser','experience'];
function questionnaireReady(version){
  const answers=version?.report?.brief?.answers;
  return Boolean(answers)&&requiredQuestionnaireFields.every(key=>String(answers[key]||'').trim()&&answers[key]!=='Пока неизвестно');
}

function Workspace({cards, refresh, updateCard, route}) {
  const [,cardPart,panelPart]=route.split('/');
  const selected=Number(cardPart)||null;
  const panel=Object.hasOwn(panels,panelPart)?panelPart:'materials';
  const [versions,setVersions]=useState([]),[older,setOlder]=useState(null);
  const [revision,setRevision]=useState(0),[busy,setBusy]=useState(false),[error,setError]=useState('');
  const [loaded,setLoaded]=useState(false);
  useEffect(()=>{
    let active=true;setLoaded(false);setVersions([]);setOlder(null);setError('');
    if(selected) api('cards/'+selected+'/versions').then(v=>{
      if(active){setVersions(v);setOlder(v[1]?.id);setLoaded(true)}
    }).catch(e=>{if(active)setError(e.message)});
    return ()=>{active=false};
  },[selected,revision]);
  async function upload(e){
    e.preventDefault();const form=e.currentTarget, data=new FormData(form);setError('');
    if(data.has('questionnaire') && !data.has('buyer')){setError('Дождитесь загрузки вопросов.');return}
    if(data.get('cover')?.size>8*1024*1024){setError('Выберите файл до 8 МБ.');return}
    const gallery=data.getAll('gallery');
    if(gallery.length>4 || gallery.some(f=>f.size>8*1024*1024)){setError('Добавьте до четырёх слайдов, каждый до 8 МБ.');return}
    setBusy(true);
    try{const result=await api(selected?'cards/'+selected+'/versions':'cards',{method:'POST',body:data});
      form.reset();await refresh();window.location.hash=workshopPath(result.id,'analysis');setRevision(n=>n+1);
    }catch(err){setError(err.message)}finally{setBusy(false)}
  }
  const card=cards.find(c=>c.id===selected);
  const currentReady=questionnaireReady(versions[0]);
  const materialForm=<form key={'upload-'+(selected||'new')} onSubmit={upload}>{!selected&&<label>Название товара<input name="title" placeholder="Например, ноутбук для занятий" required maxLength={150}/></label>}
    <label>{selected?'Новая обложка — только если хотите заменить текущую':'Обложка товара'}<input type="file" name="cover" accept="image/jpeg,image/png" required={!selected || versions.length===0}/></label><p className="muted">Для обновления текста или анкеты оставьте поле пустым — сохранённая обложка будет использована повторно. JPEG или PNG · до 8 МБ · до 16 Мп.</p>
    {(!selected || loaded) && <Materials key={'materials-'+(versions[0]?.id || 'first')} previous={versions[0]?.report.materials}/>}
    {(!selected || loaded) && <Questionnaire key={'questions-'+(versions[0]?.id || 'first')} previous={versions[0]?.report.brief?.answers}/>}
    <button disabled={busy || (selected && !loaded)}>{busy?'Проверяем…':selected?'Сохранить новую версию':'Сохранить и проверить'}</button></form>;
  return <><div className="workspace"><aside><h2>Мои товары</h2><button disabled={busy} onClick={()=>{window.location.hash=workshopPath(null,'materials')}}>+ Новая карточка</button>
    {cards.length===0&&<p className="muted">Здесь появится ваш первый товар.</p>}
    {cards.map(c=><button disabled={busy} className={'item '+(selected===c.id?'active':'')} key={c.id} onClick={()=>{window.location.hash=workshopPath(c.id,'profile')}}>{c.title}</button>)}</aside>
    <main><h1>{card?.title||(selected?'Товар':'Новая карточка')}</h1>
    <nav className="section-nav" aria-label="Разделы товара">{Object.entries(panels).filter(([key])=>selected||key==='materials'||key==='profile').map(([key,label])=><a key={key} aria-current={panel===key?'page':undefined} href={workshopPath(selected,key)}>{label}</a>)}</nav>
    <div hidden={panel!=='profile'}>
    <ProductProfile key={'profile-'+(selected||"draft")} card={card} save={async payload=>{
      const result=await api(selected?"cards/"+selected+"/profile":"products",{method:selected?"PUT":"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
      updateCard({id:result.id,title:card?.title || payload.title,profile:result.profile});window.location.hash=workshopPath(result.id,'profile');
    }}/>
    </div>
    <div hidden={panel!=='materials'}>{!selected ? materialForm : loaded && versions.length>0 ? <section className="current-materials">
      <h2>{currentReady?'Материалы и анкета готовы':'Исходные материалы сохранены'}</h2>
      <p>{currentReady?'Для анализа и генерации используется':'Анкета ещё не заполнена полностью. Дополните её в форме ниже. Сохранённая версия:'} {new Date(versions[0].created).toLocaleString('ru-RU')}.</p>
      {currentReady&&<div className="material-actions"><a className="primary-link" href={workshopPath(selected,'generate')}>Перейти к генерации</a><a href={workshopPath(selected,'analysis')}>Посмотреть анализ и ТЗ</a></div>}
      <details open={!currentReady}><summary>{currentReady?'Обновить обложку, описание или анкету':'Продолжить заполнение анкеты'}</summary><p>{currentReady?'Новая версия нужна только при изменении исходных материалов. Предыдущие версии останутся в истории для сравнения.':'Заполните обязательные вопросы и сохраните новую версию. Обложку повторно загружать не нужно.'}</p>{materialForm}</details>
    </section> : loaded ? materialForm : null}</div>
    {error&&<p role="alert">{error}</p>}{selected&&!loaded&&!error&&<p>Загружаем историю…</p>}
    {card && loaded && <div hidden={panel!=='analysis'&&panel!=='generate'}><Analysis key={card.id+'-'+(versions[0]?.id || 0)+'-'+JSON.stringify(card.profile)} panel={panel} cardId={card.id} profile={card.profile} version={versions[0]} previous={versions.find(v=>v.id===older)} request={api}/></div>}
    {panel==='analysis' && versions.length>0&&<details className="history-panel"><summary>Техническая проверка и сравнение версий</summary>
    {versions.length>1&&<label>С чем сравнить<select value={older||''} onChange={e=>setOlder(Number(e.target.value))}>{versions.slice(1).map((v,i)=><option value={v.id} key={v.id}>Версия {versions.length-i-1} · {new Date(v.created).toLocaleString('ru-RU')}</option>)}</select></label>}
    <div className="comparison"><Report version={versions[0]} heading={'ТЕКУЩАЯ · V'+versions.length}/>{versions.some(v=>v.id===older)&&<Report version={versions.find(v=>v.id===older)} heading="ДЛЯ СРАВНЕНИЯ"/>}</div></details>}
    </main></div></>
}
function Account({finish,route}){
  const [error,setError]=useState(""),[busy,setBusy]=useState(false);
  const [email,setEmail]=useState(""),[profileError,setProfileError]=useState("");
  useEffect(()=>{
    let active=true;
    api("account").then(profile=>{if(active)setEmail(profile.email)})
      .catch(err=>{if(active)setProfileError(err.message)});
    return ()=>{active=false};
  },[]);
  async function remove(e){
    e.preventDefault();setError("");setBusy(true);
    const password=new FormData(e.currentTarget).get("password");
    try{
      await api("account",{method:"DELETE",headers:{"Content-Type":"application/json"},body:JSON.stringify({password})});
      finish("Аккаунт удалён.");
    }catch(err){setError(err.message)}finally{setBusy(false)}
  }
  const settings=route==='#account/settings';
  const seriesId=route.match(/^#account\/series\/(\d+)$/)?.[1];
  return <main className="account"><h1>Личный кабинет</h1>
    {profileError ? <p role="alert">{profileError}</p> :
      <p>Текущий аккаунт: <strong style={{overflowWrap:"anywhere"}}>{email || "Загрузка…"}</strong></p>}
    <nav className="section-nav" aria-label="Разделы кабинета"><a href="#account" aria-current={!settings?'page':undefined}>Мои серии</a><a href="#account/settings" aria-current={settings?'page':undefined}>Настройки</a></nav>
    {settings ? <div className="settings-panel"><PasswordForm onChanged={()=>finish("Пароль изменён. Войдите с новым паролем.")}/>
    <details><summary>Удалить аккаунт</summary><form onSubmit={remove}>
      <p>Все ваши карточки, обложки и отчёты будут удалены без возможности восстановления.</p>
      <label>Текущий пароль<input name="password" type="password" autoComplete="current-password" required/></label>
      <label><input type="checkbox" required/> Подтверждаю удаление аккаунта и всех его данных</label>
      {error && <p role="alert">{error}</p>}
      <button className="danger" disabled={busy}>Удалить аккаунт навсегда</button>
    </form></details></div> : <SeriesLibrary request={api} id={seriesId}/>}</main>
}
function App(){
  const [page,setPage]=useState(window.location.hash);
  useEffect(()=>{const update=()=>setPage(window.location.hash);window.addEventListener("hashchange",update);return ()=>window.removeEventListener("hashchange",update)},[]);
  function finish(message){window.location.hash="";setCards(null);setNotice(message);setError("")}

  const [cards,setCards]=useState(null),[ready,setReady]=useState(false),[error,setError]=useState('');
  const [notice,setNotice]=useState("");
  async function refresh(){const c=await api('cards');setCards(c);setNotice("");setError('')}
  async function start(){try{await refresh()}catch(e){if(e.status!==401)setError(e.message)}finally{setReady(true)}}
  useEffect(()=>{start()},[]);
  const accountPage=page==='#account'||page.startsWith('#account/');
  return <div className="shell"><header><img src="/vizarno.svg" alt="Визарно"/>{cards&&<nav className="top-nav" aria-label="Основная навигация"><a href="#workshop" aria-current={!accountPage?'page':undefined}>Мастерская</a><a href="#account" aria-current={accountPage?'page':undefined}>Личный кабинет</a><button className="link" onClick={async()=>{try{await api("logout",{method:"POST"});finish("")}catch(err){setError(err.message)}}}>Выйти</button></nav>}</header>
    {error&&<p role="alert">{error} <button onClick={start}>Повторить</button></p>}
    {notice && <p role="status">{notice}</p>}
    {!ready?<p>Открываем кабинет…</p>:cards?accountPage?<Account finish={finish} route={page}/>:<Workspace route={page} cards={cards} refresh={refresh} updateCard={card=>setCards(old=>old.some(c=>c.id===card.id)?old.map(c=>c.id===card.id?card:c):[card,...old])}/>:!error&&<Auth onLogin={refresh}/>}</div>
}
createRoot(document.getElementById('root')).render(<App/>);



