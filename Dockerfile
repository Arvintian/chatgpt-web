FROM chenzhaoyu94/chatgpt-web:v2.10.9 as frontend

FROM arvintian/chatgpt-web-base:v1

COPY --from=frontend /app/public /app/public

ADD dist/server /app/server

EXPOSE 7080

CMD ["/app/server"]