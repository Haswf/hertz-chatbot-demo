# 使用 Hertz 实现 ChatBot

## 安装 Ollama
1. 使用 Brew 安装 ollama，其他平台可以参考[Ollama文档](https://github.com/ollama/ollama)

``` bash
brew install ollama
```

2. 拉取 llama2 模型镜像
``` bash
ollama pull llama2
#pulling manifest
#pulling 8934d96d3f08... 100% ▕██████████████████████████████████████████████████████████▏ 3.8 GB
#pulling 8c17c2ebb0ea... 100% ▕██████████████████████████████████████████████████████████▏ 7.0 KB
#pulling 7c23fb36d801... 100% ▕██████████████████████████████████████████████████████████▏ 4.8 KB
#pulling 2e0493f67d0c... 100% ▕██████████████████████████████████████████████████████████▏   59 B
#pulling fa304d675061... 100% ▕██████████████████████████████████████████████████████████▏   91 B
#pulling 42ba7f8a01dd... 100% ▕██████████████████████████████████████████████████████████▏  557 B
#verifying sha256 digest
#writing manifest
#removing any unused layers
#success
```

3. 启动 Ollama
``` bash
ollama serve
#time=2024-03-20T19:28:37.444+08:00 level=INFO source=images.go:806 msg="total blobs: 6"
#time=2024-03-20T19:28:37.446+08:00 level=INFO source=images.go:813 msg="total unused blobs removed: 0"
#time=2024-03-20T19:28:37.447+08:00 level=INFO source=routes.go:1110 msg="Listening on 127.0.0.1:11434 (version 0.1.29)"
#time=2024-03-20T19:28:37.448+08:00 level=INFO source=payload_common.go:112 msg="Extracting dynamic libraries to /var/folders/kt/cv_fsbpj7d1_pwv4sdrlgvgr0000gp/T/ollama2560105980/runners ..."
#time=2024-03-20T19:28:37.499+08:00 level=INFO source=payload_common.go:139 msg="Dynamic LLM libraries [cpu_avx cpu cpu_avx2]"
```

4. 通过 `curl` 命令验证服务是否能正确响应请求
``` bash
curl http://localhost:11434/api/generate -d '{
    "model": "llama2",
    "prompt": "Hello their, reply yes or no",
    "stream": true
}

#{"model":"llama2","created_at":"2024-03-20T11:21:28.278254Z","response":"Yes","done":false}
#{"model":"llama2","created_at":"2024-03-20T11:21:28.429117Z","response":".","done":false}
#{"model":"llama2","created_at":"2024-03-20T11:21:28.595316Z","response":"","done":true,"context":[518,25580,29962,3532,14816,29903,29958,#5299,829,14816,29903,6778,13,13,10994,1009,29892,8908,4874,470,694,518,29914,25580,29962,13,8241,29889],"total_duration":486825273,"load_duration":634110,"prompt_eval_duration":168786000,"eval_count":3,"eval_duration":316676000}
```

## 使用 langchaingo 调用大模型

1. 初始化模型，注册路由

``` go

var llm llms.Model

func main() {
	h := server.Default()
	var err error

	// 初始化 llama2 模型
	llm, err = ollama.New(ollama.WithModel("llama2"))
	if err != nil {
		panic(err)
	}

    // 注册路由
	h.POST("/single_prompt", SinglePrompt)

	h.Spin()
}

```

2. 使用 SSE 中间件流式返回响应

``` go

type ChatReq struct {
	Query string `json:"query"`
}

func SinglePrompt(ctx context.Context, c *app.RequestContext) {
	var req ChatReq
	err := c.BindAndValidate(&req)
	// ...

	c.SetStatusCode(http.StatusOK)
	s := sse.NewStream(c)

	response, err := llms.GenerateFromSinglePrompt(ctx, llm, req.Query,
      llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		return s.Publish(&sse.Event{
			Event: "chunk",
			Data:  chunk,
		})
	}))
	// ...
}

```

3. 使用 curl 请求接口

``` bash
curl --location 'http://127.0.0.1:8888/single_prompt' \
--header 'Content-Type: application/json' \
--data '{
    "query": "describe quicksort in 10 words"
}'
#event:chunk
#data:Sw

#event:chunk
#data:ift

#event:chunk
#data: and

#event:chunk
#data: efficient

#event:chunk
#data: sorting

#event:chunk
#data: algorithm

#event:chunk
#data:.

#event:chunk
#data:

#event:full
#data:Swift and efficient sorting algorithm.
```

### 使用中间件保存上下文
1. 使用中间件生成 ChatID header，根据 ChatID 查找和保存聊天记录
``` go

func ChatContextRecorder() app.HandlerFunc {
	historyStore := cache.New(5*time.Minute, 10*time.Minute)

	return func(ctx context.Context, c *app.RequestContext) {
		// 1. 提取或生成 ChatID
		chatID := string(c.GetHeader("x-chat-id"))
		var history []llms.MessageContent
		if len(chatID) == 0 {
			chatID = uuid.New().String()
		}
		// 2. 读取 ChatID 关联的聊天记录
		result, found := historyStore.Get(string(chatID))
		if found {
			history = result.([]llms.MessageContent)
			c.Set("history", history)
		}
		// 3. 添加 x-chat-id Header
		c.Response.Header.Set("x-chat-id", chatID)

		c.Next(ctx)
		// 4. 将 Query 和 Response 写入聊天记录
		query := c.GetString("query")
		response := c.GetString("response")

		history = append(history, llms.TextParts(schema.ChatMessageTypeHuman, query), llms.TextParts(schema.ChatMessageTypeAI, response))
		historyStore.Set(chatID, history, cache.DefaultExpiration)

	}
}
```

2. 使用中间件在请求上下文中注入的历史消息
``` go

func Chat(ctx context.Context, c *app.RequestContext) {
	// ...
	c.SetStatusCode(http.StatusOK)
	s := sse.NewStream(c)

	var message []llms.MessageContent
	history, found := c.Get("history")
	if found {
		message = history.([]llms.MessageContent)
	}

	message = append(message, llms.TextParts(schema.ChatMessageTypeHuman, req.Query))

	resp, err := llm.GenerateContent(ctx, message, llms.WithStreamingFunc(
		func(ctx context.Context, chunk []byte) error {
		return s.Publish(&sse.Event{
			Event: "chunk",
			Data:  chunk,
		})
	}))
	// ...

	if len(resp.Choices) > 0 {
		c.Set("query", req.Query)
		c.Set("response", resp.Choices[0].Content)
	}
}
```

3. 注册中间件和路由

``` go

var llm llms.Model

func main() {
	h := server.Default()	
    // ...
	h.POST("/chat", ChatContextRecorder(), Chat)
    // ...
	h.Spin()
}
```
