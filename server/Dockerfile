FROM node:18-alpine AS frontend

WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM golang:1.26-alpine AS build

WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ .
COPY --from=frontend /web/dist /app/internal/static/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM scratch

COPY --from=build /app/server /server

EXPOSE 8080

ENTRYPOINT ["/server"]
