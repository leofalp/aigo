# Guida Completa ai Formati JSON delle API dei Principali Provider LLM

## Indice
1. [OpenAI (GPT-4, GPT-3.5)](#openai-gpt-4-gpt-35)
2. [Anthropic (Claude)](#anthropic-claude)
3. [Google Gemini](#google-gemini)
4. [Mistral AI](#mistral-ai)
5. [Cohere](#cohere)
6. [Tabelle Comparative](#tabelle-comparative)

---

## OpenAI (GPT-4, GPT-3.5)

### Richiesta (Request)

```json
{
  "model": "gpt-4",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello!"
    },
    {
      "role": "assistant",
      "content": "Hi there! How can I help you?"
    },
    {
      "role": "user",
      "content": "What's the weather like?"
    }
  ],
  "max_tokens": 1000,
  "temperature": 0.7,
  "top_p": 1.0,
  "frequency_penalty": 0.0,
  "presence_penalty": 0.0,
  "response_format": { 
    "type": "json_object" 
  },
  "seed": 42,
  "stream": false
}
```

#### Parametri Principali
- `model`: ID del modello (es. "gpt-4", "gpt-3.5-turbo")
- `messages`: Array di messaggi con ruoli (`system`, `user`, `assistant`)
- `response_format`: `{ "type": "json_object" }` per forzare output JSON
- `response_format` con schema: `{ "type": "json_schema", "json_schema": {...} }`

### Risposta (Response)

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1677858242,
  "model": "gpt-4",
  "usage": {
    "prompt_tokens": 13,
    "completion_tokens": 7,
    "total_tokens": 20
  },
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "This is the response text!"
      },
      "logprobs": null,
      "finish_reason": "stop",
      "index": 0
    }
  ],
  "system_fingerprint": "fp_abc123"
}
```

#### Accesso al Contenuto
```javascript
const content = response.choices[0].message.content;
```

#### Valori `finish_reason`
- `stop`: Completamento normale
- `length`: Raggiunto limite token
- `content_filter`: Bloccato da filtri contenuti
- `tool_calls`: Chiamata a funzione
- `function_call`: Chiamata a funzione (deprecato)

---

## Anthropic (Claude)

### Richiesta (Request)

```json
{
  "model": "claude-sonnet-4-5",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "Hello, Claude"
    },
    {
      "role": "assistant",
      "content": "Hello!"
    },
    {
      "role": "user",
      "content": "Can you help me?"
    }
  ],
  "system": "You are a helpful assistant.",
  "temperature": 0.7,
  "top_p": 0.9,
  "top_k": 40,
  "stop_sequences": ["\n\nHuman:"]
}
```

#### Tecnica per JSON Output (Prefilling)
```json
{
  "model": "claude-sonnet-4-5",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "Generate a JSON with name and age."
    },
    {
      "role": "assistant",
      "content": "{"
    }
  ]
}
```

#### Caratteristiche Principali
- `system`: Prompt di sistema separato (non nell'array messages)
- `messages`: Alterna tra `user` e `assistant`
- Per JSON: usa prefilling con `{"role": "assistant", "content": "{"}`
- **Header richiesti**: `x-api-key`, `anthropic-version: 2023-06-01`, `content-type: application/json`

### Risposta (Response)

```json
{
  "id": "msg_01XFDUDYJgAACzvnptvVoYEL",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I help you today?"
    }
  ],
  "model": "claude-sonnet-4-5",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 12,
    "output_tokens": 6
  }
}
```

#### Accesso al Contenuto
```javascript
const content = response.content[0].text;
```

#### Valori `stop_reason`
- `end_turn`: Completamento normale
- `max_tokens`: Raggiunto limite token
- `stop_sequence`: Trovata sequenza di stop
- `tool_use`: Chiamata a tool

---

## Google Gemini

### Richiesta (Request)

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "Explain how AI works"
        }
      ]
    }
  ],
  "generationConfig": {
    "temperature": 0.7,
    "topP": 0.9,
    "topK": 40,
    "maxOutputTokens": 1000,
    "responseMimeType": "application/json",
    "responseSchema": {
      "type": "object",
      "properties": {
        "name": { 
          "type": "string" 
        },
        "age": { 
          "type": "integer" 
        }
      },
      "required": ["name"]
    }
  },
  "systemInstruction": {
    "parts": [
      {
        "text": "You are a helpful assistant."
      }
    ]
  },
  "safetySettings": [
    {
      "category": "HARM_CATEGORY_HARASSMENT",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    }
  ]
}
```

#### Caratteristiche Principali
- `contents`: Array di oggetti con `role` e `parts`
- `responseMimeType`: `"application/json"` per output JSON
- `responseSchema`: JSON Schema per structured output
- `systemInstruction`: Istruzioni di sistema separate

### Risposta (Response)

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "This is the AI's response."
          }
        ],
        "role": "model"
      },
      "finishReason": "STOP",
      "index": 0,
      "safetyRatings": [
        {
          "category": "HARM_CATEGORY_HARASSMENT",
          "probability": "NEGLIGIBLE"
        },
        {
          "category": "HARM_CATEGORY_HATE_SPEECH",
          "probability": "NEGLIGIBLE"
        }
      ]
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 10,
    "candidatesTokenCount": 20,
    "totalTokenCount": 30
  },
  "modelVersion": "gemini-2.5-flash"
}
```

#### Accesso al Contenuto
```javascript
const content = response.candidates[0].content.parts[0].text;
// oppure
const dict = response.to_dict();
const content = dict["candidates"][0]["content"]["parts"][0]["text"];
```

#### Valori `finishReason`
- `STOP`: Completamento normale
- `MAX_TOKENS`: Raggiunto limite token
- `SAFETY`: Bloccato da safety filters
- `RECITATION`: Contenuto ricorrente rilevato
- `OTHER`: Altri motivi

---

## Mistral AI

### Richiesta (Request)

```json
{
  "model": "mistral-large-latest",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello!"
    }
  ],
  "max_tokens": 1000,
  "temperature": 0.7,
  "top_p": 1.0,
  "response_format": {
    "type": "json_object"
  },
  "safe_prompt": false,
  "random_seed": 42
}
```

#### Request con JSON Schema

```json
{
  "model": "ministral-8b-latest",
  "messages": [
    {
      "role": "user",
      "content": "Extract book information from: 'To Kill a Mockingbird by Harper Lee'"
    }
  ],
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "schema": {
        "type": "object",
        "properties": {
          "name": { 
            "type": "string",
            "title": "Name" 
          },
          "authors": { 
            "type": "array",
            "items": { "type": "string" },
            "title": "Authors"
          }
        },
        "required": ["name", "authors"],
        "title": "Book",
        "additionalProperties": false
      },
      "name": "book",
      "strict": true
    }
  },
  "max_tokens": 256,
  "temperature": 0
}
```

#### Caratteristiche Principali
- Formato simile a OpenAI
- `response_format`: `{ "type": "json_object" }` o `{ "type": "json_schema" }`
- `strict: true` garantisce aderenza allo schema

### Risposta (Response)

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1704748168,
  "model": "mistral-large-latest",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "This is the response from Mistral."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 13,
    "completion_tokens": 38,
    "total_tokens": 51
  }
}
```

#### Accesso al Contenuto
```javascript
const content = response.choices[0].message.content;
```

#### Valori `finish_reason`
- `stop`: Completamento normale
- `length`: Raggiunto limite token
- `model_length`: Limite contesto modello
- `tool_calls`: Chiamata a tool

---

## Cohere

### Richiesta (Request)

```json
{
  "model": "command-r-plus",
  "message": "Hello!",
  "chat_history": [
    {
      "role": "USER",
      "message": "Previous message"
    },
    {
      "role": "CHATBOT",
      "message": "Previous response"
    }
  ],
  "preamble": "You are a helpful assistant.",
  "temperature": 0.7,
  "max_tokens": 1000,
  "k": 0,
  "p": 0.75,
  "frequency_penalty": 0.0,
  "presence_penalty": 0.0,
  "response_format": {
    "type": "json_object",
    "json_schema": {
      "schema": {
        "type": "object",
        "properties": {
          "title": { "type": "string" },
          "author": { "type": "string" }
        },
        "required": ["title"]
      }
    }
  },
  "seed": 42,
  "stream": false
}
```

#### Caratteristiche Principali
- `message`: Messaggio corrente (non array)
- `chat_history`: Cronologia della conversazione
- `preamble`: Prompt di sistema
- Ruoli: `USER` e `CHATBOT` (maiuscolo)
- `response_format`: Supporta `json_object` e `json_schema`

### Risposta (Response)

```json
{
  "response_id": "d479aeb3-a87d-41ef-b13f-7a712094106e",
  "text": "This is Cohere's response to your message.",
  "generation_id": "6e0f59a0-4469-412e-9279-d552f2b51db4",
  "chat_history": [
    {
      "role": "USER",
      "message": "Hello"
    },
    {
      "role": "CHATBOT",
      "message": "Hi there!"
    },
    {
      "role": "USER",
      "message": "How are you?"
    },
    {
      "role": "CHATBOT",
      "message": "This is Cohere's response to your message."
    }
  ],
  "finish_reason": "COMPLETE",
  "token_count": {
    "prompt_tokens": 68,
    "response_tokens": 59,
    "total_tokens": 127,
    "billed_tokens": 116
  },
  "meta": {
    "api_version": {
      "version": "1"
    },
    "billed_units": {
      "input_tokens": 57,
      "output_tokens": 59
    }
  }
}
```

#### Accesso al Contenuto
```javascript
const content = response.text;
```

#### Valori `finish_reason`
- `COMPLETE`: Completamento normale
- `MAX_TOKENS`: Raggiunto limite token
- `STOP_SEQUENCE`: Trovata sequenza di stop
- `ERROR`: Errore durante generazione
- `TOOL_CALL`: Chiamata a tool

---

## Tabelle Comparative

### Struttura della Richiesta

| Provider | Sistema Prompt | Formato Messaggi | JSON Mode | JSON Schema |
|----------|---------------|------------------|-----------|-------------|
| **OpenAI** | Nel messages array (`system` role) | `messages[].role/content` | ✅ `response_format: {type: "json_object"}` | ✅ `{type: "json_schema"}` |
| **Anthropic** | Campo `system` separato | `messages[].role/content` | ⚠️ Via prefilling | ❌ No |
| **Gemini** | `systemInstruction` separato | `contents[].parts[]` | ✅ `responseMimeType: "application/json"` | ✅ `responseSchema` |
| **Mistral** | Nel messages array (`system` role) | `messages[].role/content` | ✅ `response_format: {type: "json_object"}` | ✅ `{type: "json_schema"}` |
| **Cohere** | Campo `preamble` separato | `message` + `chat_history` | ✅ `response_format: {type: "json_object"}` | ✅ con `json_schema` |

### Struttura della Risposta

| Provider | Path per il Testo | Ruolo Risposta | Array Wrapper | Token Info |
|----------|------------------|----------------|---------------|------------|
| **OpenAI** | `choices[0].message.content` | `assistant` | `choices[]` | `usage.{prompt/completion/total}_tokens` |
| **Anthropic** | `content[0].text` | `assistant` | `content[]` | `usage.{input/output}_tokens` |
| **Gemini** | `candidates[0].content.parts[0].text` | `model` | `candidates[]` | `usageMetadata.{prompt/candidates/total}TokenCount` |
| **Mistral** | `choices[0].message.content` | `assistant` | `choices[]` | `usage.{prompt/completion/total}_tokens` |
| **Cohere** | `text` | N/A | Nessuno | `token_count.{prompt/response/total/billed}_tokens` |

### Parametri Comuni

| Parametro | OpenAI | Anthropic | Gemini | Mistral | Cohere |
|-----------|--------|-----------|--------|---------|--------|
| **Temperature** | `temperature` | `temperature` | `temperature` | `temperature` | `temperature` |
| **Max Tokens** | `max_tokens` | `max_tokens` | `maxOutputTokens` | `max_tokens` | `max_tokens` |
| **Top P** | `top_p` | `top_p` | `topP` | `top_p` | `p` |
| **Top K** | ❌ | `top_k` | `topK` | ❌ | `k` |
| **Stop Sequences** | `stop` | `stop_sequences` | ❌ | ❌ | `stop_sequences` |
| **Seed** | `seed` | ❌ | ❌ | `random_seed` | `seed` |
| **Stream** | `stream` | `stream` | ❌ | `stream` | `stream` |

### Motivi di Stop (finish_reason)

| Motivo | OpenAI | Anthropic | Gemini | Mistral | Cohere |
|--------|--------|-----------|--------|---------|--------|
| **Completamento Normale** | `stop` | `end_turn` | `STOP` | `stop` | `COMPLETE` |
| **Limite Token** | `length` | `max_tokens` | `MAX_TOKENS` | `length` | `MAX_TOKENS` |
| **Filtri Contenuto** | `content_filter` | ❌ | `SAFETY` | ❌ | ❌ |
| **Sequenza Stop** | ❌ | `stop_sequence` | ❌ | ❌ | `STOP_SEQUENCE` |
| **Tool/Function Call** | `tool_calls` | `tool_use` | ❌ | `tool_calls` | `TOOL_CALL` |
| **Errore** | ❌ | ❌ | ❌ | ❌ | `ERROR` |

---

## Note Importanti

### OpenAI
- Richiede la parola "JSON" nel prompt quando si usa `response_format: {type: "json_object"}`
- `system_fingerprint` aiuta a tracciare cambiamenti nel backend
- Supporta streaming con Server-Sent Events

### Anthropic
- **Non ha JSON mode nativo** - usa la tecnica del prefilling
- Il prompt di sistema è separato dall'array messages
- Header `anthropic-version` obbligatorio
- `content` è sempre un array (può contenere multiple parti)

### Google Gemini
- Usa `model` come ruolo invece di `assistant`
- Include sempre `safetyRatings` nella risposta
- `responseSchema` supporta un subset di OpenAPI 3.0
- Usa `to_dict()` per convertire l'oggetto response in JSON

### Mistral
- Formato molto simile a OpenAI
- `strict: true` nel JSON Schema garantisce aderenza stretta
- Supporta `safe_prompt` per filtraggio contenuti

### Cohere
- Struttura unica: messaggio singolo + cronologia separata
- Include `chat_history` nella risposta
- Distingue tra `token_count` e `billed_tokens`
- Ruoli in maiuscolo: `USER` e `CHATBOT`

---

## Esempi di Uso Pratico

### JavaScript/TypeScript

```javascript
// OpenAI
const openaiResponse = await fetch('https://api.openai.com/v1/chat/completions', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${OPENAI_API_KEY}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    model: 'gpt-4',
    messages: [{ role: 'user', content: 'Hello!' }]
  })
});
const openaiData = await openaiResponse.json();
console.log(openaiData.choices[0].message.content);

// Anthropic
const anthropicResponse = await fetch('https://api.anthropic.com/v1/messages', {
  method: 'POST',
  headers: {
    'x-api-key': ANTHROPIC_API_KEY,
    'anthropic-version': '2023-06-01',
    'content-type': 'application/json'
  },
  body: JSON.stringify({
    model: 'claude-sonnet-4-5',
    max_tokens: 1024,
    messages: [{ role: 'user', content: 'Hello!' }]
  })
});
const anthropicData = await anthropicResponse.json();
console.log(anthropicData.content[0].text);

// Gemini
const geminiResponse = await fetch(
  `https://generativelanguage.googleapis.com/v1/models/gemini-pro:generateContent?key=${GEMINI_API_KEY}`,
  {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      contents: [{ parts: [{ text: 'Hello!' }] }]
    })
  }
);
const geminiData = await geminiResponse.json();
console.log(geminiData.candidates[0].content.parts[0].text);
```

### Python

```python
import requests

# OpenAI
openai_response = requests.post(
    'https://api.openai.com/v1/chat/completions',
    headers={
        'Authorization': f'Bearer {OPENAI_API_KEY}',
        'Content-Type': 'application/json'
    },
    json={
        'model': 'gpt-4',
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
)
print(openai_response.json()['choices'][0]['message']['content'])

# Anthropic
anthropic_response = requests.post(
    'https://api.anthropic.com/v1/messages',
    headers={
        'x-api-key': ANTHROPIC_API_KEY,
        'anthropic-version': '2023-06-01',
        'content-type': 'application/json'
    },
    json={
        'model': 'claude-sonnet-4-5',
        'max_tokens': 1024,
        'messages': [{'role': 'user', 'content': 'Hello!'}]
    }
)
print(anthropic_response.json()['content'][0]['text'])
```

---

## Conclusione

Ogni provider ha le sue particolarità:

- **OpenAI e Mistral** condividono una struttura molto simile
- **Anthropic** ha un approccio unico con prefilling per JSON
- **Gemini** usa una struttura più complessa con `parts[]` e include safety ratings
- **Cohere** ha la struttura più diversa con messaggio singolo + cronologia

La scelta del provider dipende dalle tue esigenze specifiche in termini di:
- Capacità del modello
- Costi
- Supporto per JSON structured output
- Funzionalità aggiuntive (safety, tools, ecc.)