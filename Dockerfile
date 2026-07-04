FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/team-task-tracker ./cmd/api

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bin/team-task-tracker /app/team-task-tracker
COPY config.example.yaml /app/config.yaml
COPY migrations /app/migrations
EXPOSE 8080
CMD ["/app/team-task-tracker"]

