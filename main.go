package main

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

var llm llms.Model

func main() {
	h := server.Default()
	var err error
	llm, err = ollama.New(ollama.WithModel("llama2"))
	if err != nil {
		panic(err)
	}
	h.POST("/single_prompt", SinglePrompt)
	h.POST("/chat", ChatContextRecorder(), Chat)

	h.Spin()
}
