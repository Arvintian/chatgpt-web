# ChatGPT-Web

[国内在线服务](https://faka.v95.xyz)

[English](https://github.com/Arvintian/chatgpt-web/blob/main/README_en.md)

使用[Gin](https://github.com/gin-gonic/gin)搭建ChatGPT服务,使用[ChatGPT Web](https://github.com/Chanzhaoyu/chatgpt-web)作为前端

## 功能

[✓] 用户认证、管理

[✓] 对话交互帮助

[✓] Token使用量计费、充值

[✓] OPENAI接口代理服务器

## 效果

![cover](./docs/chat-shot.png)

## 快速部署

[Docker Hub](https://hub.docker.com/repository/docker/arvintian/chatgpt-web/general)

```
docker run --restart unless-stopped -d --log-opt max-size=50m -p 7080:7080 \
-e OPENAI_KEY=openai-key \
-e BASIC_AUTH_USER=user1,user2 \
-e BASIC_AUTH_PASSWORD=passwd1,passwd2 \
arvintian/chatgpt-web
```

- SERVER_PORT 服务端口,默认7080
- SERVER_HOST 服务监听地址,默认0.0.0.0
- SOCKS_PROXY socks代理URL,例如socks5://user:password@127.0.0.1:1080
- CHAT_SESSION_TTL 会话上下文保持时间,默认30分钟
- CHAT_MIN_RESPONSE_TOKENS 预留给会话响应的token数,可能导致截断最久的上下文,默认600
- OPENAI_KEY openai api key,参考OpenAI文档
- OPENAI_BASE_URL openai api base url,默认https://api.openai.com/v1
- OPENAI_MODEL 调用模型,默认gpt-3.5-turbo
- OPENAI_MAX_TOKENS 模型max_tokens参数,参考OpenAI文档
- OPENAI_TEMPERATURE 模型temperature参数,参考OpenAI文档
- OPENAI_PRESENCE_PENALTY 模型presence_penalty参数,参考OpenAI文档
- OPENAI_FREQUENCY_PENALTY 模型frequency_penalty参数,参考OpenAI文档

模型float32参数使用(整型/100)设置,例如: temperature设置0.8,需要设置为80

## 环境变量配置

### 静态用户

- BASIC_AUTH_USER 认证用户,多用户英文逗号分隔
- BASIC_AUTH_PASSWORD 认证用户密码,多用户英文逗号分隔

设置静态用户，静态用户不计token使用量，方便不共享搭建

### 系统管理

- OPS_KEY 管理接口认证key
- OPS_LINK 当用户认证失败、token余额不足提示自助链接

系统管理配置项，方便集成其他系统

### 数据库设置

- DB 数据库连接

系统默认内置使用SQLite，数据路径/data/chatgpt.db，支持MySQL，正确设置数据库连接即可，参考[GORM](https://gorm.io/zh_CN/docs/connecting_to_the_database.html)

### 代理服务器 

- OPENAI_PROXY 开启OpenAI接口的代理服务器

在OPENAI_BASE_URL的基础上再开正向代理，方便用作OpenAI接口的代理服务器


## API

调用api需要管理认证

Header opskey:OPS_KEY

### 用户管理

POST /accounts

添加用户

```
{
    "action":"register",
    "i_username":"arvin",
    "i_password":"test",
    "count":2000 # 初始token数
}
```

补充token数

```
{
    "action":"recharge",
    "i_username":"arvin",
    "count":1000
}
```

查询用户

```
{
    "action":"check",
    "i_username":"arvin"
}
```