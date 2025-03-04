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
	"path"
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
	OpsKey                 string `name:"ops-key" env:"OPS_KEY" default:"admin" usage:"ops key"`
	OpsLink                string `name:"ops-link" env:"OPS_LINK" default:"/admin" usage:"ops link"`
	DataBase               string `name:"db" env:"DB" default:"/data/chatgpt.db" usage:"mysql database url or sqlite path, user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"`
	FrontendPath           string `name:"frontend-path" env:"FRONTEND_PATH" default:"/app/public" usage:"frontend path"`
	SocksProxy             string `name:"socks-proxy" env:"SOCKS_PROXY" usage:"socks proxy url"`
	ChatSessionTTL         int    `name:"chat-session-ttl" env:"CHAT_SESSION_TTL" default:"30" usage:"chat session ttl minute"`
	ChatMinResponseTokens  int    `name:"chat-min-response-tokens" env:"CHAT_MIN_RESPONSE_TOKENS" default:"600" usage:"chat min response tokens"`
	OpenAIKey              string `name:"openai-key" env:"OPENAI_KEY" usage:"openai key"`
	OpenAIBaseURL          string `name:"openai-base-url" env:"OPENAI_BASE_URL" default:"https://api.openai.com/v1" usage:"openai base url"`
	OpenAIModel            string `name:"openai-model" env:"OPENAI_MODEL" default:"gpt-3.5-turbo" usage:"openai params model"`
	OpenAIMaxTokens        int    `name:"openai-max-tokens" env:"OPENAI_MAX_TOKENS" default:"4096" usage:"openai params max-tokens"`
	OpenAITemperature      int    `name:"openai-temperature" env:"OPENAI_TEMPERATURE" default:"80" usage:"openai params temperature"`
	OpenAIPresencePenalty  int    `name:"openai-presence-penalty" env:"OPENAI_PRESENCE_PENALTY" default:"100" usage:"openai params presence-penalty"`
	OpenAIFrequencyPenalty int    `name:"openai-frequency-penalty" env:"OPENAI_FREQUENCY_PENALTY" default:"0" usage:"openai params frequency-penalty"`
	OpenAIProxy            bool   `name:"openai-proxy" env:"OPENAI_PROXY" usage:"enable proxy openai api"`
	Version                bool   `name:"version" usage:"show version"`
}

var Version = "0.0.0-dev"

func (r *ChatGPTWebServer) Run(cmd *cobra.Command, args []string) error {
	if r.Version {
		return r.ShowVersion()
	}
	gin.SetMode(gin.ReleaseMode)
	if err := r.updateAssetsFiles(r.OpsLink); err != nil {
		return err
	}
	go r.startTokenizer(cmd.Context())
	go r.httpServer(cmd.Context())

	<-cmd.Context().Done()
	return nil
}

func (r *ChatGPTWebServer) httpServer(ctx context.Context) {
	accountService, err := controllers.NewAccountService(r.DataBase, r.BasicAuthUser, r.BasicAuthPassword)
	if err != nil {
		klog.Fatal(err)
	}
	chatService, err := controllers.NewChatService(r.OpenAIKey, r.OpenAIBaseURL, r.SocksProxy, controllers.ChatCompletionParams{
		Model:                 r.OpenAIModel,
		MaxTokens:             r.OpenAIMaxTokens,
		Temperature:           float32(r.OpenAITemperature) / 100.0,
		PresencePenalty:       float32(r.OpenAIPresencePenalty) / 100.0,
		FrequencyPenalty:      float32(r.OpenAIFrequencyPenalty) / 100.0,
		ChatSessionTTL:        time.Duration(r.ChatSessionTTL) * time.Minute,
		ChatMinResponseTokens: r.ChatMinResponseTokens,
	}, accountService)
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
	chat.POST("/chat-process", BasicAuth(accountService, r.OpsLink), middlewares.RateLimitMiddleware(1, 2), chatService.ChatProcess)
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
	entry.POST("/accounts", OpsAuth(r.OpsKey), accountService.AccountProcess)
	entry.Any("/admin/*relativePath", gin.BasicAuth(gin.Accounts{"admin": r.OpsKey}), func(ctx *gin.Context) {
		if ctx.Request.URL.Path == "/admin/accounts" {
			accountService.AccountProcess(ctx)
		} else {
			http.FileServer(http.Dir(path.Join(r.FrontendPath))).ServeHTTP(ctx.Writer, ctx.Request)
		}
	})
	if r.OpenAIProxy {
		klog.Info("enable proxy openai api server")
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
	} else {
		entry.NoRoute(func(ctx *gin.Context) {
			http.FileServer(http.Dir(r.FrontendPath)).ServeHTTP(ctx.Writer, ctx.Request)
		})
	}

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

func (r *ChatGPTWebServer) updateAssetsFiles(link string) error {
	pairs := map[string]string{}
	old := `{avatar:"https://raw.githubusercontent.com/Chanzhaoyu/chatgpt-web/main/src/assets/avatar.jpg",name:"ChenZhaoYu",description:'Star on <a href="https://github.com/Chanzhaoyu/chatgpt-bot" class="text-blue-500" target="_blank" >Github</a>'}`
	new := fmt.Sprintf(`{avatar:"https://raw.githubusercontent.com/Chanzhaoyu/chatgpt-web/main/src/assets/avatar.jpg",name:"获取帮助输入/help",description:'<a href="%s" class="text-blue-500" target="_blank" >自助中心</a>'}`, link)
	pairs[old] = new
	old = `{}.VITE_GLOB_OPEN_LONG_REPLY`
	new = `{VITE_GLOB_OPEN_LONG_REPLY:"true"}.VITE_GLOB_OPEN_LONG_REPLY`
	pairs[old] = new
	old = `<link rel="manifest" href="/manifest.webmanifest"><script id="vite-plugin-pwa:register-sw" src="/registerSW.js"></script>`
	new = ``
	pairs[old] = new
	old = `[y(" 此项目开源于 "),e("a",{class:"text-blue-600 dark:text-blue-500",href:"https://github.com/Chanzhaoyu/chatgpt-web",target:"_blank"}," Github "),y(" ，免费且基于 MIT 协议，没有任何形式的付费行为！ ")]`
	new = `[y(" 此项目开源于 "),e("a",{class:"text-blue-600 dark:text-blue-500",href:"https://github.com/Arvintian/chatgpt-web",target:"_blank"}," Github ")]`
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
