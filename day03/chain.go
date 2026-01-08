package main

import (
	"context"
	"fmt"
	"github.com/cloudwego/eino/callbacks"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()
	_ = godotenv.Load()
	// 1. 准备组件 A：翻译 Prompt
	translatePrompt := prompt.FromMessages(schema.FString,
		schema.SystemMessage("你是一个专业的翻译官，请将以下中文翻译成英文。"),
		schema.UserMessage("{input}"),
	)

	// 2. 准备组件 B：ChatModel
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("LLM_API_KEY"),
		Model:  os.Getenv("LLM_MODEL_ID"),
	})
	if err != nil {
		log.Fatalf("init model failed: %v", err)
	}

	// 3. 准备组件 C：润色 Prompt
	// 注意：这里我们假设上一步的输出会作为这一步的变量
	polishPrompt := prompt.FromMessages(schema.FString,
		schema.SystemMessage("你是一个英文学术编辑，请对输入的英文进行润色，使其更符合论文规范。"),
		schema.UserMessage("{input}"),
	)

	// 4. 使用 Compose 进行编排
	// NewChain 定义了输入类型为 map[string]any，输出类型为 *schema.Message
	chain := compose.NewChain[map[string]any, *schema.Message]()
	// 定义中间件函数，用于打印每个节点的输入输出
	handler := callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			log.Printf("[Global Start] component=%s name=%s input=%T", info.Component, info.Name, input)
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			log.Printf("[Global End] component=%s name=%s output=%T", info.Component, info.Name, output)
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			log.Printf("[Global Error] component=%s name=%s err=%v", info.Component, info.Name, err)
			return ctx
		}).
		Build()

	// Register as global callbacks (applies to all subsequent runs)
	callbacks.AppendGlobalHandlers(handler)
	chain.
		AppendChatTemplate(translatePrompt).                        // 注入 Prompt
		AppendChatModel(chatModel, compose.WithOutputKey("input")). // 注入模型，得到初稿
		AppendChatTemplate(polishPrompt).                           // 将初稿再次注入润色 Prompt
		AppendChatModel(chatModel)                                  // 再次调用模型，得到最终稿

	// 5. 编译并运行
	runnable, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("compile chain failed: %v", err)
	}

	// 执行流水线
	input := map[string]any{"input": "人工智能将深刻改变人类的生产方式。"}
	result, err := runnable.Invoke(ctx, input)
	if err != nil {
		log.Fatalf("run chain failed: %v", err)
	}

	fmt.Printf("\n[最终学术版]: %s\n", result.Content)
}
