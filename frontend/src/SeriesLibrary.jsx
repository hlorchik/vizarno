import React, {useEffect, useState} from 'react';
import MethodReport from './MethodReport.jsx';

const date = value => new Date(value).toLocaleString('ru-RU', {dateStyle:'medium',timeStyle:'short'});
const image = (id, page) => '/api/renders/'+id+'/pages/'+page;

export default function SeriesLibrary({request, id}) {
  const [cursors,setCursors]=useState([0]);
  const [page,setPage]=useState(1);
  const [result,setResult]=useState(null);
  const [error,setError]=useState('');
  const [attempt,setAttempt]=useState(0);
  const cursor=cursors.at(-1);
  const path=id?'account/series/'+id:'account/series'+(cursor?'?before='+cursor:'');
  useEffect(()=>{
    let active=true;
    setError('');setResult(null);setPage(1);
    request(path).then(data=>{if(active)setResult({path,data})})
      .catch(e=>{if(active)setError(e.message)});
    return ()=>{active=false};
  },[path,request,attempt]);
  const data=result?.path===path?result.data:null;
  return <section className="library">
    {id && <a className="back-link" href="#account">← Все серии</a>}
    {error ? <p role="alert">{error} <button onClick={()=>setAttempt(n=>n+1)}>Повторить</button></p>
    : !data ? <p role="status">Загружаем сери{ id?'ю':'и'}…</p>
    : id ? <>
      <h2>{data.title}</h2>
      <p className="muted">Серия №{data.id} · {date(data.created)}</p>
      {data.stale && <p>После сохранения этой серии профиль или исходная обложка изменились.</p>}
      {data.methodReport?.kind==='ai'&&<MethodReport report={data.methodReport}/>} 
      <div className="series-viewer">
        <a className="slide-stage" href={image(data.id,page)} target="_blank" rel="noreferrer" aria-label="Открыть PNG в полном размере">
          <img src={image(data.id,page)} alt={'Слайд '+page+' из '+data.pages}/>
        </a>
        <div className="slide-tools">
          <h3>Слайды серии</h3>
          <div className="slide-picker">{Array.from({length:data.pages},(_,index)=>{
            const number=index+1;
            return <button key={number} type="button" aria-label={'Показать слайд '+number} aria-pressed={number===page} onClick={()=>setPage(number)}>
              <img src={image(data.id,number)} alt="" loading="lazy"/><span>{number}</span>
            </button>
          })}</div>
          <p>Слайд {page} из {data.pages}</p>
          <a className="primary-link" href={image(data.id,page)+'?download=1'}>Скачать слайд {page} · PNG</a>
          <a href={'#workshop/'+data.cardId+'/profile'}>Открыть товар в мастерской</a>
        </div>
      </div>
    </> : <>
      <h2>Сохранённые серии</h2>
      {data.items.length===0 ? <div className="empty-library"><p>Здесь появятся серии, которые вы сохраните после генерации.</p><a href="#workshop">Перейти в мастерскую</a></div> :
      <div className="series-grid">{data.items.map(s=><a href={'#account/series/'+s.id} className="series-tile" key={s.id}>
        <img src={image(s.id,1)} alt="" loading="lazy"/>
        <div><strong>{s.title}</strong><span>Серия №{s.id} · Слайдов: {s.pages}</span><time dateTime={s.created}>{date(s.created)}</time></div>
      </a>)}</div>}
      {(cursor!==0||data.next!==0)&&<div className="pager">
        <button disabled={!cursor} onClick={()=>setCursors(old=>old.slice(0,-1))}>Предыдущие серии</button>
        <button disabled={!data.next} onClick={()=>setCursors(old=>[...old,data.next])}>Следующие серии</button>
      </div>}
    </>}
  </section>
}
