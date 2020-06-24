FROM alpine
MAINTAINER  Aze
WORKDIR /tmp/hello
COPY ./app/main /tmp/main
EXPOSE 2508
RUN chmod +x main