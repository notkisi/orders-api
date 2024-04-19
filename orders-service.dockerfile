# build a tiny docker image
FROM alpine:latest

RUN mkdir /app

COPY bin/api /app

CMD [ "/app/api" ]
