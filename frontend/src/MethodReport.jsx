import React from 'react';

export default function MethodReport({report}){
  const local=report.kind==='local';
  const labels=local?{pass:'Пройдено',skip:'Не требуется',missing:'Нужны данные',fix:'Нужна правка'}:{pass:'Применено',skip:'Неприменимо',missing:'Нужны данные',fix:'Нужна правка'};
  return <section aria-label="Проверка по методике">
    <strong>{report.approved?(local?'Серия прошла обязательную локальную проверку':'ИИ не нашёл нарушений методики'):(local?'Локальная проверка не пройдена':'ИИ рекомендует внести правки')}</strong>
    <p className="muted">{report.provider}</p>
    {!report.approved&&<ul>{report.checks.filter(c=>c.status==='missing'||c.status==='fix').map(c=><li key={c.rule}>{c.name}: {c.note}</li>)}</ul>}
    <details><summary>{local?'Что проверено':'Основания по всем правилам'}</summary><ol>{report.checks.map(c=><li key={c.rule}><b>{c.name}</b> — {labels[c.status]}{!local&&c.slide>0?' · слайд '+c.slide:''}. {c.note}</li>)}</ol></details>
  </section>
}
