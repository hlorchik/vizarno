import React,{useEffect,useState,useRef} from 'react';
import {loadPhoto,renderCard,themeNames} from './renderCards.js';
import CropPreview from './CropPreview.jsx';
import MethodReport from './MethodReport.jsx';

const defaultCrop={x:0,y:0,w:100,h:100};
export default function CardGenerator({plan,review,profile,version,request,providers,provider,setProvider}){
  const [steps,setSteps]=useState(()=>plan.steps.map((s,i)=>({...s,design:review?.designs?.[i]||{layout:i===0?'hero':i===1?'split':'focus',theme:'light',photo:0}})));
  const [photo,setPhoto]=useState('auto'),[local,setLocal]=useState(''),[crop,setCrop]=useState(defaultCrop);
  const [manualCrop,setManualCrop]=useState(false);
  const openAIReady=providers.some(p=>p.id==='openai'&&p.enabled);
  const [generationMode,setGenerationMode]=useState('canvas');
  const [platform,setPlatform]=useState('universal');
  const [method,setMethod]=useState(null);
  const [aiMethod,setAIMethod]=useState(null);
  const [pages,setPages]=useState([]),[saved,setSaved]=useState(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
  const previewURLs=useRef([]),localURL=useRef('');
  const base='/api/covers/'+version.id;
  const previewPhoto=photo==='auto'?steps[0].design.photo:Number(photo);
  const previewSource=photo==='local'?local:previewPhoto>0?base+'/slides/'+(previewPhoto+1):base;
  useEffect(()=>{
    let active=true;
    request('cards/'+plan.cardId+'/renders').then(s=>{if(active)setSaved(s)}).catch(e=>{if(active)setError(e.message)});
    return()=>{active=false;previewURLs.current.forEach(URL.revokeObjectURL);if(localURL.current)URL.revokeObjectURL(localURL.current)};
  },[plan.cardId,request]);
  function invalidate(){previewURLs.current.forEach(URL.revokeObjectURL);previewURLs.current=[];setPages([]);setMethod(null);setAIMethod(null);setNotice('')}
  function payload(images){
    const data=new FormData();data.set('version',version.id);data.set('profile',JSON.stringify(profile));data.set('platform',platform);
    images.forEach((p,i)=>data.append('pages',p.blob,`slide-${i+1}.png`));return data;
  }
  function edit(i,patch){invalidate();setSteps(old=>old.map((s,j)=>j===i?{...s,...patch}:s))}
  function editDesign(i,patch){edit(i,{design:{...steps[i].design,...patch}})}
  function upload(e){
    const f=e.target.files[0];if(!f)return;
    if(!['image/png','image/jpeg'].includes(f.type)||f.size>8*1024*1024){setError('Выберите PNG или JPEG до 8 МБ.');e.target.value='';return}
    invalidate();if(localURL.current)URL.revokeObjectURL(localURL.current);
    localURL.current=URL.createObjectURL(f);setLocal(localURL.current);setPhoto('local');setCrop(defaultCrop);setError('');
  }
  async function generate(){
    const continuing=generationMode==='ai'&&pages.length>0&&pages.length<steps.length;
    const previousPages=continuing?pages:[];
    setBusy(true);setError('');setNotice('');if(!continuing)invalidate();
    const images=new Map();
    try{
      await document.fonts.ready;
      let result=[];
      if(generationMode==='ai'){
        const answer=await request('cards/'+plan.cardId+'/ai-images',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({version:version.id,start:previousPages.length+1,platform})});
        const fresh=await Promise.all(answer.pages.map(async page=>{
          const binary=atob(page.image);const bytes=new Uint8Array(binary.length);
          for(let i=0;i<binary.length;i++)bytes[i]=binary.charCodeAt(i);
          const source=new Blob([bytes],{type:'image/png'});
          const image=await createImageBitmap(source);
          try{
            const canvas=document.createElement('canvas');canvas.width=1200;canvas.height=1600;
            const ctx=canvas.getContext('2d');ctx.fillStyle='#fff';ctx.fillRect(0,0,1200,1600);
            const scale=Math.min(1200/image.width,1600/image.height),w=image.width*scale,h=image.height*scale;
            ctx.drawImage(image,(1200-w)/2,(1600-h)/2,w,h);
            const blob=await new Promise(resolve=>canvas.toBlob(resolve,'image/png'));
            if(!blob)throw new Error('Браузер не смог подготовить PNG нейросети.');
            return {blob,url:URL.createObjectURL(blob)};
          }finally{image.close()}
        }));
        result=[...previousPages,...fresh];
        previewURLs.current=result.map(p=>p.url);setPages(result);
        if(!answer.complete){setError(answer.error||'Генерация остановлена. Нажмите продолжить позже.');return}
      }else{
        for(let i=0;i<steps.length;i++){
          const source=photo==='auto'?steps[i].design.photo:Number(photo);
          const url=photo==='local'?local:source>0?base+'/slides/'+(source+1):base;
          if(!images.has(url))images.set(url,await loadPhoto(url));
          const area=manualCrop?crop:photo==='auto'?(steps[i].design.crop||defaultCrop):defaultCrop;
          const blob=await renderCard(steps[i],images.get(url),area,steps[i].design,i,steps.length,profile,platform);
          result.push({blob,url:URL.createObjectURL(blob)});
        }
      }
      previewURLs.current=result.map(p=>p.url);
      const data=payload(result);data.set('review','local');
      const report=await request('cards/'+plan.cardId+'/render-check',{method:'POST',body:data});
      setPages(result);setMethod(report);
    }catch(e){if(!continuing)invalidate();setError(e.message)}finally{images.forEach(img=>img.close());setBusy(false)}
  }
  async function reviewVisual(){
    setBusy(true);setError('');setNotice('');
    try{
      const data=payload(pages);data.set('review','ai');data.set('provider',provider);
      setAIMethod(await request('cards/'+plan.cardId+'/render-check',{method:'POST',body:data}));
    }catch(e){setError(e.message)}finally{setBusy(false)}
  }
  async function save(){
    setBusy(true);setError('');setNotice('');
    try{
      const data=payload(pages);
      setSaved(await request('cards/'+plan.cardId+'/renders',{method:'POST',body:data}));
      setNotice('Серия сохранена в кабинете. Она будет доступна после перезагрузки.');
    }catch(e){setError(e.message)}finally{setBusy(false)}
  }
  const generationBlocked=plan.releaseNeeds?.length>0
    ? 'Заполните обязательные сведения в материалах и анкете.'
    : '';
  return <section className="generator"><h3>Готовые карточки</h3>
    <p className="muted">PNG · 1200 × 1600 · {generationMode==='ai'?'изображения создаёт OpenAI':'точная сборка браузером'}</p>
    {plan.releaseNeeds?.length>0&&<section><strong>Для генерации нужны сведения</strong><ul>{plan.releaseNeeds.map(s=><li key={s}>{s}</li>)}</ul><a href={'#workshop/'+plan.cardId+'/materials'}>Открыть материалы и анкету</a></section>}
    {!!plan.pending?.length && <details><summary>Что ещё уточнить ({plan.pending.length})</summary><ul>{plan.pending.map((p,i)=><li key={i}>{p}</li>)}</ul></details>}
    <div className="editor-layout"><fieldset disabled={busy}><legend>Фото и оформление</legend>
      <label>Назначение серии<select value={platform} onChange={e=>{invalidate();setPlatform(e.target.value)}}>
        <option value="universal">Универсальная / рекламная</option>
        <option value="wildberries">Wildberries</option>
      </select></label>
      {platform==='wildberries'
        ?<p className="muted">Режим Wildberries убирает с изображения прямой призыв «Покупай!», но сохраняет «Тот самый», выгоды и доказуемые характеристики. Это снижает риск отклонения карточки площадкой.</p>
        :<p className="muted">Универсальный режим использует всю методику психолога, включая прямой призыв «Покупай!» на первой карточке.</p>}
      <label>Способ создания<select value={generationMode} onChange={e=>{invalidate();setGenerationMode(e.target.value)}}>
        <option value="ai" disabled={!openAIReady}>Нейрогенерация OpenAI{openAIReady?'':' — нужен ключ'}</option>
        <option value="canvas">Дизайн-система Визарно · без оплаты API</option>
      </select></label>
      {generationMode==='ai'&&<p className="muted">OpenAI создаст каждый слайд заново по исходному фото и правилам. Это расходует API-кредиты и может занять несколько минут. Текст и детали будут проверены после генерации.</p>}
      {generationMode==='canvas'&&<p className="muted">Визарно соберёт единую дизайнерскую серию из точных текстов и исходных фотографий. Характеристики не перерисовываются и не искажаются нейросетью.</p>}
      <div hidden={generationMode==='ai'}>
      <label>Фото для слайдов<select value={photo} onChange={e=>{invalidate();setPhoto(e.target.value);setCrop(defaultCrop)}}>
        <option value="auto">По плану каждого слайда</option><option value="0">Исходная обложка</option>
        {version.report.materials?.slides?.map((_,i)=><option value={i+1} key={i}>Слайд {i+2}</option>)}
        {local&&<option value="local">Новое фото товара</option>}
      </select></label>
      <label>Загрузить чистое фото товара<input type="file" accept="image/png,image/jpeg" onChange={upload}/></label>
      <details><summary>Обрезать поля или интерфейс на фото</summary>
        <label><input type="checkbox" checked={manualCrop} onChange={e=>{invalidate();if(e.target.checked&&photo==='auto'){setCrop(steps[0].design.crop||defaultCrop);setPhoto(String(previewPhoto))}setManualCrop(e.target.checked)}}/>Задать область вручную</label>
        <p>{manualCrop?'Выделите товар от одного угла до другого. Эта область попадёт во все слайды серии.':'Рамка показывает область для обложки. Включите ручной выбор, чтобы изменить её.'}</p>
        <CropPreview src={previewSource} area={manualCrop?crop:(photo==='auto'&&steps[0].design.crop)||defaultCrop} onSelect={manualCrop&&!busy?area=>{invalidate();setCrop(area)}:null}/>
        {manualCrop&&<div className="crop-fields">{Object.entries({x:'Слева',y:'Сверху',w:'Ширина',h:'Высота'}).map(([key,label])=><label key={key}>{label}, %<input type="number" min={key==='w'||key==='h'?1:0} max={100} value={crop[key]} onChange={e=>{invalidate();setCrop({...crop,[key]:Number(e.target.value)})}}/></label>)}</div>}
      </details>
      </div>
      {steps.map((s,i)=><details key={i}><summary>{s.name}</summary>
        {photo==='auto'&&version.report.materials?.slides?.length>0&&<label>Фото для слайда {i+1}<select value={s.design.photo} onChange={e=>editDesign(i,{photo:Number(e.target.value),crop:defaultCrop})}>
          <option value={0}>Исходная обложка</option>{version.report.materials.slides.map((item,j)=><option key={j} value={j+1}>Фото {j+2}{item.caption?' · '+item.caption:''}</option>)}
        </select></label>}
        <label>Текст слайда {i+1}<textarea disabled={generationMode==='ai'} rows={5} maxLength={1500} value={(s.copy||[]).join('\n')} onChange={e=>edit(i,{copy:e.target.value.split('\n')})}/></label>
        <label>Композиция слайда {i+1}<select disabled={generationMode==='ai'} value={s.design.layout} onChange={e=>editDesign(i,{layout:e.target.value})}><option value="hero">Крупное фото</option><option value="split">Фото и характеристики рядом</option><option value="focus">Акцент на пояснении</option></select></label>
        <label>Палитра слайда {i+1}<select disabled={generationMode==='ai'} value={s.design.theme} onChange={e=>editDesign(i,{theme:e.target.value})}>{Object.entries(themeNames).map(([key,name])=><option value={key} key={key}>{name}</option>)}</select></label>
        <p className="muted">{s.composition}</p>{s.design.crop&&<p className="muted">Область фото по плану ИИ: слева {s.design.crop.x}%, сверху {s.design.crop.y}%, ширина {s.design.crop.w}%, высота {s.design.crop.h}%.</p>}
      </details>)}
      {generationMode==='ai'&&<p className="muted">В режиме OpenAI тексты и задачи заблокированы: сервер заново строит их из сохранённой анкеты, подтверждённых характеристик и действующей методики психолога.</p>}
      <label>Дополнительная визуальная проверка с ИИ<select value={provider} onChange={e=>{setAIMethod(null);setProvider(e.target.value)}}>
        {!provider&&<option value="">Не настроена — это не мешает сохранению</option>}{providers.map(p=><option key={p.id} value={p.id} disabled={!p.enabled}>{p.label}</option>)}
      </select></label>
      <p><strong>Обязательная проверка выполняется автоматически и бесплатно</strong> — прямо на сервере Визарно, без Groq, OpenAI или другого внешнего API. При желании выбранная нейросеть отдельно оценит композицию и применение методики психолога.</p>
      {generationBlocked&&<p className="blocked-action" role="status"><strong>Генерация пока недоступна.</strong> {generationBlocked}</p>}
      <button type="button" disabled={busy||Boolean(generationBlocked)||(generationMode==='ai'&&!openAIReady)} title={generationBlocked||'Создать и проверить серию'} onClick={generate}>{busy?(generationMode==='ai'?'Нейросеть создаёт серию…':'Обрабатываем…'):(generationMode==='ai'&&pages.length>0&&pages.length<steps.length?'Продолжить со слайда '+(pages.length+1):'Создать карточки')}</button>
    </fieldset>
    <div className="editor-result">
    {pages.length===0&&<figure><figcaption>Исходное фото обложки</figcaption><img src={previewSource} alt="Фото перед генерацией"/></figure>}
    {error&&<p role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
    {method&&!method.approved&&<MethodReport report={method}/>} {aiMethod&&<MethodReport report={aiMethod}/>} 
    {pages.length>0&&<><div className="generated-grid">{pages.map((p,i)=><figure key={i}><img src={p.url} alt={(method?.approved?'Готовый слайд ':'Черновик слайда ')+(i+1)}/><figcaption>{method?.approved?<a href={p.url} download={`vizarno-${plan.cardId}-${i+1}.png`}>Скачать слайд {i+1} · PNG</a>:'Черновик · '+(i+1)}</figcaption></figure>)}</div>
      {method?.approved&&<div className="actions"><button disabled={busy} onClick={save}>Сохранить серию в кабинете</button>{provider&&<button type="button" disabled={busy} onClick={reviewVisual}>Проверить визуал с ИИ</button>}</div>}</>}
    {saved&&<p><a className="next-step" href={'#account/series/'+saved.id}>Открыть сохранённую серию в кабинете →</a></p>}
    </div></div>
  </section>
}
