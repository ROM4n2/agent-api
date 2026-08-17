FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
ENV GOTOOLCHAIN=auto
RUN CGO_ENABLED=0 go build -v -o agent-api . 2>&1
RUN find /src -name "agent-api*" -o -name "*.exe" 2>/dev/null
RUN ls -la /src/agent-api

# 第二阶段：运行
FROM alpine
COPY --from=build /src/agent-api /agent-api
CMD ["./agent-api"]
