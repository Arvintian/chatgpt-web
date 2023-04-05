package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Arvintian/chatgpt-web/pkg/controllers"
	"github.com/Arvintian/chatgpt-web/pkg/middlewares"
	"github.com/Arvintian/chatgpt-web/pkg/utils"
	"github.com/Arvintian/go-utils/cmdutil"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

type ChatGPTWebServer struct {
	Host                   string `name:"host" env:"SERVER_HOST" usage:"http bind host" default:"0.0.0.0"`
	Port                   int    `name:"port" env:"SERVER_PORT" usage:"http bind port" default:"7080"`
	BasicAuthUser          string `name:"auth-user" env:"BASIC_AUTH_USER" usage:"http basic auth user"`
	BasicAuthPassword      string `name:"auth-password" env:"BASIC_AUTH_PASSWORD" usage:"http basic auth password"`
	FrontendPath           string `name:"frontend-path" env:"FRONTEND_PATH" default:"/app/public" usage:"frontend path"`
	SocksProxy             string `name:"socks-proxy" env:"SOCKS_PROXY" usage:"socks proxy url"`
	ChatSessionTTL         int    `name:"chat-session-ttl" env:"CHAT_SESSION_TTL" default:"30" usage:"chat session ttl minute"`
	ChatMinResponseTokens  int    `name:"chat-min-response-tokens" env:"CHAT_MIN_RESPONSE_TOKENS" default:"600" usage:"chat min response tokens"`
	OpenAIKey              string `name:"openapi-key" env:"OPENAI_KEY" usage:"openai key"`
	OpenAIBaseURL          string `name:"openapi-base-url" env:"OPENAI_BASE_URL" default:"https://api.openai.com/v1" usage:"openai base url"`
	OpenAIModel            string `name:"openai-model" env:"OPENAI_MODEL" default:"gpt-3.5-turbo-0301" usage:"openai params model"`
	OpenAIMaxTokens        int    `name:"openai-max-tokens" env:"OPENAI_MAX_TOKENS" default:"4096" usage:"openai params max-tokens"`
	OpenAITemperature      int    `name:"openai-temperature" env:"OPENAI_TEMPERATURE" default:"80" usage:"openai params temperature"`
	OpenAIPresencePenalty  int    `name:"openai-presence-penalty" env:"OPENAI_PRESENCE_PENALTY" default:"100" usage:"openai params presence-penalty"`
	OpenAIFrequencyPenalty int    `name:"openai-frequency-penalty" env:"OPENAI_FREQUENCY_PENALTY" default:"0" usage:"openai params frequency-penalty"`
	Version                bool   `name:"version" usage:"show version"`
}

var Version = "0.0.0-dev"

func (r *ChatGPTWebServer) Run(cmd *cobra.Command, args []string) error {
	if r.Version {
		return r.ShowVersion()
	}
	gin.SetMode(gin.ReleaseMode)
	if err := r.updateAssetsFiles(); err != nil {
		return err
	}
	go r.startTokenizer(cmd.Context())
	go r.httpServer(cmd.Context())

	<-cmd.Context().Done()
	return nil
}

func (r *ChatGPTWebServer) httpServer(ctx context.Context) {
	chatService, err := controllers.NewChatService(r.OpenAIKey, r.OpenAIBaseURL, r.SocksProxy, controllers.ChatCompletionParams{
		Model:                 r.OpenAIModel,
		MaxTokens:             r.OpenAIMaxTokens,
		Temperature:           float32(r.OpenAITemperature) / 100.0,
		PresencePenalty:       float32(r.OpenAIPresencePenalty) / 100.0,
		FrequencyPenalty:      float32(r.OpenAIFrequencyPenalty) / 100.0,
		ChatSessionTTL:        time.Duration(r.ChatSessionTTL) * time.Minute,
		ChatMinResponseTokens: r.ChatMinResponseTokens,
	})
	if err != nil {
		klog.Fatal(err)
	}

	addr := fmt.Sprintf("%s:%d", r.Host, r.Port)
	klog.Infof("ChatGPT Web Server on: %s", addr)
	server := &http.Server{
		Addr: addr,
	}
	entry, proxy := gin.New(), gin.New()
	entry.Use(gin.Logger())
	entry.Use(gin.Recovery())
	chat := entry.Group("/api")
	if len(r.BasicAuthUser) > 0 {
		accounts := gin.Accounts{}
		users := strings.Split(r.BasicAuthUser, ",")
		passwords := strings.Split(r.BasicAuthPassword, ",")
		if len(users) != len(passwords) {
			panic("basic auth setting error")
		}
		for i := 0; i < len(users); i++ {
			accounts[users[i]] = passwords[i]
		}
		chat.POST("/chat-process", gin.BasicAuth(accounts), middlewares.RateLimitMiddleware(1, 2), chatService.ChatProcess)
	} else {
		chat.POST("/chat-process", middlewares.RateLimitMiddleware(1, 2), chatService.ChatProcess)
	}
	chat.POST("/config", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"status": "Success",
			"data": map[string]string{
				"apiModel":   "ChatGPTAPI",
				"socksProxy": r.SocksProxy,
			},
		})
	})
	chat.POST("/session", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"status":  "Success",
			"message": "",
			"data": gin.H{
				"auth": false,
			},
		})
	})
	upstreamURL, err := url.Parse(strings.TrimSuffix(r.OpenAIBaseURL, "/v1"))
	if err != nil {
		klog.Fatal(err)
	}
	upstream := httputil.NewSingleHostReverseProxy(upstreamURL)
	if r.SocksProxy != "" {
		proxyUrl, err := url.Parse(r.SocksProxy)
		if err != nil {
			klog.Fatal(err)
		}
		upstream.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
	}
	apis := proxy.Group("/v1")
	apis.Any("/*relativePath", func(ctx *gin.Context) {
		ctx.Request.Host = upstreamURL.Host
		upstream.ServeHTTP(ctx.Writer, ctx.Request)
	})
	proxy.NoRoute(func(ctx *gin.Context) {
		http.FileServer(http.Dir(r.FrontendPath)).ServeHTTP(ctx.Writer, ctx.Request)
	})
	entry.NoRoute(func(ctx *gin.Context) {
		proxy.ServeHTTP(ctx.Writer, ctx.Request)
	})

	server.Handler = entry
	go func(ctx context.Context) {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown with error %v", err)
		}
	}(ctx)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server listen and serve error %v", err)
	}
}

func (r *ChatGPTWebServer) startTokenizer(ctx context.Context) {
	// devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0755)
	// if err != nil {
	// 	klog.Error(err)
	// 	os.Exit(1)
	// }
	args := strings.Split("nuxt --module tokenizer.py --workers 2", " ")
	klog.Infof("Start Tokenizer with %v", args)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func (r *ChatGPTWebServer) updateAssetsFiles() error {
	pairs := map[string]string{}
	old := `{avatar:"https://raw.githubusercontent.com/Chanzhaoyu/chatgpt-web/main/src/assets/avatar.jpg",name:"ChenZhaoYu",description:'Star on <a href="https://github.com/Chanzhaoyu/chatgpt-bot" class="text-blue-500" target="_blank" >Github</a>'}`
	new := `{avatar:"https://raw.githubusercontent.com/Chanzhaoyu/chatgpt-web/main/src/assets/avatar.jpg",name:"ChatGPT",description:'Star on <a href="https://github.com/Arvintian/chatgpt-web" class="text-blue-500" target="_blank" >Github</a>'}`
	pairs[old] = new
	old = `{}.VITE_GLOB_OPEN_LONG_REPLY`
	new = `{VITE_GLOB_OPEN_LONG_REPLY:"true"}.VITE_GLOB_OPEN_LONG_REPLY`
	pairs[old] = new
	old = `<link rel="manifest" href="/manifest.webmanifest"><script id="vite-plugin-pwa:register-sw" src="/registerSW.js"></script>`
	new = ``
	pairs[old] = new
	return utils.ReplaceFiles(r.FrontendPath, pairs)
}

func (r *ChatGPTWebServer) ShowVersion() error {
	fmt.Println(Version)
	return nil
}

func main() {
	root := cmdutil.Command(&ChatGPTWebServer{}, cobra.Command{
		Long: "ChatGPT Web Server",
	})
	cmdutil.Main(root)
}
