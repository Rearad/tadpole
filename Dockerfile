FROM alpine
MAINTAINER  Aze
WORKDIR /go/src/
COPY . .
EXPOSE 2508
ENTRYPOINT ["./app/main"]

