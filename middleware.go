/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2015-present Aliaksandr Valialkin, VertaMedia, Kirill Danshin, Erik Dubbelboer, FastHTTP Authors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

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
