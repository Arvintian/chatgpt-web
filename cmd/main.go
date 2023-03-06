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

	"github.com/Arvintian/go-utils/cmdutil"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

type ChatGPTWebServer struct {
	Host              string `name:"host" env:"SERVER_HOST" usage:"http bind host" default:"0.0.0.0"`
	Port              int    `name:"port" env:"SERVER_PORT" usage:"http bind port" default:"7080"`
	BasicAuthUser     string `name:"auth-user" env:"BASIC_AUTH_USER" usage:"http basic auth user"`
	BasicAuthPassword string `name:"auth-password" env:"BASIC_AUTH_PASSWORD" usage:"http basic auth password"`
	BackendServer     string `name:"backend-server" default:"http://127.0.0.1:3002" usage:"backend's server endpoint"`
	BackendCMD        string `name:"backend-cmd" default:"pnpm,run,start" usage:"backend's server command"`
	BackendPath       string `name:"backend-path" default:"/app/public" usage:"backend's server path"`
	Version           bool   `name:"version" usage:"show version"`
}

var Version = "0.0.0-dev"

func (r *ChatGPTWebServer) Run(cmd *cobra.Command, args []string) error {
	if r.Version {
		return r.ShowVersion()
	}
	if r.BackendServer == "" {
		fmt.Printf("Version: %s\n\n", Version)
		return cmd.Help()
	}
	gin.SetMode(gin.ReleaseMode)
	if err := r.updateAssetsFiles(); err != nil {
		return err
	}
	go r.startBackend(cmd.Context())
	go r.httpServer(cmd.Context())

	<-cmd.Context().Done()
	return nil
}

func (r *ChatGPTWebServer) httpServer(ctx context.Context) {
	serverURL, err := url.Parse(r.BackendServer)
	if err != nil {
		log.Fatal(err)
	}
	serverProxy := httputil.NewSingleHostReverseProxy(serverURL)

	addr := fmt.Sprintf("%s:%d", r.Host, r.Port)
	klog.Infof("ChatGPT Web Server on: %s", addr)
	server := &http.Server{
		Addr: addr,
	}
	entry := gin.New()
	entry.Use(gin.Logger())
	entry.Use(gin.Recovery())
	apis := entry.Group("/api")
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
		apis.Use(gin.BasicAuth(accounts))
	}
	apis.POST("/chat-process", func(ctx *gin.Context) {
		serverProxy.ServeHTTP(ctx.Writer, ctx.Request)
	})
	entry.NoRoute(func(ctx *gin.Context) {
		serverProxy.ServeHTTP(ctx.Writer, ctx.Request)
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

func (r *ChatGPTWebServer) startBackend(ctx context.Context) {
	args := strings.Split(r.BackendCMD, ",")
	klog.Infof("Start Backend with %v", args)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		os.Exit(1)
		klog.Error(err)
	}
}

func (r *ChatGPTWebServer) updateAssetsFiles() error {
	pairs := map[string]string{}
	old := `{avatar:"https://raw.githubusercontent.com/Chanzhaoyu/chatgpt-web/main/src/assets/avatar.jpg",name:"ChenZhaoYu",description:'Star on <a href="https://github.com/Chanzhaoyu/chatgpt-bot" class="text-blue-500" target="_blank" >Github</a>'}`
	new := `{avatar:"https://raw.githubusercontent.com/Chanzhaoyu/chatgpt-web/main/src/assets/avatar.jpg",name:"ChatGPT",description:'知之为知之'}`
	pairs[old] = new
	old = `<title>ChatGPT Web</title>`
	new = `<title>ChatGPT</title>`
	pairs[old] = new
	return replaceFiles(r.BackendPath, pairs)
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
