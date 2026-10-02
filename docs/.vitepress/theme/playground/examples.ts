export interface PlaygroundExample {
  readonly id: string;
  readonly version: string;
  readonly title: Readonly<{ en: string; ko: string }>;
  readonly description: Readonly<{ en: string; ko: string }>;
  readonly document: string;
}

export const playgroundExamples: readonly PlaygroundExample[] = [
  {
    id: "todo-api",
    version: "3.0.4",
    title: { en: "Todo API", ko: "Todo API" },
    description: {
      en: "Basic operations, parameters, JSON bodies, and typed responses.",
      ko: "기본 API 호출, 매개변수, JSON 본문과 응답 타입을 확인합니다.",
    },
    document: `{
  "openapi": "3.0.4",
  "info": { "title": "Todo API", "version": "1.0.0" },
  "paths": {
    "/todos": {
      "get": {
        "operationId": "listTodos",
        "parameters": [
          { "name": "completed", "in": "query", "schema": { "type": "boolean" } }
        ],
        "responses": {
          "200": {
            "description": "Todos",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": { "$ref": "#/components/schemas/Todo" }
                }
              }
            }
          }
        }
      },
      "post": {
        "operationId": "createTodo",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["title"],
                "properties": { "title": { "type": "string" } }
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Created",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Todo" }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Todo": {
        "type": "object",
        "required": ["id", "title", "completed"],
        "properties": {
          "id": { "type": "string" },
          "title": { "type": "string" },
          "completed": { "type": "boolean" }
        }
      }
    }
  }
}`,
  },
  {
    id: "json-schema-31",
    version: "3.1.1",
    title: { en: "JSON Schema 2020-12", ko: "JSON Schema 2020-12" },
    description: {
      en: "OpenAPI 3.1 unions, nullable values, const, and reusable schemas.",
      ko: "OpenAPI 3.1에서 여러 스키마의 조합, 널 값 허용, 고정값과 스키마 재사용을 확인합니다.",
    },
    document: `{
  "openapi": "3.1.1",
  "info": { "title": "Events API", "version": "1.0.0" },
  "paths": {
    "/events/{eventId}": {
      "get": {
        "operationId": "getEvent",
        "parameters": [
          {
            "name": "eventId",
            "in": "path",
            "required": true,
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": {
            "description": "Event",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Event" }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Event": {
        "oneOf": [
          {
            "type": "object",
            "required": ["type", "message"],
            "properties": {
              "type": { "const": "message" },
              "message": { "type": "string" },
              "source": { "type": ["string", "null"] }
            }
          },
          {
            "type": "object",
            "required": ["type", "progress"],
            "properties": {
              "type": { "const": "progress" },
              "progress": { "type": "number", "minimum": 0, "maximum": 1 }
            }
          }
        ]
      }
    }
  }
}`,
  },
  {
    id: "typed-sse",
    version: "3.2.0",
    title: { en: "Typed SSE stream", ko: "타입을 검사하는 SSE 스트림" },
    description: {
      en: "Generate an OperationStream from itemSchema over standard SSE framing.",
      ko: "각 SSE 이벤트의 스키마에 따라 응답 타입을 지정하고 스트림을 생성합니다.",
    },
    document: `{
  "openapi": "3.2.0",
  "info": { "title": "Todo Stream API", "version": "1.0.0" },
  "paths": {
    "/todos/stream": {
      "get": {
        "operationId": "watchTodos",
        "responses": {
          "200": {
            "description": "Todo events",
            "content": {
              "text/event-stream": {
                "itemSchema": {
                  "type": "object",
                  "required": ["data"],
                  "properties": {
                    "data": { "type": "string" },
                    "event": { "type": "string" },
                    "id": { "type": "string" },
                    "retry": { "type": "integer", "minimum": 0 }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`,
  },

];
