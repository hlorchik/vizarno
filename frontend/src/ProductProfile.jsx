import React, {useState} from 'react';

const fields = {model:'Модель',sku:'Артикул производителя',cpu:'Процессор',ram:'Оперативная память',ssd:'SSD',os:'Операционная система',display:'Экран',graphics:'Графика',weight:'Вес',battery:'Автономность',charging:'Зарядка',ports:'Порты',keyboard:'Клавиатура',camera:'Камера',body:'Корпус',package:'Комплектация'};
export default function ProductProfile({card, save}){
  const [draft,setDraft]=useState(card?.profile || {});
  const [title,setTitle]=useState(''),[busy,setBusy]=useState(false),[message,setMessage]=useState('');
  function edit(key,prop,text){
    setMessage('');
    setDraft(old=>({...old,[key]:{...old[key],[prop]:text}}));
  }
  async function submit(e){
    e.preventDefault();if(busy)return;setBusy(true);setMessage('');
    try{
      const profile=Object.fromEntries(Object.entries(draft).filter(([,f])=>Object.values(f).some(v=>typeof v==='string' && v.trim())));
      await save({title,profile});setMessage('Профиль сохранён.');
    }catch(err){setMessage(err.message)}finally{setBusy(false)}
  }
  return <section className="profile"><h2>{card?'Профиль ноутбука':'Создать товар без обложки'}</h2>
    <form onSubmit={submit} aria-busy={busy}>{!card&&<label>Название товара<input value={title} onChange={e=>setTitle(e.target.value)} required maxLength={150}/></label>}
    <p>Оставляйте неизвестные свойства пустыми. Для ОС значение «без ОС» означает, что источник прямо указывает на отсутствие системы.</p>
    <div className="profile-fields">{Object.entries(fields).map(([key,label])=>{
      const f=draft[key]||{};
      return <fieldset key={key}><legend>{label}</legend>
        <label>Значение<input value={f.value||''} onChange={e=>edit(key,'value',e.target.value)} maxLength={250}/></label>
        <label>Источник<input placeholder="Например, заголовок предложения" value={f.source||''} onChange={e=>edit(key,'source',e.target.value)} maxLength={500}/></label>
        <details><summary>Сведения из другого источника</summary>
          <label>Второе значение<input value={f.alternative||''} onChange={e=>edit(key,'alternative',e.target.value)} maxLength={250}/></label>
          <label>Второй источник<input value={f.otherSource||''} onChange={e=>edit(key,'otherSource',e.target.value)} maxLength={500}/></label>
        </details></fieldset>
    })}</div>
    <h2>Сводка для генерации</h2><ul>{Object.entries(fields).map(([key,label])=>{
      const f=draft[key];let state='Нет сведений';
      if(f && Object.values(f).some(v=>v.trim())){
        if(!f.value?.trim()||!f.source?.trim()||Boolean(f.alternative?.trim())!==Boolean(f.otherSource?.trim()))state='Дополните значение и источник';
        else if(f.alternative?.trim() && f.value.trim().toLowerCase()!==f.alternative.trim().toLowerCase())state='Источники расходятся — уточнить до генерации';
        else state=f.value.trim()+' · '+f.source.trim();
      }
      return <li key={key}><strong>{label}</strong> — {state}</li>;
    })}</ul><button type="submit" disabled={busy}>{busy?'Сохраняем профиль…':'Сохранить профиль'}</button><p role="status" aria-live="polite">{message}</p>
    </form></section>
}
