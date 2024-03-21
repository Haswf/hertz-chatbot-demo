package main

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/sse"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
)

type ChatReq struct {
	Query string `json:"query"`
}

func Chat(ctx context.Context, c *app.RequestContext) {
	var req ChatReq
	err := c.BindAndValidate(&req)
	if err != nil {
		renderError(c, http.StatusBadRequest, err)
		return
	}

	c.SetStatusCode(http.StatusOK)
	s := sse.NewStream(c)

	var message []llms.MessageContent
	history, found := c.Get("history")
	if found {
		message = history.([]llms.MessageContent)
	}
	message = append(message, llms.TextParts(schema.ChatMessageTypeHuman, req.Query))

	resp, err := llm.GenerateContent(ctx, message, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		return s.Publish(&sse.Event{
			Event: "chunk",
			Data:  chunk,
		})
	}))
	if err != nil {
		hlog.CtxErrorf(ctx, "failed to generate: %s", err)
		return
	}

	if len(resp.Choices) > 0 {

		err = s.Publish(&sse.Event{
			Event: "full",
			Data:  []byte(resp.Choices[0].Content),
		})
		if err != nil {
			hlog.CtxErrorf(ctx, "failed to publish: %s", err)
			return
		}
		c.Set("response", resp.Choices[0].Content)
	}
}

func SinglePrompt(ctx context.Context, c *app.RequestContext) {
	var req ChatReq
	err := c.BindAndValidate(&req)
	if err != nil {
		renderError(c, http.StatusBadRequest, err)
		return
	}

	c.SetStatusCode(http.StatusOK)
	s := sse.NewStream(c)

	response, err := llms.GenerateFromSinglePrompt(ctx, llm, req.Query,
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			return s.Publish(&sse.Event{
				Event: "chunk",
				Data:  chunk,
			})
		}))
	if err != nil {
		hlog.CtxErrorf(ctx, "failed to generate: %s", err)
		return
	}
	err = s.Publish(&sse.Event{
		Event: "full",
		Data:  []byte(response),
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "failed to publish: %s", err)
		return
	}
}

func renderError(c *app.RequestContext, status int, err error) {
	c.JSON(status, map[string]interface{}{
		"message": err,
	})
}
