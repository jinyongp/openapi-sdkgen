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
      ko: "기본 operation, parameter, JSON body, typed response를 확인합니다.",
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
      ko: "OpenAPI 3.1의 union, nullable value, const, reusable schema를 확인합니다.",
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
    title: { en: "Typed SSE stream", ko: "Typed SSE stream" },
    description: {
      en: "Generate an OperationStream from itemSchema over standard SSE framing.",
      ko: "표준 SSE framing 위의 itemSchema에서 OperationStream을 생성합니다.",
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
