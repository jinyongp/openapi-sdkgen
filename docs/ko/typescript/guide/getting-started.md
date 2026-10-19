# 시작하기

<span id="_1-실행-환경과-프로젝트-준비"></span>
<span id="_2-todo-명세-저장"></span>
<span id="_3-sdk-생성"></span>
<span id="_4-선택-모의-응답으로-호출-확인"></span>
<span id="_5-실제-api-연결과-다음-작업"></span>
<span id="homebrew"></span>
<span id="npm"></span>
<span id="실행-파일-다운로드"></span>

OpenAPI 명세에서 SDK를 생성하고 프로젝트에서 불러옵니다.
전체 실행 예제가 필요하면 [Todo 예제 ZIP](/downloads/typescript-todo-app.zip)을 받아
포함된 README를 따라 실행하세요. 로컬 API 서버와 클라이언트가 함께 들어 있습니다.

## CLI 설치 {#install}

macOS / Linux:

```sh
curl -fsSL https://jinyongp.dev/openapi-sdkgen/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://jinyongp.dev/openapi-sdkgen/install.ps1 | iex
```

안내된 설치 경로를 필요에 따라 `PATH`에 추가한 뒤 `openapi-sdkgen --version`으로 확인하세요.

::: details 버전·설치 경로 지정과 다른 설치 방법

기본 경로는 macOS/Linux에서 `~/.local/bin`, Windows에서
`%LOCALAPPDATA%\Programs\openapi-sdkgen\bin`입니다.

버전이나 경로를 지정하려면 스크립트를 파일로 저장해 실행하세요.

```sh
curl -fsSL https://jinyongp.dev/openapi-sdkgen/install.sh -o install.sh
sh install.sh --version X.Y.Z --install-dir /your/bin
```

```powershell
irm https://jinyongp.dev/openapi-sdkgen/install.ps1 -OutFile install.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Version X.Y.Z -InstallDir C:\your\bin
```

`X.Y.Z`는 설치할 릴리스 버전으로 바꾸세요.
업데이트할 때는 설치 스크립트를 새로 받아 실행하세요. 삭제하려면 설치된 실행 파일을 지우세요.

[실행 파일 직접 다운로드](https://github.com/jinyongp/openapi-sdkgen/releases),
`npm install -g openapi-sdkgen`(Node.js 22+), `brew install jinyongp/tap/openapi-sdkgen`도 사용할 수 있습니다.

:::

## SDK 생성 {#generate}

프로젝트의 `openapi.json`을 입력으로 지정합니다.
샘플은 [Todo 명세](https://github.com/jinyongp/openapi-sdkgen/blob/main/examples/typescript-todo-app/openapi.json)를 사용하세요.

```sh
openapi-sdkgen generate \
  --input ./openapi.json \
  --target typescript \
  --output ./src/generated/api \
  --incremental
```

`src/generated/api/`에 생성된 소스를 프로젝트의 빌드 도구로 빌드합니다.
명세가 바뀌면 같은 명령으로 갱신할 수 있습니다.

## 애플리케이션에서 호출 {#call}

Todo 명세를 생성한 뒤 다음처럼 사용합니다. `baseURL`에는 실제 API 서버 주소를 넣으세요.
아래 주소는 내려받은 예제의 로컬 서버 주소입니다.

```ts
import { createClient } from "./generated/api/index.js";

const api = createClient({ baseURL: "http://localhost:18787/v1" });

const todo = await api.todos.create({
  body: { title: "Write documentation" },
});

const todos = await api.todos.list({ query: { completed: false } });
console.log(JSON.stringify(todo));
console.log(JSON.stringify(todos));
```

예제 서버를 처음 실행한 상태라면 첫 호출은 항목 `1`을 만들고, 두 번째 호출은 그 항목을 `items`에 담아 반환합니다. 출력은 다음과 같습니다.

```text
{"id":"1","title":"Write documentation","completed":false}
{"items":[{"id":"1","title":"Write documentation","completed":false}]}
```

`todo`에는 생성된 항목이, `todos.items`에는 조회한 항목들이 반환됩니다.
Vite·Next.js 같은 번들러에서는 `./generated/api`로 가져올 수 있습니다.
위의 `/index.js` 경로는 컴파일한 코드를 Node ESM에서 실행할 때 사용합니다.

[호출과 오류 처리](./client.md), [인증과 전송](./transport.md),
[명세 변경 후 재생성](./generate.md)을 이어서 확인하세요.
