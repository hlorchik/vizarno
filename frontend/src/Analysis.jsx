import React, {useEffect, useState} from 'react';
import CardGenerator from './CardGenerator.jsx';

const statusText = {ok:'Проверка пройдена', fix:'Нужна правка', unknown:'Недостаточно данных'};
const order = {fix:0, unknown:1, ok:2};

export default function Analysis({cardId, profile, version, previous, request, panel}) {
  const [preview,setPreview]=useState(null), [enhanced,setEnhanced]=useState(null);
  const [selection,setSelection]=useState(null), [providers,setProviders]=useState([]), [provider,setProvider]=useState('');
  const [loading,setLoading]=useState(true), [busy,setBusy]=useState(false), [error,setError]=useState(''), [notice,setNotice]=useState('');
  const [retry,setRetry]=useState(0);
  const [excluded,setExcluded]=useState([]);
  const root='cards/'+cardId+'/';
  const query=new URLSearchParams({version:String(version?.id || 0)});
  if(selection!==null) query.set('tasks',selection.join(','));
  const params=query.toString();
  useEffect(()=>{
    let active=true;
    setLoading(true);setError('');
    request(root+'designer-brief?format=preview&'+params).then(data=>{
      if(active){setPreview(data);setEnhanced(data.saved);setExcluded([])}
    }).catch(e=>{if(active){setPreview(null);setError(e.message)}})
      .finally(()=>{if(active)setLoading(false)});
    return ()=>{active=false};
  },[root,params,request,retry]);
  useEffect(()=>{
    let active=true;
    request('ai/providers').then(list=>{
      if(active){setProviders(list);setProvider(list.find(p=>p.enabled)?.id || '')}
    }).catch(e=>{if(active)setError(e.message)});
    return ()=>{active=false};
  },[request,retry]);
  const chosen=selection ?? (preview?.plan.tasks || []).map(f=>f.id);
  function toggle(id){
    setLoading(true);
    setSelection(chosen.includes(id)?chosen.filter(x=>x!==id):[...chosen,id]);
  }
  async function enhance(){
    setBusy(true);setError('');setNotice('');
    try{
      const target=new URLSearchParams({provider,version:String(version.id),tasks:chosen.join(',')});
      setEnhanced(await request(root+'ai-brief?'+target,{method:'POST'}));
      setExcluded([]);setNotice('Визуальный разбор добавлен в ТЗ. Результат расположен выше в разделе проверок по методике психолога.');
    }catch(e){setError(e.message)}finally{setBusy(false)}
  }
  const current=enhanced || preview;
  const insights=enhanced?.review.insights || [];
  const selectedInsights=insights.filter((_,i)=>!excluded.includes(i));
  const text=current?.text + (selectedInsights.length?'\nВыбранные замечания ИИ\n'+selectedInsights.map(x=>
    x.source+' · '+x.rule+'\nОснование: '+x.evidence+'\nДля покупателя: '+x.why+'\nПравка: '+x.action+'\nПроверка: '+x.verify).join('\n\n'):'');
  function download(){
    const url=URL.createObjectURL(new Blob([text],{type:'text/plain;charset=utf-8'}));
    const link=document.createElement('a');link.href=url;link.download='vizarno-'+cardId+'-v'+(version?.id || 0)+'.txt';link.click();
    setTimeout(()=>URL.revokeObjectURL(url),1000);
  }
  const rows=[...(preview?.findings || [])].sort((a,b)=>order[a.status]-order[b.status] || (a.priority===b.priority?0:a.priority==='Высокий'?-1:1));
  const former=previous?.report.findings || [];
  return <section className="analysis">
    {error && <p role="alert">{error} <button type="button" onClick={()=>setRetry(retry+1)}>Обновить</button></p>}
    <div hidden={panel!=='analysis'}><h2>Результат анализа</h2>
    {version && <p>Версия обложки №{version.id}. Локально проверяются переданные тексты, профиль, анкета и размеры изображений. Визуальный разбор запускается отдельно.</p>}
    {version && <Sources version={version}/>}
    {previous && <details><summary>Что изменилось с выбранной версии №{previous.id}</summary>
      {!former.length ? <p>У этой версии нет сохранённых результатов по правилам.</p> :
        <ul>{rows.map(f=>{
          const old=former.find(x=>x.id===f.id);
          return <li key={f.id}>{f.source} · {f.rule} — {old ? old.status===f.status ? 'статус без изменений' : statusText[old.status]+' → '+statusText[f.status] : 'новая проверка'}
            {old && (old.status!==f.status || old.evidence!==f.evidence) && <details><summary>Сопоставить основания</summary>
              <p>Было · {old.source}: {old.evidence}</p><p>Сейчас · {f.source}: {f.evidence}</p>
            </details>}
          </li>
        })}{former.filter(f=>!rows.some(n=>n.id===f.id)).map(f=><li key={f.id}>{f.source} · {f.rule} — в новой версии основание для этой проверки отсутствует.</li>)}</ul>}
      <Sources version={previous}/>
    </details>}
    {rows.length>0 && <><p>Отметьте замечания, которые нужно включить в задание дизайнеру.</p>
      {rows.map(f=><details className="finding" key={f.id}><summary><strong>{f.rule}</strong> · {f.source} — {statusText[f.status]}</summary>
        <p><b>Основание:</b> {f.evidence}</p><p>{f.why}</p>
        <p><b>{f.status==='ok'?'Комментарий':'Действие'}:</b> {f.action}</p>
        <p><b>Как проверить:</b> {f.verify}</p>
        {f.status!=='ok' && <label><input type="checkbox" checked={chosen.includes(f.id)} disabled={loading || busy} onChange={()=>toggle(f.id)}/> {f.status==='unknown'?'Включить в вопросы продавцу':'Включить в правки дизайнеру'} · приоритет {f.priority.toLowerCase()}</label>}
      </details>)}</>}
    {insights.length>0 && <section><h3>Проверки по методике психолога · {enhanced.provider}</h3>
      {enhanced.review.omittedInsights>0&&<p>Замечаний без подтверждённого источника исключено: {enhanced.review.omittedInsights}. Они не включены в ТЗ.</p>}
      {insights.map((x,i)=><details className="finding" key={i}><summary>{x.source} · {x.rule}</summary>
        <p><b>Основание:</b> {x.evidence}</p><p>{x.why}</p><p>{x.action}</p><p><b>Как проверить:</b> {x.verify}</p>
        <label><input type="checkbox" checked={!excluded.includes(i)} onChange={e=>setExcluded(e.target.checked?excluded.filter(n=>n!==i):[...excluded,i])}/> Включить замечание ИИ в ТЗ</label>
      </details>)}</section>}
    <h3>Задание дизайнеру</h3>
    {loading && <p role="status">Готовим ТЗ…</p>}
    {current && !loading && <><button onClick={download} disabled={busy}>Скачать ТЗ (.txt)</button>
      <p>{enhanced?'С визуальным разбором · '+enhanced.provider+' · '+enhanced.model:'По правилам мастерской'} · выбрано правок: {rows.filter(f=>f.status==='fix' && chosen.includes(f.id)).length+selectedInsights.length} · вопросов продавцу: {rows.filter(f=>f.status==='unknown' && chosen.includes(f.id)).length}</p>
      <details><summary>Посмотреть текст ТЗ</summary><pre>{text}</pre></details></>}
    {version && preview && <><label>Нейросеть для визуального разбора<select value={provider} disabled={busy} onChange={e=>setProvider(e.target.value)}>
      {!provider && <option value="">Нет настроенных ключей</option>}{providers.map(p=><option value={p.id} key={p.id} disabled={!p.enabled}>{p.label} · {p.model}{p.enabled?'':' — нужен ключ'}</option>)}</select></label>
      <p>Выбранный сервис получит план, тексты, обложку и загруженные слайды.{provider==='gemini' && ' На бесплатном тарифе Google может использовать материалы для улучшения продуктов.'}</p>
      {!provider&&<p className="blocked-action" role="status"><strong>Визуальный разбор недоступен.</strong> На сервере не настроен API-ключ сервиса ИИ.</p>}
      <div className="analysis-actions"><button type="button" disabled={loading || busy || !provider} title={!provider?'На сервере не настроен API-ключ сервиса ИИ':'Проверить изображения и дополнить ТЗ'} onClick={enhance}>{busy?'Анализируем…':enhanced?'Обновить визуальный разбор':'Дополнить ТЗ визуальным разбором'}</button>
        <a className="next-step" href={'#workshop/'+cardId+'/generate'}>Перейти к генерации →</a></div>
      {notice&&<p role="status" className="success-notice">{notice}</p>}
      {enhanced?.review && <details open><summary>Наблюдения ИИ · {enhanced.provider}</summary><p><b>Обложка:</b> {enhanced.review.observation}</p>
        {enhanced.review.gallery?.map((note,i)=><p key={i}><b>Слайд {i+2}:</b> {note}</p>)}</details>}
    </>}
    {version&&!preview&&<a className="next-step" href={'#workshop/'+cardId+'/generate'}>Перейти к генерации →</a>}</div>
    <div hidden={panel!=='generate'}>
      {!version&&<p>Для генерации загрузите фото товара в разделе <a href={'#workshop/'+cardId+'/materials'}>«Материалы»</a>.</p>}
      {loading&&version&&<p role="status">Готовим план слайдов…</p>}
      {version && current && !loading && <CardGenerator key={JSON.stringify(current.plan)+JSON.stringify(enhanced?.review?.designs)} plan={current.plan} review={enhanced?.review} profile={profile} version={version} request={request} providers={providers} provider={provider} setProvider={setProvider}/>}
    </div>
  </section>
}

function Sources({version}){
  const m=version.report.materials || {};
  const url='/api/covers/'+version.id;
  return <details><summary>Открыть материалы версии</summary>
    <p><b>Заголовок предложения:</b> {m.headline || 'не передан'}</p>
    <p><b>Описание:</b> {m.description || 'не передано'}</p>
    <div className="sources"><figure><a href={url} target="_blank" rel="noreferrer"><img src={url} alt="Исходная обложка"/></a><figcaption>Обложка</figcaption></figure>
    {m.slides?.map((s,i)=><figure key={i}><a href={url+'/slides/'+(i+2)} target="_blank" rel="noreferrer"><img src={url+'/slides/'+(i+2)} alt={'Слайд '+(i+2)}/></a><figcaption>Слайд {i+2}{s.caption && ' · '+s.caption}</figcaption></figure>)}</div>
  </details>
}

