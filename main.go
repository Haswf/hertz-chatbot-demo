package main

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/google/uuid"
	"github.com/hertz-contrib/sse"
	"github.com/patrickmn/go-cache"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/schema"
)

type ChatReq struct {
	Query string `json:"query"`
}

func renderError(c *app.RequestContext, status int, err error) {
	c.JSON(status, map[string]interface{}{
		"message": err,
	})
}

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
		// 3. Header 写入ChatID
		c.Response.Header.Set("x-chat-id", chatID)

		c.Next(ctx)

		// 4. 将 Query 和 Response 写入聊天记录
		query := c.GetString("query")
		response := c.GetString("response")

		history = append(history, llms.TextParts(schema.ChatMessageTypeHuman, query), llms.TextParts(schema.ChatMessageTypeAI, response))
		historyStore.Set(chatID, history, cache.DefaultExpiration)

	}
}

func main() {
	h := server.Default()
	h.Use(ChatContextRecorder())
	llm, err := ollama.New(ollama.WithModel("gemma"))
	if err != nil {
		panic(err)
	}

	h.POST("/sse", func(ctx context.Context, c *app.RequestContext) {
		c.SetStatusCode(http.StatusOK)
		s := sse.NewStream(c)

		var req ChatReq
		err = c.BindAndValidate(&req)
		if err != nil {
			renderError(c, http.StatusBadRequest, err)
			return
		}

		var message []llms.MessageContent
		history, found := c.Get("history")
		if found {
			message = history.([]llms.MessageContent)
		}
		message = append(message, llms.TextParts(schema.ChatMessageTypeHuman, req.Query))
		for _, msg := range message {
			hlog.CtxInfof(ctx, "%s msg: %s", msg.Role, msg.Parts)
		}

		resp, err := llm.GenerateContent(ctx, message, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			return s.Publish(&sse.Event{
				Event: "chunk",
				Data:  chunk,
			})
		}))
		if err != nil {
			hlog.CtxErrorf(ctx, "failed to call llm: %s", err)
			return
		}

		if len(resp.Choices) > 0 {

			s.Publish(&sse.Event{
				Event: "full",
				Data:  []byte(resp.Choices[0].Content),
			})
			c.Set("response", resp.Choices[0].Content)
		}

	})

	h.Spin()
}
