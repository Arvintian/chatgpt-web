package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Arvintian/chatgpt-web/pkg/controllers"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

var help = `#### 帮助命令
- /help 获取帮助信息
- /me 获取用户信息
- /usage 获取Token余额
- /user 新账户:新密码 更改账户、密码
- /login 登录、重新登录
`

func BasicAuth(ac *controllers.AccountService) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			klog.Error(err)
			c.JSON(http.StatusOK, gin.H{
				"status":  "Fail",
				"message": "内部错误",
				"data":    nil,
			})
			c.Abort()
			return
		}
		payload := controllers.ChatMessageRequest{}
		if err := json.Unmarshal(body, &payload); err == nil && strings.HasPrefix(payload.Prompt, "/") {
			switch payload.Prompt {
			case "/help":
				c.JSON(http.StatusOK, gin.H{
					"status":  "Fail",
					"message": help,
					"data":    nil,
				})
				c.Abort()
				return
			case "/me":
				username, password, ok := c.Request.BasicAuth()
				message := "未登录"
				if ok {
					if user, err := ac.GetUser(username, password); err != nil {
						message = "用户信息错误"
					} else {
						message = fmt.Sprintf("账户: %s\nToken余额: %d", user.Username, user.Balance-user.Usage)
					}
				}
				c.JSON(http.StatusOK, gin.H{
					"status":  "Fail",
					"message": message,
					"data":    nil,
				})
				c.Abort()
				return
			case "/usage":
				username, password, ok := c.Request.BasicAuth()
				message := "未登录"
				if ok {
					if user, err := ac.GetUser(username, password); err != nil {
						message = "用户信息错误"
					} else {
						message = fmt.Sprintf("Token余额: %d", user.Balance-user.Usage)
					}
				}
				c.JSON(http.StatusOK, gin.H{
					"status":  "Fail",
					"message": message,
					"data":    nil,
				})
				c.Abort()
				return
			case "/login":
				_, err := c.Request.Cookie("dologin")
				if err != nil {
					c.Header("WWW-Authenticate", "Basic realm=\"Restricted\"")
					cookie := &http.Cookie{
						Name:    "dologin",
						Value:   "yes",
						Expires: time.Now().Add(24 * time.Hour),
						Path:    "/",
					}
					http.SetCookie(c.Writer, cookie)
					c.AbortWithStatus(http.StatusUnauthorized)
				} else {
					cookie := &http.Cookie{
						Name:   "dologin",
						Value:  "yes",
						MaxAge: -1,
						Path:   "/",
					}
					http.SetCookie(c.Writer, cookie)
					c.JSON(http.StatusOK, gin.H{
						"status":  "Fail",
						"message": "登录成功",
						"data":    nil,
					})
					c.Abort()
				}
				return
			}
			if strings.HasPrefix(payload.Prompt, "/user") {
				username, password, ok := c.Request.BasicAuth()
				if !ok {
					c.JSON(http.StatusOK, gin.H{
						"status":  "Fail",
						"message": "未登录",
						"data":    nil,
					})
					c.Abort()
					return
				}
				if _, err := ac.GetUser(username, password); err != nil {
					c.JSON(http.StatusOK, gin.H{
						"status":  "Fail",
						"message": "当前用户信息错误,请输入/login登录其他账户",
						"data":    nil,
					})
					c.Abort()
					return
				}
				name, passwd, err := ExtractAccountAndPassword(payload.Prompt)
				if err != nil {
					c.JSON(http.StatusOK, gin.H{
						"status":  "Fail",
						"message": fmt.Sprintf("%v\n\n账户格式:大小写字母和数字4-12位,必须字母开头\n密码格式:大小写字母和数字6-12位", err),
						"data":    nil,
					})
					c.Abort()
					return
				}
				if err := ac.UpdateUser(username, password, name, passwd); err != nil {
					c.JSON(http.StatusOK, gin.H{
						"status":  "Fail",
						"message": fmt.Sprintf("更新失败:%v", err),
						"data":    nil,
					})
				} else {
					c.JSON(http.StatusOK, gin.H{
						"status":  "Fail",
						"message": "更新成功,请输入/login重新登录",
						"data":    nil,
					})
				}
				c.Abort()
				return
			}
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		auth := c.Request.Header.Get("Authorization")
		if auth == "" {
			c.Header("WWW-Authenticate", "Basic realm=\"Restricted\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		username, password, ok := c.Request.BasicAuth()
		authCode := ac.AuthenticateUser(username, password)
		if !ok || authCode != 0 {
			if authCode == 1 {
				c.JSON(http.StatusOK, gin.H{
					"status":  "Fail",
					"message": "账号未授权,请输入/login登录其他账户",
					"data":    nil,
				})
			}
			if authCode == 2 {
				c.JSON(http.StatusOK, gin.H{
					"status":  "Fail",
					"message": fmt.Sprintf("Token数已用尽, 您的账号: %s", username),
					"data":    nil,
				})
			}
			c.Abort()
			return
		}

		c.Set("username", username)
		c.Next()
	}
}

func OpsAuth(theKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Request.Header.Get("Opskey")
		if key != theKey {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "Fail",
				"message": "Header Opskey error",
				"data":    nil,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
