# Build tailstream-agent as a static binary
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /tailstream-agent .

# Minimal runtime image
FROM alpine:3.19
COPY --from=build /tailstream-agent /usr/local/bin/tailstream-agent
ENTRYPOINT ["/usr/local/bin/tailstream-agent"]
