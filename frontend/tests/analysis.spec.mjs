import {test,expect} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import {readFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';

test('продавец проверяет основание, выбирает правку и сравнивает версии',async({page,browser})=>{
  test.setTimeout(100000);
  const base=process.env.VIZARNO_TEST_URL || 'http://127.0.0.1:5188';
  const secret=randomUUID();
  const register=async(request)=>expect((await request.post(base+'/api/register',{data:{email:randomUUID()+'@example.test',password:secret}})).ok()).toBeTruthy();
  const stranger=await browser.newContext();
  const cover=fileURLToPath(new URL('../../Исследование/01-онлайнтрейд-превью.png',import.meta.url));
  const gallery=fileURLToPath(new URL('../../Исследование/01-онлайнтрейд-характеристики.png',import.meta.url));
  try{
    await register(page.request);await register(stranger.request);
    const created=await page.request.post(base+'/api/products',{data:{title:'Учебный MateBook',profile:{os:{value:'без ОС',source:'Заголовок предложения 53013YDL'}}}});
    expect(created.status()).toBe(201);
    const {id}=await created.json();
    await page.goto(base);
    await page.getByRole('button',{name:'Учебный MateBook'}).click();
    const upload=async (description,caption)=>{
      await page.getByRole('link',{name:'Материалы',exact:true}).click();
      await page.locator('input[name=cover]').setInputFiles(cover);
      await page.locator('input[name=headline]').fill('Huawei MateBook D 16');
      await page.locator('textarea[name=description]').fill(description);
      await page.locator('input[name=gallery]').setInputFiles(gallery);
      await page.locator('input[name=caption]').fill(caption);
      await page.getByRole('button',{name:'Сохранить и проверить'}).click();
    };
    await upload('Ноутбук для занятий. Без ОС.','Характеристики ноутбука');
    const issue=page.locator('.finding').filter({has:page.locator('summary',{hasText:'Подготовка к первому запуску'})});
    await expect(issue.locator('summary')).toContainText('Нужна правка');
    await issue.locator('summary').click();
    const cb=issue.getByRole('checkbox');
    await expect(cb).toBeChecked();
    await cb.uncheck();await expect(cb).not.toBeChecked();
    const downloads=page.getByRole('button',{name:'Скачать ТЗ (.txt)',exact:true});
    await expect(downloads).toBeVisible();
    await cb.check();await expect(cb).toBeChecked();
    const [download]=await Promise.all([page.waitForEvent('download'),downloads.click()]);
    expect(await readFile(await download.path(),'utf8')).toContain('Выбранные правки');
    const v=(await (await page.request.get(base+'/api/cards/'+id+'/versions')).json())[0];
    const slideURL=base+'/api/covers/'+v.id+'/slides/2';
    expect((await page.request.get(slideURL)).status()).toBe(200);
    expect((await stranger.request.get(slideURL)).status()).toBe(404);
    expect((await stranger.request.get(base+'/api/cards/'+id+'/designer-brief?format=preview')).status()).toBe(404);
    if(process.env.VIZARNO_LIVE_GROQ==='1'){
      await page.getByLabel('Нейросеть для визуального разбора').selectOption('groq');
      const [res]=await Promise.all([page.waitForResponse(r=>r.url().includes('/ai-brief?'),{timeout:50000}),page.getByRole('button',{name:'Дополнить ТЗ визуальным разбором'}).click()]);
      expect(res.status(),await res.text()).toBe(200);
      const result=await res.json();
      expect(result.review.gallery).toHaveLength(1);expect(result.review.insights.length).toBeGreaterThan(0);
      const restored=await (await page.request.get(base+'/api/cards/'+id+'/designer-brief?format=preview')).json();
      expect(restored.saved.review).toEqual(result.review);
    }
    await upload('Без ОС.','Операционная система не установлена. Перед первым занятием её нужно установить.');
    await expect(issue.locator('summary')).toContainText('Слайд 2 · подпись продавца — Проверка пройдена');
    await page.getByText('Что изменилось с выбранной версии',{exact:false}).click();
    await expect(page.getByText('Нужна правка → Проверка пройдена',{exact:false}).first()).toBeVisible();
  }finally{
    for(const api of [page.request,stranger.request])await api.delete(base+'/api/account',{data:{password:secret}});
    await stranger.close();
  }
});

