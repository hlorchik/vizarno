import {test,expect} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import {readFile,writeFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';

test('профиль сохраняется повторно, серия PNG доступна только владельцу',async({page,browser})=>{
 test.setTimeout(100000);
 const base=process.env.VIZARNO_TEST_URL||'http://127.0.0.1:5188';
 const secret=randomUUID();const stranger=await browser.newContext();
 const errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(m.type()==='error')errors.push(m.text())});
 for(const request of [page.request,stranger.request])expect((await request.post(base+'/api/register',{data:{email:randomUUID()+'@example.test',password:secret}})).ok()).toBeTruthy();
 try{
  const res=await page.request.post(base+'/api/products',{data:{title:'Тест генерации',profile:{}}});const {id}=await res.json();
  await page.goto(base);await page.getByRole('button',{name:'Тест генерации',exact:true}).click();
  const profile=page.locator('.profile');
  for(const [name,value] of [['Модель','HUAWEI MateBook D 16'],['Артикул производителя','53013YDL'],['Процессор','Intel Core i5-13420H'],['Оперативная память','16 ГБ'],['SSD','512 ГБ'],['Операционная система','без ОС'],['Экран','16 дюймов WUXGA']]){
   const field=profile.getByRole('group',{name,exact:true});
   await field.getByLabel('Значение',{exact:true}).fill(value);await field.getByLabel('Источник',{exact:true}).fill('Карточка производителя');
  }
  const save=profile.getByRole('button',{name:'Сохранить профиль',exact:true});
  await save.click();await expect(profile.getByRole('status')).toHaveText('Профиль сохранён.');await expect(save).toBeEnabled();
  await profile.getByRole('group',{name:'SSD',exact:true}).getByLabel('Значение',{exact:true}).fill('1 ТБ');
  await save.click();await expect(profile.getByRole('status')).toHaveText('Профиль сохранён.');
  expect((await (await page.request.get(base+'/api/cards')).json()).find(c=>c.id===id).profile.ssd.value).toBe('1 ТБ');
  await expect(page.locator('.profile')).toHaveCount(1);await expect(page.locator('input[name=cover]')).toHaveCount(1);
  expect(errors.filter(e=>/same key|Encountered two|trim is not/.test(e))).toEqual([]);
  await page.getByRole('link',{name:'Материалы',exact:true}).click();
  await page.locator('input[name=cover]').setInputFiles(fileURLToPath(new URL('../../Исследование/01-онлайнтрейд.png',import.meta.url)));
  await page.getByRole('button',{name:'Сохранить новую версию',exact:true}).click();
  const generate=page.getByRole('button',{name:'Создать карточки',exact:true});
  await expect(page.getByRole('heading',{name:'Результат анализа'})).toBeVisible();
  await page.getByRole('link',{name:'Генерация',exact:true}).click();
  await expect(generate).toBeDisabled();
  await page.getByRole('link',{name:'Материалы',exact:true}).click();
  await page.getByText('Обновить обложку, описание или анкету',{exact:true}).click();
  await page.locator('input[name=cover]').setInputFiles(fileURLToPath(new URL('../../Исследование/01-онлайнтрейд.png',import.meta.url)));
  await page.locator('textarea[name=endUser]').fill('Школьник');
  await page.locator('textarea[name=experience]').fill('Первый личный ноутбук');
  for(const [k,v] of Object.entries({buyer:'Близкий человек',occasion:'Для учёбы',priority:'Состав комплекта',channel:'В мобильной выдаче',visual:'Отдельный предмет',benefitField:'Операционная система'}))await page.locator('select[name='+k+']').selectOption(v);
  for(const [k,v] of Object.entries({task:'Подготовить ноутбук к первому уроку',concern:'Успеем ли подготовить ноутбук до первого занятия?',barriers:'Необходимость самостоятельно установить операционную систему',keyCriteria:'Экран, память, накопитель и готовность к первому занятию',message:'Перед первым занятием установите систему',basis:'Интервью с родителем',motive:'Начать урок вовремя',advantage:'Можно выбрать систему для установки',benefit:'Подготовьте программы к первому уроку',proof:'Описание поставки без ОС'}))await page.locator('textarea[name='+k+']').fill(v);
  await page.getByRole('button',{name:'Сохранить новую версию',exact:true}).click();
  await expect(page.getByRole('heading',{name:'Результат анализа'})).toBeVisible();
  await page.getByRole('link',{name:'Генерация',exact:true}).click();
  if(process.env.VIZARNO_LIVE_GENERATION==='1')await page.getByLabel('Сервис проверки итоговых слайдов').selectOption('groq');
  await expect(profile).toBeHidden();
  await page.getByText('Обрезать поля или интерфейс на фото',{exact:true}).click();
  await page.getByLabel('Задать область вручную').check();
  const editor=page.locator('.crop-preview');await editor.scrollIntoViewIfNeeded();const box=await editor.boundingBox();
  await page.mouse.move(box.x+.20*box.width,box.y+.38*box.height);await page.mouse.down();
  await page.mouse.move(box.x+.48*box.width,box.y+.78*box.height,{steps:8});await page.mouse.up();
  await expect(page.getByLabel('Слева, %',{exact:true})).toHaveValue('20');await expect(page.getByLabel('Ширина, %',{exact:true})).toHaveValue('28');
  if(process.env.VIZARNO_METHOD_FIXTURE==='1'){
    await generate.click();
    await expect(page.getByText('Для выпуска серии нужны правки',{exact:true})).toBeVisible();
    await expect(page.getByRole('button',{name:'Сохранить серию в кабинете'})).toHaveCount(0);
    await expect(page.getByRole('link',{name:'Скачать слайд 1 · PNG'})).toHaveCount(0);
  }
  const [release]=await Promise.all([page.waitForResponse(r=>r.url().endsWith('/render-check'),{timeout:48000}),generate.click()]);
  expect(release.status(),await release.text()).toBe(200);
  if(process.env.VIZARNO_LIVE_GENERATION==='1'){
    await expect(page.getByLabel('Проверка по методике')).toBeVisible({timeout:48000});
    if(await page.getByText('Для выпуска серии нужны правки',{exact:true}).count()){
      await expect(page.getByRole('link',{name:'Скачать слайд 1 · PNG'})).toHaveCount(0);return;
    }
  }
  await expect(page.getByAltText('Готовый слайд 8',{exact:true})).toBeVisible();
  const [download]=await Promise.all([page.waitForEvent('download'),page.getByRole('link',{name:'Скачать слайд 1 · PNG',exact:true}).click()]);
  const png=await readFile(await download.path());expect(png.subarray(1,4).toString()).toBe('PNG');expect(png.readUInt32BE(16)).toBe(1200);expect(png.readUInt32BE(20)).toBe(1600);
  await writeFile(test.info().outputPath('generated-slide.png'),png);
  await page.locator('.generator').screenshot({path:test.info().outputPath('generator.png')});
  const [saved]=await Promise.all([page.waitForResponse(r=>r.url().endsWith('/renders')&&r.request().method()==='POST'),page.getByRole('button',{name:'Сохранить серию в кабинете',exact:true}).click()]);
  expect(saved.status(),await saved.text()).toBe(201);const set=await saved.json();
  const info=await(await page.request.get(base+'/api/account/series/'+set.id)).json();
  expect(info.methodReport.approved).toBe(true);expect(info.methodReport.checks).toHaveLength(38);
  const swapped=new FormData();
  const activeProfile=(await(await page.request.get(base+'/api/cards')).json()).find(c=>c.id===id).profile;
  swapped.set('profile',JSON.stringify(activeProfile));swapped.set('version',String(set.version));
  for(let i=0;i<8;i++)swapped.append('pages',new Blob([png],{type:'image/png'}),'same.png');
  expect((await page.request.post(base+'/api/cards/'+id+'/renders',{multipart:swapped})).status()).toBe(422);
  expect((await stranger.request.get(base+'/api/renders/'+set.id+'/pages/1')).status()).toBe(404);
  expect((await stranger.request.get(base+'/api/cards/'+id+'/renders')).status()).toBe(404);
  const [second]=await Promise.all([page.waitForResponse(r=>r.url().endsWith('/renders')&&r.request().method()==='POST'),page.getByRole('button',{name:'Сохранить серию в кабинете',exact:true}).click()]);
  const newer=await second.json();expect(newer.id).toBeGreaterThan(set.id);
  await page.getByRole('link',{name:'Открыть сохранённую серию в кабинете',exact:false}).click();
  await expect(page).toHaveURL(base+'/#account/series/'+newer.id);
  await page.getByRole('link',{name:'Личный кабинет',exact:true}).click();
  await expect(page.locator('.series-tile')).toHaveCount(2);
  await page.locator('a.series-tile[href="#account/series/'+set.id+'"]').click();
  await page.reload();
  await page.getByRole('button',{name:'Показать слайд 3'}).click();
  await expect(page.getByAltText('Слайд 3 из 4')).toHaveAttribute('src','/api/renders/'+set.id+'/pages/3');
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBeTruthy();
  await page.getByRole('button',{name:'Показать слайд 1'}).click();
  const [file]=await Promise.all([page.waitForEvent('download'),page.getByRole('link',{name:'Скачать слайд 1 · PNG'}).click()]);
  expect((await readFile(await file.path())).equals(png)).toBe(true);
  expect((await (await stranger.request.get(base+'/api/account/series')).json()).items).toEqual([]);
  expect((await stranger.request.get(base+'/api/account/series/'+set.id)).status()).toBe(404);
  await page.setViewportSize({width:1280,height:800});
  await page.getByRole('link',{name:'Открыть товар в мастерской'}).click();
  const restored=await (await page.request.get(base+'/api/renders/'+set.id+'/pages/1')).body();expect(restored.equals(png)).toBeTruthy();
  // An error must unlock the button and preserve entered values.
  await expect(profile).toBeVisible();
  await page.route('**/api/cards/'+id+'/profile',route=>route.fulfill({status:503,contentType:'application/json',body:'{"error":"Тест: база временно недоступна"}'}),{times:1});
  await save.click();await expect(profile.getByRole('status')).toContainText('база временно недоступна');await expect(save).toBeEnabled();
  await page.clock.install();
  let stalled;
  await page.route('**/api/cards/'+id+'/profile',route=>{stalled=route},{times:1});
  await save.click();await expect(profile.getByRole('button',{name:'Сохраняем профиль…',exact:true})).toBeDisabled();
  await page.clock.fastForward(21000);
  await expect(profile.getByRole('status')).toContainText('Сервер не ответил вовремя');await expect(save).toBeEnabled();
  await stalled.abort().catch(()=>{});
  await page.clock.resume();
  await profile.getByRole('group',{name:'SSD',exact:true}).getByLabel('Значение',{exact:true}).fill('2 ТБ');await save.click();
  await expect(profile.getByRole('status')).toHaveText('Профиль сохранён.');
  const old=await(await page.request.get(base+'/api/cards/'+id+'/renders')).json();expect(old.stale).toBe(true);
  const staleData={version:String(set.version),profile:'{}',pages:{name:'test.png',mimeType:'image/png',buffer:png}};
  expect((await page.request.post(base+'/api/cards/'+id+'/renders',{multipart:staleData})).status()).toBe(409);
  expect((await stranger.request.post(base+'/api/cards/'+id+'/renders',{multipart:staleData})).status()).toBe(404);
  const invalid={...staleData,pages:{name:'bad.png',mimeType:'image/png',buffer:Buffer.from('not a png')}};
  expect((await page.request.post(base+'/api/cards/'+id+'/renders',{multipart:invalid})).status()).toBe(400);
 }finally{
  for(const request of [page.request,stranger.request])await request.delete(base+'/api/account',{data:{password:secret}});
  await stranger.close();
 }
});
