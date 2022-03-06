FROM golang:latest
RUN mkdir voice-chat-server
WORKDIR voice-chat-server
COPY go.mod voicechat.go .
RUN mkdir server
COPY server/main.go ./server
RUN go mod tidy
EXPOSE 8080
# ENTRYPOINT ["/bin/bash"]
CMD ["go", "run", "./server/main.go"]
