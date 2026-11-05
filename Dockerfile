FROM node:24-alpine AS ui
WORKDIR /ui
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /server .

FROM alpine:3.22
WORKDIR /app
COPY --from=api /server ./server
COPY --from=ui /ui/dist/ ./public/
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/app/server"]

