# OpenAPI x-* 확장

표준 OpenAPI만으로 SDK를 생성할 수 있습니다. 이 페이지의 `x-*` 필드는 일반
OpenAPI 계약 위에 openapi-sdkgen의 선택적인 편의 기능을 추가합니다.

Schema extension은 필수 사용자 정의 JSON Schema vocabulary를 처리합니다.
해당 기능은 [사용자 정의 JSON Schema vocabulary](../guide/schema-vocabularies.md)에서
설명합니다.

openapi-sdkgen은 지원하는 `x-*` 선언을 코드를 쓰기 전에 검증합니다. 잘못된
선언은 diagnostic과 함께 생성을 중단합니다.

## 표준 OpenAPI 동작

- `api.$routes["METHOD /path"]`는 HTTP method/path를 기준으로 API를 호출합니다.
- query, header, cookie, path 매개변수는 OpenAPI에 선언된 이름을 그대로
  사용합니다.
- `required`, `minimum`, `pattern`, `enum` 같은 스키마 제약은 생성된 코드의
  요청·응답 검사에 반영됩니다.
- openapi-sdkgen이 알지 못하는 `x-*` 필드는 메타데이터에 보존되고 SDK 동작은
  그대로 유지됩니다.

필터는 query 매개변수로, `If-Match`와 `Idempotency-Key`는 header 매개변수로
선언하세요. 이 페이지의 지원 `x-*` 필드가 SDK 확장 동작을 정의합니다.

## `x-envelope`

`x-envelope`를 선언하면 일반 호출은 성공 응답의 `data` 속성을 반환합니다.

```yaml
x-envelope: data
```

이 확장을 사용하려면 본문이 있는 모든 성공 JSON 응답이 `data` 속성을 가진
object여야 합니다. `.raw()`는 나머지 메타데이터를 포함한 전체 응답 본문을
반환합니다.

`x-envelope`의 기본 동작은 전체 응답 반환입니다.

## `x-pagination`

`x-pagination`은 페이지 순회를 돕는 `.paginate()` 메서드를 추가합니다. 일반
operation 호출도 함께 생성됩니다.

### 기본 형식

값은 `cursor`, `offset`, `both` 중 하나입니다.

```yaml
x-pagination: cursor
```

각 방식에 필요한 query 매개변수는 다음과 같습니다.

| 방식 | 필요한 query 매개변수 |
| --- | --- |
| `cursor` | 문자열 `cursor`, 양의 정수 `limit` |
| `offset` | 0 이상의 정수 `offset`, 양의 정수 `limit` |
| `both` | `cursor`, `offset`, `limit` |

성공 JSON 응답은 다음 구조 중 하나를 사용해야 합니다.

| 응답 구조 | 항목 | 페이지 정보 |
| --- | --- | --- |
| 루트 목록 | `/items` | `/pagination/*` |
| 중첩 목록 | `/data/items` | `/data/pagination/*` |
| `data` 배열 | `/data` | `/meta/pagination/*` |

cursor 방식의 `nextCursor`는 문자열 또는 `null`이어야 합니다. offset 방식은
`offset`, `limit`, `total`을 사용할 수 있으며 스키마에도 각 값의 범위를
선언해야 합니다.

### 매개변수와 응답 경로 지정

다른 이름이나 응답 구조를 사용한다면 query 매개변수 이름과 응답 본문의 JSON
Pointer를 연결합니다.

```yaml
x-pagination:
  mode: both
  request:
    cursor: cursorToken
    offset: pageOffset
    limit: pageSize
  response:
    items: /payload/todos
    nextCursor: /payload/page/next
    offset: /payload/page/offset
    limit: /payload/page/limit
    total: /payload/page/total
```

`mode`와 `items`는 필수입니다. cursor 방식은 요청의 `cursor`와 응답의
`nextCursor`가 필요합니다. offset 방식은 요청의 `offset`과 `limit`이
필요합니다.

`.paginate()`는 다음 cursor가 없거나 같은 값이 반복될 때, 또는 offset
페이지가 비었거나 마지막 항목에 도달했을 때 순회를 끝냅니다.

## `x-sort`

정렬에 사용하는 query 매개변수에 선언합니다.

```yaml
- name: sort
  in: query
  schema:
    type: array
    items:
      type: string
      enum: [title:asc, title:desc, createdAt:asc, createdAt:desc]
  x-sort:
    format: field-direction
```

스키마는 `field:asc` 또는 `field:desc` 형식의 고유한 문자열 enum 배열이어야
합니다. 생성된 클라이언트에서는 다음과 같은 값으로 전달할 수 있습니다.

```ts
{ field: "createdAt", direction: "desc" }
```

`x-sort`는 client operation의 정렬 query parameter에 사용합니다.

## `x-sdk-visibility`

클라이언트에서 API를 노출할 방법을 지정합니다.

```yaml
x-sdk-visibility: internal
```

- `internal`: `$routes`와 `$operations`에는 유지하고 resource method에서는 숨깁니다.
- `hidden`: API와 관련 client method를 generated output에서 제거합니다.

`x-sdk-visibility` 기본값은 일반 공개 API입니다.

## `x-error-category`

오류 응답의 `error` object에 `code`가 있고 `category`가 없을 때 정적인 오류
범주를 추가합니다.

```yaml
x-error-category: validation
```

스키마에 이미 필수 `category`가 선언되어 있다면 그 값이 우선합니다. 서로 다른
값을 중복으로 선언하면 오류가 발생합니다.

생성 오류와 CI 사용법은 [SDK 생성](../guide/generate.md)을 참고하세요.
