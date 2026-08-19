FROM node:24-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/importer ./cmd/importer && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate-avatars ./cmd/migrate-avatars

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=frontend /app/frontend/dist /app/dist
COPY --from=backend /out/server /app/server
COPY --from=backend /out/importer /app/importer
COPY --from=backend /out/migrate-avatars /app/migrate-avatars
WORKDIR /app
EXPOSE 8080
ENV STATIC_DIR=/app/dist
ENTRYPOINT ["/app/server"]
