FROM chenzhaoyu94/chatgpt-web:v2.10.9 as frontend

FROM arvintian/chatgpt-web-base:v3

COPY --from=frontend /app/public /app/public

ADD dist/server /app/server

RUN mkdir -p /data

EXPOSE 7080

CMD ["/app/server"]