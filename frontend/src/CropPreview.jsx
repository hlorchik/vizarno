import React,{useRef} from 'react';

export default function CropPreview({src,area,onSelect}) {
  const start=useRef(null);
  function point(event) {
    const box=event.currentTarget.getBoundingClientRect();
    const percent=(value,size)=>Math.max(0,Math.min(100,Math.round(value*100/size)));
    return {x:percent(event.clientX-box.left,box.width),y:percent(event.clientY-box.top,box.height)};
  }
  function move(event) {
    if(!start.current || !onSelect)return;
    const end=point(event);
    const w=Math.abs(start.current.x-end.x), h=Math.abs(start.current.y-end.y);
    if(w && h)onSelect({x:Math.min(start.current.x,end.x),y:Math.min(start.current.y,end.y),w,h});
  }
  return <div className={'crop-preview'+(onSelect?' editable':'')}
    onPointerDown={e=>{if(!onSelect||e.button!==0)return;start.current=point(e);e.currentTarget.setPointerCapture(e.pointerId)}}
    onPointerMove={move} onPointerUp={e=>{move(e);start.current=null}} onPointerCancel={()=>{start.current=null}}>
    <img src={src} draggable={false} alt="Исходное фото с рамкой выбранной области"/>
    <span aria-hidden="true" style={{left:area.x+'%',top:area.y+'%',width:area.w+'%',height:area.h+'%'}}/>
  </div>;
}
