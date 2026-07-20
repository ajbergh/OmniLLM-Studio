import type { VideoTimelineDocument } from '../../../types/video';
export type PatchOperation={path:(string|number)[];before:unknown;after:unknown};
export interface TimelinePatch{forward:PatchOperation[];inverse:PatchOperation[];bytes:number}
const clone=<T>(value:T):T=>structuredClone(value);
function equal(a:unknown,b:unknown):boolean{return JSON.stringify(a)===JSON.stringify(b)}
function diff(before:unknown,after:unknown,path:(string|number)[],ops:PatchOperation[]):void{
  if(equal(before,after))return;
  if(Array.isArray(before)&&Array.isArray(after)){
    if(before.length!==after.length){ops.push({path,before:clone(before),after:clone(after)});return}
    for(let i=0;i<before.length;i+=1)diff(before[i],after[i],[...path,i],ops);return;
  }
  if(before&&after&&typeof before==='object'&&typeof after==='object'){
    const keys=new Set([...Object.keys(before as object),...Object.keys(after as object)]);
    for(const key of keys)diff((before as Record<string,unknown>)[key],(after as Record<string,unknown>)[key],[...path,key],ops);return;
  }
  ops.push({path,before:clone(before),after:clone(after)});
}
function apply(document:VideoTimelineDocument,ops:PatchOperation[],direction:'before'|'after'):VideoTimelineDocument{
  const next=clone(document) as unknown as Record<string,unknown>;
  for(const op of ops){let cursor:unknown=next;for(let i=0;i<op.path.length-1;i+=1)cursor=(cursor as Record<string|number,unknown>)[op.path[i]];const key=op.path[op.path.length-1];if(key===undefined)return clone((direction==='after'?op.after:op.before) as VideoTimelineDocument);const value=clone(direction==='after'?op.after:op.before);if(value===undefined){if(Array.isArray(cursor))cursor.splice(key as number,1);else delete (cursor as Record<string|number,unknown>)[key]}else{(cursor as Record<string|number,unknown>)[key]=value}}
  return next as unknown as VideoTimelineDocument;
}
export function createTimelinePatch(before:VideoTimelineDocument,after:VideoTimelineDocument):TimelinePatch{const forward:PatchOperation[]=[];diff(before,after,[],forward);const inverse=forward.map((op)=>({path:op.path,before:op.after,after:op.before}));return{forward,inverse,bytes:new TextEncoder().encode(JSON.stringify(forward)).byteLength}}
export const applyTimelinePatch=(document:VideoTimelineDocument,patch:TimelinePatch)=>apply(document,patch.forward,'after');
export const revertTimelinePatch=(document:VideoTimelineDocument,patch:TimelinePatch)=>apply(document,patch.forward,'before');
export class PatchHistory{undo:TimelinePatch[]=[];redo:TimelinePatch[]=[];constructor(public budgetBytes=32*1024*1024){}record(patch:TimelinePatch){if(patch.forward.length===0)return;this.undo.push(patch);this.redo=[];this.compact()}popUndo(){const patch=this.undo.pop();if(patch)this.redo.push(patch);return patch}popRedo(){const patch=this.redo.pop();if(patch)this.undo.push(patch);return patch}reset(){this.undo=[];this.redo=[]}private compact(){let total=this.undo.reduce((sum,item)=>sum+item.bytes,0);while(this.undo.length>1&&total>this.budgetBytes){total-=this.undo.shift()!.bytes}}}
