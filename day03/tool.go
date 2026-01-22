package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// 1. 定义工具参数结构体（Eino 会自动解析 Tag 生成 Schema）
type WeatherArgs struct {
	Location string `json:"location" jsonschema:"description=城市名,比如北京"`
}

// 2. 定义工具输出结构体
type WeatherReply struct {
	Weather string `json:"weather"`
}

func main() {
	ctx := context.Background()
	_ = godotenv.Load()

	// 3. 创建工具实例 (Invoker)
	// Eino 的 utils.NewTool 能够将普通的 Go 函数封装成可调用的工具
	weatherTool := utils.NewTool(&schema.ToolInfo{
		Name: "get_weather",
		Desc: "根据城市名查询天气",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"location": {
				Desc: "城市名",
				Type: schema.String,
			},
		}),
	}, func(ctx context.Context, args *WeatherArgs) (*WeatherReply, error) {
		// 这里接入实际的 API，现在模拟返回
		return &WeatherReply{Weather: fmt.Sprintf("%s今天晴转多云，25度", args.Location)}, nil
	})

	// 4. 初始化支持工具的模型
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("LLM_API_KEY"),
		Model:  os.Getenv("LLM_MODEL_ID"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 5. 将工具绑定到模型
	// 注意：BindTools 是 Eino 组件组合的关键步骤
	info, err := weatherTool.Info(ctx)
	if err != nil {
		log.Fatalf("Failed to get tool info: %v", err)
	}
	err = chatModel.BindTools([]*schema.ToolInfo{info})

	// 6. 测试调用
	// 当用户问天气时，模型会触发 Tool Call
	input := []*schema.Message{
		schema.UserMessage("北京的天气怎么样？"),
	}

	res, err := chatModel.Generate(ctx, input)
	if err != nil {
		log.Fatalf("generate failed: %v", err)
	}

	// 7. 打印结果
	// 如果配置正确，res.ToolCalls 会包含工具请求，Eino 在更高阶的 Graph 中会自动处理这些调用
	fmt.Printf("模型响应内容: %v\n", res)
	for _, tc := range res.ToolCalls {
		fmt.Printf("模型请求调用工具: %s, 参数: %s\n", tc.Function.Name, tc.Function.Arguments)
		toolReply, err := weatherTool.InvokableRun(ctx, tc.Function.Arguments)
		if err != nil {
			log.Printf("工具调用失败: %v", err)
		}
		fmt.Printf("工具响应内容: %s\n", toolReply)
	}
}
