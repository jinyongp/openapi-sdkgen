import {expect,it} from "vitest";
import {decodeJSONStreamItems} from "../../../internal/target/typescript/runtime/stream/stream-json.js";
async function read(text: string,size: number=1):Promise<unknown[]> {
 const bytes: Uint8Array = new TextEncoder().encode(text);
 const body: ReadableStream<Uint8Array> = new ReadableStream({start(controller:ReadableStreamDefaultController<Uint8Array>):void {for(let index:number=0;index<bytes.length;index+=size)controller.enqueue(bytes.slice(index,index+size));controller.close();}});
 const result: unknown[]=[];
 for await(const item of decodeJSONStreamItems(body,{contentType:"application/json-seq",streamFraming:"json-sequence",maxFrameBytes:1024,streamCodec:undefined,signal:undefined}))result.push(item);
 expect(body.locked).toBe(false);
 return result;
}
it.each([1,64])("applies RFC 7464 scalar termination with %s-byte chunks",async(size:number):Promise<void>=>{
 for(const text of ["\x1e123","\x1etrue","\x1efalse","\x1enull"])expect(await read(text,size)).toEqual([]);
 expect(await read("\x1e123\x1e{}",size)).toEqual([{}]);
 expect(await read("\x1e123\n\x1etrue\t\x1enull \x1e{}\x1e[]\x1e\"text\"",size)).toEqual([123,true,null,{},[],"text"]);
 await expect(read("123\n",size)).rejects.toThrow("RS");
 await expect(read("\x1e123\u00a0",size)).rejects.toThrow("JSON");
});
