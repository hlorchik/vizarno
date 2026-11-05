export const themes = {
  light:{bg:'#eaf4ff',ink:'#102942',accent:'#ff6a13',panel:'#b9dcff'},
  dark:{bg:'#17282e',ink:'#ffffff',accent:'#a2e3cf',panel:'#243b43'},
  warm:{bg:'#f6f0e6',ink:'#302b23',accent:'#87522f',panel:'#ffffff'},
  orange:{bg:'#fff6ed',ink:'#32271d',accent:'#d56816'},yellow:{bg:'#fffbe5',ink:'#302b16',accent:'#e6b720'},
  red:{bg:'#fff3f1',ink:'#392320',accent:'#bf3333'},blue:{bg:'#edf5ff',ink:'#142c43',accent:'#246caf'},
  pink:{bg:'#fff3f8',ink:'#38232e',accent:'#b94f86'},purple:{bg:'#f5f0fb',ink:'#32233d',accent:'#75569a'},
  green:{bg:'#eff8ee',ink:'#1b362a',accent:'#39824c'},black:{bg:'#171819',ink:'#ffffff',accent:'#d9c492'},
};
export const themeNames={light:'Светлая',dark:'Тёмная',warm:'Тёплая',orange:'Оранжевый акцент',yellow:'Жёлтый акцент',red:'Красный акцент',blue:'Синий акцент',pink:'Розовый акцент',purple:'Фиолетовый акцент',green:'Зелёный акцент',black:'Чёрный фон'};

export async function loadPhoto(url) {
  const res=await fetch(url,{signal:AbortSignal.timeout(15000)});
  if(!res.ok)throw new Error('Не удалось загрузить фото. Обновите страницу и проверьте вход в аккаунт.');
  const blob=await res.blob();
  const image=await createImageBitmap(blob);
  if(image.width*image.height>16000000){image.close();throw new Error('Фото должно быть не больше 16 мегапикселей.')}
  return image;
}

function linesFor(ctx,text,width) {
  const result=[];
  for(const paragraph of text.split('\n')){
    let line='';
    for(const word of paragraph.split(/\s+/).filter(Boolean)){
      if(ctx.measureText((line?line+' ':'')+word).width<=width){line+=(line?' ':'')+word;continue}
      if(line){result.push(line);line=''}
      for(const char of word){
        if(line && ctx.measureText(line+char).width>width){result.push(line);line=''}
        line+=char;
      }
    }
    result.push(line);
  }
  return result;
}
function textBox(ctx,text,x,y,w,h,maxSize,minSize,weight,color,spacing=1.12) {
  if(!text)return;
  let lines,size;
  for(size=maxSize;size>=minSize;size-=2){
    ctx.font=`${weight} ${size}px Arial, sans-serif`;
    lines=linesFor(ctx,text,w);
    if(lines.length*size*spacing<=h)break;
  }
  if(size<minSize)throw new Error('Текст не помещается на слайде. Сократите его в редакторе.');
  ctx.fillStyle=color;ctx.textBaseline='top';
  lines.forEach((line,i)=>ctx.fillText(line,x,y+i*size*spacing));
}
function panel(ctx,x,y,w,h,color,r=28){ctx.fillStyle=color;ctx.beginPath();ctx.roundRect(x,y,w,h,r);ctx.fill()}
function imageArea(img,crop){
  const sx=img.width*crop.x/100,sy=img.height*crop.y/100;
  const sw=img.width*crop.w/100,sh=img.height*crop.h/100;
  if(!Object.values(crop).every(Number.isFinite)||crop.x<0||crop.y<0||sw<=0||sh<=0||crop.x+crop.w>100||crop.y+crop.h>100)throw new Error('Область фото выходит за границы изображения.');
  return {sx,sy,sw,sh};
}
function product(ctx,img,crop,x,y,w,h,pad=18){
  const {sx,sy,sw,sh}=imageArea(img,crop),scale=Math.min((w-pad*2)/sw,(h-pad*2)/sh),dw=sw*scale,dh=sh*scale;
  ctx.save();ctx.shadowColor='rgba(25,55,85,.22)';ctx.shadowBlur=34;ctx.shadowOffsetY=22;
  ctx.globalCompositeOperation='multiply';ctx.drawImage(img,sx,sy,sw,sh,x+(w-dw)/2,y+(h-dh)/2,dw,dh);ctx.restore();
}
function softPanel(ctx,x,y,w,h,alpha=.76){ctx.save();ctx.fillStyle=`rgba(255,255,255,${alpha})`;ctx.strokeStyle='rgba(255,255,255,.85)';ctx.lineWidth=2;ctx.beginPath();ctx.roundRect(x,y,w,h,34);ctx.fill();ctx.stroke();ctx.restore()}
function backdrop(ctx,t,index){
  const gradient=ctx.createLinearGradient(0,0,1200,1600);gradient.addColorStop(0,t.bg);gradient.addColorStop(.58,t.bg);gradient.addColorStop(1,t.panel||'#fff');ctx.fillStyle=gradient;ctx.fillRect(0,0,1200,1600);
  ctx.save();ctx.globalAlpha=.18;ctx.fillStyle=t.accent;ctx.beginPath();ctx.moveTo(790,-80);ctx.quadraticCurveTo(1140,270,930,650);ctx.lineTo(1200,500);ctx.lineTo(1200,-80);ctx.closePath();ctx.fill();
  ctx.globalAlpha=.12;ctx.beginPath();ctx.arc(index%2?30:1180,1450,420,0,Math.PI*2);ctx.fill();ctx.restore();
}
function brandName(profile){
  const model=String(profile?.model?.value||'');
  const brands=['HUAWEI','HONOR','APPLE','ASUS','ACER','LENOVO','MSI','DELL','HP','XIAOMI','SAMSUNG','TECNO','GIGABYTE'];
  return brands.find(brand=>new RegExp(`(^|[^A-ZА-Я])${brand}([^A-ZА-Я]|$)`,'i').test(model))||'ВИЗАРНО';
}
function brandBar(ctx,brand,t){
  ctx.save();ctx.fillStyle=t.accent;ctx.beginPath();ctx.arc(82,82,25,0,Math.PI*2);ctx.fill();ctx.fillStyle=t.ink;ctx.font='700 34px Arial, sans-serif';ctx.textBaseline='middle';ctx.fillText(brand,124,82);ctx.restore();
}
function underline(ctx,x,y,w,t){ctx.save();ctx.strokeStyle=t.accent;ctx.lineWidth=10;ctx.lineCap='round';ctx.beginPath();ctx.moveTo(x,y);ctx.quadraticCurveTo(x+w*.48,y-11,x+w,y+1);ctx.stroke();ctx.restore()}
function footer(ctx,index,total,t){ctx.fillStyle=t.ink;ctx.globalAlpha=.55;ctx.font='600 22px Arial, sans-serif';ctx.fillText(`${String(index+1).padStart(2,'0')} / ${String(total).padStart(2,'0')}`,64,1540);ctx.globalAlpha=1}
function cta(ctx,text,t){
  ctx.font='700 54px Arial, sans-serif';const w=Math.min(680,ctx.measureText(text).width+126);panel(ctx,64,1390,w,104,t.accent,52);ctx.fillStyle='#fff';ctx.textBaseline='middle';ctx.fillText(text,104,1442);ctx.lineWidth=7;ctx.strokeStyle='#fff';ctx.beginPath();ctx.moveTo(64+w-62,1419);ctx.lineTo(64+w-38,1442);ctx.lineTo(64+w-62,1465);ctx.stroke();ctx.textBaseline='top';
}
function icon(ctx,kind,x,y,color){
  ctx.save();ctx.strokeStyle=color;ctx.fillStyle=color;ctx.lineWidth=8;ctx.lineJoin='round';
  if(/процесс|core|cpu/i.test(kind)){ctx.strokeRect(x+12,y+12,58,58);for(let i=0;i<4;i++){ctx.beginPath();ctx.moveTo(x+24+i*15,y);ctx.lineTo(x+24+i*15,y+12);ctx.moveTo(x+24+i*15,y+70);ctx.lineTo(x+24+i*15,y+82);ctx.stroke()}}
  else if(/озу|ram|памят/i.test(kind)){ctx.strokeRect(x+4,y+20,76,45);for(let i=0;i<4;i++)ctx.fillRect(x+15+i*16,y+31,9,18)}
  else if(/ssd/i.test(kind)){ctx.beginPath();ctx.roundRect(x+7,y+9,68,64,10);ctx.stroke();ctx.font='700 20px Arial';ctx.textBaseline='middle';ctx.textAlign='center';ctx.fillText('SSD',x+41,y+42)}
  else {
    ctx.lineWidth=6;ctx.beginPath();ctx.roundRect(x+8,y+10,66,62,13);ctx.stroke();
    ctx.lineWidth=9;ctx.lineCap='round';ctx.lineJoin='round';ctx.beginPath();
    ctx.moveTo(x+24,y+42);ctx.lineTo(x+36,y+54);ctx.lineTo(x+60,y+28);ctx.stroke();
  }ctx.restore();
}
function specCard(ctx,line,x,y,w,t){
  const parts=line.split(/\s*[·—]\s*/,2),label=parts.length>1?parts[0]:'',value=parts.length>1?parts[1]:line;
  softPanel(ctx,x,y,w,148,.82);icon(ctx,label||value,x+28,y+32,t.ink);if(label){ctx.fillStyle=t.ink;ctx.globalAlpha=.58;ctx.font='600 22px Arial';ctx.fillText(label.toUpperCase(),x+132,y+28);ctx.globalAlpha=1;textBox(ctx,value,x+132,y+60,w-158,72,36,24,700,t.ink)}else{textBox(ctx,value,x+132,y+42,w-158,82,34,23,700,t.ink)}
}
function coverLayout(ctx,step,img,crop,t,profile,platform){
  const copy=step.copy||[],title=copy[0]||step.name,hasAction=copy.at(-1)==='Покупай!',action=platform==='wildberries'?'':(hasAction?copy.at(-1):''),model=copy.slice(1,hasAction? -1:undefined).join(' · ');
  textBox(ctx,title,64,155,690,430,92,48,800,t.ink,1.03);underline(ctx,68,590,260,t);if(model)textBox(ctx,model,66,635,540,190,44,27,700,t.ink);
  product(ctx,img,crop,350,385,840,875,10);if(action)cta(ctx,action,t);
}
function benefitLayout(ctx,step,img,crop,t){
  const [title='',...rest]=step.copy||[];textBox(ctx,title,64,150,1072,300,82,42,800,t.ink,1.04);underline(ctx,68,460,240,t);
  product(ctx,img,crop,90,500,1020,650);if(rest.length){softPanel(ctx,64,1160,1072,250,.78);textBox(ctx,rest.join('\n'),100,1200,1000,170,43,25,650,t.ink)}
}
function storyLayout(ctx,step,img,crop,t){
  const [title='',...rest]=step.copy||[];textBox(ctx,title,64,150,820,300,78,40,800,t.ink,1.04);underline(ctx,68,455,250,t);
  ctx.fillStyle='rgba(155,109,68,.20)';ctx.fillRect(0,1160,1200,440);ctx.fillStyle='rgba(98,66,42,.22)';ctx.fillRect(0,1185,1200,36);
  panel(ctx,72,1050,250,72,'rgba(255,255,255,.82)',10);panel(ctx,98,990,260,58,'rgba(255,255,255,.7)',10);ctx.strokeStyle=t.accent;ctx.lineWidth=9;ctx.beginPath();ctx.arc(1030,1080,56,0,Math.PI*2);ctx.stroke();
  product(ctx,img,crop,310,540,820,700,8);if(rest.length)textBox(ctx,rest.join('\n'),64,470,390,230,40,25,600,t.ink);
}
function specsLayout(ctx,step,img,crop,t){
  const [title='',...rest]=step.copy||[];textBox(ctx,title,64,150,1072,270,76,40,800,t.ink,1.03);underline(ctx,68,425,240,t);
  product(ctx,img,crop,25,500,700,850,10);const cards=rest.slice(0,4);cards.forEach((line,i)=>specCard(ctx,line,680,500+i*180,456,t));
  if(rest.length>4)textBox(ctx,rest.slice(4).join(' · '),680,1240,456,170,30,22,600,t.ink);
}
function rationalLayout(ctx,step,img,crop,t){
  const [title='',...rest]=step.copy||[];textBox(ctx,title,64,150,1072,230,72,38,800,t.ink);underline(ctx,68,390,220,t);product(ctx,img,crop,690,400,460,400,8);
  const left=rest.slice(0,Math.ceil(rest.length/2)),right=rest.slice(Math.ceil(rest.length/2));
  softPanel(ctx,64,450,570,920,.82);textBox(ctx,left.join('\n\n'),105,500,490,820,36,22,650,t.ink);
  if(right.length){softPanel(ctx,660,850,476,520,.82);textBox(ctx,right.join('\n\n'),700,900,400,420,34,21,650,t.ink)}
}
export async function renderCard(step,image,crop,design,index,total,profile,platform='universal'){
  const canvas=document.createElement('canvas');canvas.width=1200;canvas.height=1600;
  const ctx=canvas.getContext('2d');
  const t=themes[design.theme]||themes.light;
  backdrop(ctx,t,index);brandBar(ctx,brandName(profile),t);
  const name=(step.name||'').toLowerCase();
  if(index===0)coverLayout(ctx,step,image,crop,t,profile,platform);
  else if(/товар в ситуации|ситуац/.test(name))storyLayout(ctx,step,image,crop,t);
  else if(/производительность|экран|детали|мобильность/.test(name))specsLayout(ctx,step,image,crop,t);
  else if(/рациональ|конфигурац/.test(name))rationalLayout(ctx,step,image,crop,t);
  else benefitLayout(ctx,step,image,crop,t);
  footer(ctx,index,total,t);
  const blob=await new Promise(resolve=>canvas.toBlob(resolve,'image/png'));
  if(!blob)throw new Error('Браузер не смог создать PNG. Попробуйте снова.');
  return blob;
}
