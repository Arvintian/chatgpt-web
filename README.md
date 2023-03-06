# ChatGPT-Web

使用[Gin](https://github.com/gin-gonic/gin)搭建ChatGPT服务,使用[ChatGPT Web](https://github.com/Chanzhaoyu/chatgpt-web)作为前端

## Usage

[Docker Hub](https://hub.docker.com/repository/docker/arvintian/chatgpt-web/general)

```
docker run --restart unless-stopped -d --log-opt max-size=50m -p 7080:7080 \
-e OPENAI_API_KEY=openai-key \
-e BASIC_AUTH_USER=user1,user2 \
-e BASIC_AUTH_PASSWORD=passwd1,passwd2 \
arvintian/chatgpt-web
```

- 兼容[ChatGPT Web](https://github.com/Chanzhaoyu/chatgpt-web#%E4%BD%BF%E7%94%A8-docker)所有环境变量
- SERVER_PORT 服务端口,默认7080
- SERVER_HOST 服务监听地址,默认0.0.0.0
- BASIC_AUTH_USER 认证用户,多用户英文逗号分隔
- BASIC_AUTH_PASSWORD 认证用户密码,多用户英文逗号分隔

## 待实现

[done] 支持认证

[ ] 替换NodeAPI对接ChatGPT API