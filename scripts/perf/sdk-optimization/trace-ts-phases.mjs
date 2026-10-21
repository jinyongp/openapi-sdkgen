// Intrusive diagnostic using the pinned compiler API, with the same complete
// source/public consumer configuration. This never relaxes strict options.
import fs from "node:fs";
import path from "node:path";
const [compiler,configArg,outputArg]=process.argv.slice(2);
if(!['typescript-5-7','typescript-5-9','typescript-6'].includes(compiler)||!configArg||!outputArg)throw Error('COMPILER CONFIG FRESH_JSONL required');
const ts=(await import(new URL(`../../../test/typescript/node_modules/${compiler}/lib/typescript.js`,import.meta.url))).default;
const config=path.resolve(configArg),output=fs.openSync(path.resolve(outputArg),'wx',0o600),started=performance.now();
const phase=(name,extra={})=>{
  const row={phase:name,elapsedSeconds:(performance.now()-started)/1000,...process.memoryUsage(),...extra};
  fs.writeSync(output,JSON.stringify(row)+'\n');console.log(JSON.stringify(row));
};
const loaded=ts.readConfigFile(config,ts.sys.readFile);
if(loaded.error)throw Error(ts.flattenDiagnosticMessageText(loaded.error.messageText,'\n'));
const parsed=ts.parseJsonConfigFileContent(loaded.config,ts.sys,path.dirname(config));
if(parsed.errors.length)throw Error('configuration diagnostics: '+parsed.errors.length);
if(parsed.options.noCheck||parsed.options.skipLibCheck||!parsed.options.strict)throw Error('strict configuration required');
phase('configuration',{compiler:ts.version,rootFiles:parsed.fileNames.length});
ts.performance.enable();
phase('program-start');
const program=ts.createProgram(parsed.fileNames,parsed.options);
phase('program-created',{sourceFiles:program.getSourceFiles().length});
const syntactic=program.getSyntacticDiagnostics();phase('syntactic',{diagnostics:syntactic.length});
program.getTypeChecker();phase('bound');
const diagnostics=ts.getPreEmitDiagnostics(program);
const measures={};ts.performance.forEachMeasure((name,value)=>{measures[name]=value;});
phase('checked',{diagnostics:diagnostics.length,nodes:program.getNodeCount(),identifiers:program.getIdentifierCount(),symbols:program.getSymbolCount(),types:program.getTypeCount(),instantiations:program.getInstantiationCount(),measures});
for(const diagnostic of diagnostics){
  const position=diagnostic.file&&diagnostic.start!==undefined?diagnostic.file.getLineAndCharacterOfPosition(diagnostic.start):undefined;
  fs.writeSync(output,JSON.stringify({diagnostic:true,code:diagnostic.code,file:diagnostic.file?.fileName,line:position?position.line+1:undefined,character:position?position.character+1:undefined,message:ts.flattenDiagnosticMessageText(diagnostic.messageText,'\n')})+'\n');
}
fs.closeSync(output);
process.exitCode=diagnostics.some(diagnostic=>diagnostic.category===ts.DiagnosticCategory.Error)?1:0;
