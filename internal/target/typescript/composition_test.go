package typescript

import (
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestComposedUploadRequestTypesAndRuntime(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
 "openapi":"3.1.0","info":{"title":"Composed upload","version":"1"},
 "paths":{"/uploads":{"post":{"operationId":"upload","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/UploadRequest"}}}},"responses":{"204":{"description":"OK"}}}}},
 "components":{"schemas":{"UploadRequest":{
 "type":"object","properties":{"filename":{"type":"string"},"purpose":{"type":"string","enum":["CUSTOM_INPUT","INQUIRY_ATTACHMENT"]},"customInputID":{"type":"string"}},
 "required":["filename","purpose"],
 "oneOf":[{"properties":{"purpose":{"const":"CUSTOM_INPUT"}},"required":["customInputID"]},{"properties":{"purpose":{"const":"INQUIRY_ATTACHMENT"}}}]
 }}}
 }`))
	if err != nil {
		t.Fatal(err)
	}
	probe := `import { createClient } from "./index.js";
declare const api: ReturnType<typeof createClient>;
type Input = Parameters<typeof api.$operations.upload>[0];
type Body = Input["body"];
const custom: Body = { filename: "a", purpose: "CUSTOM_INPUT", customInputID: "id" };
const attachment: Body = { filename: "a", purpose: "INQUIRY_ATTACHMENT" };
const attachmentWithID: Body = { ...attachment, customInputID: "id" };
// @ts-expect-error common filename is required
const noFilename: Body = { purpose: "CUSTOM_INPUT", customInputID: "id" };
// @ts-expect-error common purpose is required
const noPurpose: Body = { filename: "a", customInputID: "id" };
// @ts-expect-error custom input requires customInputID
const noID: Body = { filename: "a", purpose: "CUSTOM_INPUT" };
// @ts-expect-error required customInputID excludes undefined
const undefinedID: Body = { filename: "a", purpose: "CUSTOM_INPUT", customInputID: undefined };
// @ts-expect-error parent string constraint still applies
const wrongID: Body = { filename: "a", purpose: "CUSTOM_INPUT", customInputID: 1 };
declare const body: Body;
if (body.purpose === "CUSTOM_INPUT") {
 const id: string = body.customInputID;
 void id;
}
void [custom, attachment, attachmentWithID, noFilename, noPurpose, noID, undefinedID, wrongID];
`
	output := compileTypeScriptArtifactsWithProbe(t, document, "composition.probe.ts", probe)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
let calls = 0;
const api = createClient({ baseURL: "https://api.example.test", fetch: async () => { calls++; return new Response(null, {status: 204}); } });
for (const body of [
 {filename:"a", purpose:"CUSTOM_INPUT", customInputID:"id"},
 {filename:"a", purpose:"INQUIRY_ATTACHMENT"},
 {filename:"a", purpose:"INQUIRY_ATTACHMENT", customInputID:"id"}
]) await api.$operations.upload({body});
if (calls !== 3) throw new Error("valid requests did not reach fetch");
for (const body of [
 {purpose:"CUSTOM_INPUT",customInputID:"id"},
 {filename:"a",customInputID:"id"},
 {filename:"a",purpose:"CUSTOM_INPUT"},
 {filename:"a",purpose:"CUSTOM_INPUT",customInputID:undefined},
 {filename:"a",purpose:"CUSTOM_INPUT",customInputID:1}
]) {
 let rejected = false;
 try { await api.$operations.upload({body}); } catch { rejected = true; }
 if (!rejected || calls !== 3) throw new Error("invalid request accepted");
}
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute composed request: %v\n%s", err, result)
	}
}
