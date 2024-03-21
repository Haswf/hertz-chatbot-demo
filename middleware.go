package main

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
)

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
		// 3. ChatID将写入 Header
		c.Response.Header.Set("x-chat-id", chatID)

		c.Next(ctx)
		// 4. 将 Query 和 Response 写入聊天记录
		query := c.GetString("query")
		response := c.GetString("response")

		history = append(history, llms.TextParts(schema.ChatMessageTypeHuman, query), llms.TextParts(schema.ChatMessageTypeAI, response))
		historyStore.Set(chatID, history, cache.DefaultExpiration)
	}
}
