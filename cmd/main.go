package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/Arvintian/go-utils/cmdutil"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

type ChatGPTWebServer struct {
	Host              string `name:"host" usage:"http bind host" default:"0.0.0.0"`
	Port              int    `name:"port" usage:"http bind port" default:"7080"`
	BasicAuthUser     string `name:"auth-user" env:"BASIC_USER" usage:"http basic auth user"`
	BasicAuthPassword string `name:"auth-password" env:"BASIC_PASSWORD" usage:"http basic auth password"`
	BackendServer     string `name:"backend-server" default:"http://127.0.0.1:3002" usage:"backend's server endpoint"`
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

	//proxy to upstream
	server := &http.Server{
		Addr: fmt.Sprintf("%s:%d", r.Host, r.Port),
	}
	entry := gin.New()
	entry.Use(gin.Logger())
	entry.Use(gin.Recovery())
	if len(r.BasicAuthUser) > 0 {
		entry.Use(gin.BasicAuth(gin.Accounts{
			r.BasicAuthUser: r.BasicAuthPassword,
		}))
	}
	entry.NoRoute(func(ctx *gin.Context) {
		serverProxy.ServeHTTP(ctx.Writer, ctx.Request)
	})

	//listen and serve
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
